//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	nats "github.com/nats-io/nats.go"
)

// ---------------------------------------------------------------------------
// MQTT: raw MQTT v3.1.1 packet helpers (no client library dependency; the
// module only depends on a server-side MQTT broker library, so round trips
// are hand-crafted at the byte level, mirroring the approach used by
// internal/protocols/mqttserver's own tests).
// ---------------------------------------------------------------------------

func mqttString(s string) []byte {
	return append([]byte{byte(len(s) >> 8), byte(len(s))}, []byte(s)...)
}

func mqttRemainingLength(n int) []byte {
	var out []byte
	for {
		digit := byte(n % 128)
		n /= 128
		if n > 0 {
			digit |= 0x80
		}
		out = append(out, digit)
		if n == 0 {
			return out
		}
	}
}

func writeMQTTPacket(t *testing.T, conn net.Conn, header byte, body []byte) {
	t.Helper()
	packet := []byte{header}
	packet = append(packet, mqttRemainingLength(len(body))...)
	packet = append(packet, body...)
	if _, err := conn.Write(packet); err != nil {
		t.Fatalf("write MQTT packet: %v", err)
	}
}

func readMQTTPacket(t *testing.T, conn net.Conn) (byte, []byte) {
	t.Helper()
	head := make([]byte, 1)
	if _, err := io.ReadFull(conn, head); err != nil {
		t.Fatalf("read MQTT header: %v", err)
	}
	multiplier := 1
	remaining := 0
	for {
		b := make([]byte, 1)
		if _, err := io.ReadFull(conn, b); err != nil {
			t.Fatalf("read MQTT remaining length: %v", err)
		}
		remaining += int(b[0]&127) * multiplier
		if b[0]&128 == 0 {
			break
		}
		multiplier *= 128
	}
	body := make([]byte, remaining)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatalf("read MQTT body: %v", err)
	}
	return head[0], body
}

func connectMQTT(t *testing.T, conn net.Conn, clientID string) {
	t.Helper()
	body := append(mqttString("MQTT"), 0x04, 0x02, 0x00, 0x1e)
	body = append(body, mqttString(clientID)...)
	writeMQTTPacket(t, conn, 0x10, body)
	header, resp := readMQTTPacket(t, conn)
	if header>>4 != 2 || len(resp) != 2 || resp[1] != 0 {
		t.Fatalf("unexpected CONNACK: header=%#x body=%v", header, resp)
	}
}

func subscribeMQTT(t *testing.T, conn net.Conn, topic string) {
	t.Helper()
	body := []byte{0x00, 0x01}
	body = append(body, mqttString(topic)...)
	body = append(body, 0x00)
	writeMQTTPacket(t, conn, 0x82, body)
	header, _ := readMQTTPacket(t, conn)
	if header>>4 != 9 {
		t.Fatalf("unexpected SUBACK header %#x", header)
	}
}

func publishMQTT(t *testing.T, conn net.Conn, topic, payload string) {
	t.Helper()
	body := append(mqttString(topic), []byte(payload)...)
	writeMQTTPacket(t, conn, 0x30, body)
}

func readMQTTPublish(t *testing.T, conn net.Conn) (string, string) {
	t.Helper()
	header, body := readMQTTPacket(t, conn)
	if header>>4 != 3 {
		t.Fatalf("unexpected MQTT publish header %#x", header)
	}
	topicLen := int(body[0])<<8 | int(body[1])
	topic := string(body[2 : 2+topicLen])
	payload := string(body[2+topicLen:])
	return topic, payload
}

// TestE2E_MQTT_MockAndRequest proves that mockly.yaml's protocols.mqtt block
// is wired into cmd/mockly's startup path: it starts the real binary with an
// MQTT mock configured, subscribes to the mock's response topic over a raw
// MQTT v3.1.1 connection, publishes to the mock's request topic, and asserts
// the broker relays back the exact configured response topic/payload.
func TestE2E_MQTT_MockAndRequest(t *testing.T) {
	mqttPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  mqtt:
    enabled: true
    port: %d
    mocks:
      - id: ping
        topic: devices/ping
        response:
          topic: devices/pong
          payload: "pong"
`, mqttPort)
	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", mqttPort)
	sub := dialWithRetry(t, "tcp", addr, 5*time.Second)
	defer func() { _ = sub.Close() }()
	connectMQTT(t, sub, "e2e-sub")
	subscribeMQTT(t, sub, "devices/pong")

	pub := dialWithRetry(t, "tcp", addr, 5*time.Second)
	defer func() { _ = pub.Close() }()
	connectMQTT(t, pub, "e2e-pub")

	_ = sub.SetReadDeadline(time.Now().Add(5 * time.Second))
	publishMQTT(t, pub, "devices/ping", "ping")

	topic, payload := readMQTTPublish(t, sub)
	if topic != "devices/pong" {
		t.Errorf("response topic = %q, want %q", topic, "devices/pong")
	}
	if payload != "pong" {
		t.Errorf("response payload = %q, want %q", payload, "pong")
	}
}

// ---------------------------------------------------------------------------
// NATS: the module already depends on github.com/nats-io/nats.go for its own
// server tests, so use it as a real client against the embedded server
// started via the real binary + mockly.yaml.
// ---------------------------------------------------------------------------

// TestE2E_NATS_MockAndRequest proves that mockly.yaml's protocols.nats block
// is wired into cmd/mockly's startup path: it starts the real binary with a
// NATS request/reply mock configured, connects a real nats.go client, and
// asserts nc.Request returns the exact configured payload.
func TestE2E_NATS_MockAndRequest(t *testing.T) {
	natsPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  nats:
    enabled: true
    port: %d
    mocks:
      - id: ping
        subject: svc.ping
        response:
          payload: "pong"
`, natsPort)
	startMocklyConfig(t, cfg)

	url := fmt.Sprintf("nats://127.0.0.1:%d", natsPort)
	var nc *nats.Conn
	var err error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nc, err = nats.Connect(url, nats.Timeout(200*time.Millisecond))
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("connect to NATS: %v", err)
	}
	defer nc.Close()

	reply, err := nc.Request("svc.ping", []byte("ping"), 3*time.Second)
	if err != nil {
		t.Fatalf("NATS request: %v", err)
	}
	if string(reply.Data) != "pong" {
		t.Errorf("NATS reply = %q, want %q", reply.Data, "pong")
	}
}

// ---------------------------------------------------------------------------
// AMQP: hand-rolled AMQP 0-9-1 frames, adapted from
// internal/protocols/amqpserver's fault_test.go helpers (no client library
// dependency needed).
// ---------------------------------------------------------------------------

func amqpWriteFrame(t *testing.T, conn net.Conn, frameType byte, channel uint16, payload []byte) {
	t.Helper()
	head := []byte{frameType, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(head[1:3], channel)
	binary.BigEndian.PutUint32(head[3:7], uint32(len(payload)))
	if _, err := conn.Write(head); err != nil {
		t.Fatalf("write AMQP frame header: %v", err)
	}
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write AMQP frame payload: %v", err)
	}
	if _, err := conn.Write([]byte{0xce}); err != nil {
		t.Fatalf("write AMQP frame end: %v", err)
	}
}

func amqpWriteMethodFrame(t *testing.T, conn net.Conn, channel, classID, methodID uint16, args []byte) {
	t.Helper()
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], classID)
	binary.BigEndian.PutUint16(payload[2:4], methodID)
	payload = append(payload, args...)
	amqpWriteFrame(t, conn, 1, channel, payload)
}

func amqpWriteHeaderFrame(t *testing.T, conn net.Conn, channel, classID uint16, body []byte) {
	t.Helper()
	payload := make([]byte, 14)
	binary.BigEndian.PutUint16(payload[0:2], classID)
	binary.BigEndian.PutUint64(payload[4:12], uint64(len(body)))
	amqpWriteFrame(t, conn, 2, channel, payload)
}

func amqpReadFrame(t *testing.T, conn net.Conn) (byte, uint16, []byte) {
	t.Helper()
	head := make([]byte, 7)
	if _, err := io.ReadFull(conn, head); err != nil {
		t.Fatalf("read AMQP frame header: %v", err)
	}
	size := binary.BigEndian.Uint32(head[3:7])
	payload := make([]byte, size+1)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read AMQP frame payload: %v", err)
	}
	return head[0], binary.BigEndian.Uint16(head[1:3]), payload[:len(payload)-1]
}

func amqpShortStr(s string) []byte {
	return append([]byte{byte(len(s))}, []byte(s)...)
}

func openAMQPConnection(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn := dialWithRetry(t, "tcp", addr, 5*time.Second)
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("AMQP\x00\x00\x09\x01")); err != nil {
		t.Fatalf("write AMQP protocol header: %v", err)
	}
	amqpReadFrame(t, conn)                                     // Connection.Start
	amqpWriteMethodFrame(t, conn, 0, 10, 11, nil)              // Connection.StartOk
	amqpReadFrame(t, conn)                                     // Connection.Tune
	amqpWriteMethodFrame(t, conn, 0, 10, 31, make([]byte, 8))  // Connection.TuneOk
	amqpWriteMethodFrame(t, conn, 0, 10, 40, amqpShortStr("")) // Connection.Open
	amqpReadFrame(t, conn)                                     // Connection.OpenOk
	amqpWriteMethodFrame(t, conn, 1, 20, 10, nil)              // Channel.Open
	amqpReadFrame(t, conn)                                     // Channel.OpenOk
	queueArgs := append([]byte{0, 0}, amqpShortStr("e2e-queue")...)
	amqpWriteMethodFrame(t, conn, 1, 50, 10, queueArgs) // Queue.Declare
	amqpReadFrame(t, conn)                              // Queue.DeclareOk
	consumeArgs := append([]byte{0, 0}, amqpShortStr("e2e-queue")...)
	consumeArgs = append(consumeArgs, amqpShortStr("ctag")...)
	amqpWriteMethodFrame(t, conn, 1, 60, 20, consumeArgs) // Basic.Consume
	amqpReadFrame(t, conn)                                // Basic.ConsumeOk
	return conn
}

func publishAMQP(t *testing.T, conn net.Conn, exchange, routingKey, body string) {
	t.Helper()
	args := append([]byte{0, 0}, amqpShortStr(exchange)...)
	args = append(args, amqpShortStr(routingKey)...)
	amqpWriteMethodFrame(t, conn, 1, 60, 40, args) // Basic.Publish
	payload := []byte(body)
	amqpWriteHeaderFrame(t, conn, 1, 60, payload)
	amqpWriteFrame(t, conn, 3, 1, payload)
}

func readAMQPDeliveryBody(t *testing.T, conn net.Conn) string {
	t.Helper()
	frameType, _, payload := amqpReadFrame(t, conn)
	if frameType != 1 || binary.BigEndian.Uint16(payload[0:2]) != 60 || binary.BigEndian.Uint16(payload[2:4]) != 60 {
		t.Fatalf("unexpected AMQP method frame: type=%d payload=%v", frameType, payload)
	}
	frameType, _, _ = amqpReadFrame(t, conn)
	if frameType != 2 {
		t.Fatalf("unexpected AMQP header frame type %d", frameType)
	}
	frameType, _, payload = amqpReadFrame(t, conn)
	if frameType != 3 {
		t.Fatalf("unexpected AMQP body frame type %d", frameType)
	}
	return string(payload)
}

// TestE2E_AMQP_MockAndRequest proves that mockly.yaml's protocols.amqp block
// is wired into cmd/mockly's startup path: it starts the real binary with an
// AMQP mock configured, performs a hand-crafted AMQP 0-9-1 connection/channel
// handshake plus a basic.publish, and asserts the mocked response body is
// delivered back on the consumer.
func TestE2E_AMQP_MockAndRequest(t *testing.T) {
	amqpPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  amqp:
    enabled: true
    port: %d
    mocks:
      - id: m
        exchange: test
        routing_key: rk
        response:
          body: "hello"
`, amqpPort)
	startMocklyConfig(t, cfg)

	conn := openAMQPConnection(t, fmt.Sprintf("127.0.0.1:%d", amqpPort))
	defer func() { _ = conn.Close() }()

	publishAMQP(t, conn, "test", "rk", "ping")
	if body := readAMQPDeliveryBody(t, conn); body != "hello" {
		t.Errorf("AMQP delivery body = %q, want %q", body, "hello")
	}
}

// ---------------------------------------------------------------------------
// STOMP: raw text frames over net.Dial, adapted from
// internal/protocols/stompserver's fault_test.go helpers.
// ---------------------------------------------------------------------------

func readSTOMPFrame(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	frame, err := reader.ReadString(0)
	if err != nil {
		t.Fatalf("read STOMP frame: %v", err)
	}
	return strings.TrimSuffix(frame, "\x00")
}

func dialSTOMP(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn := dialWithRetry(t, "tcp", addr, 5*time.Second)
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	if _, err := io.WriteString(conn, "CONNECT\naccept-version:1.2\nhost:localhost\n\n\x00"); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	frame := readSTOMPFrame(t, reader)
	if !strings.HasPrefix(frame, "CONNECTED\n") {
		t.Fatalf("connect frame = %q, want CONNECTED", frame)
	}
	return conn, reader
}

// TestE2E_STOMP_MockAndRequest proves that mockly.yaml's protocols.stomp
// block is wired into cmd/mockly's startup path: it starts the real binary
// with a STOMP mock configured, performs a real STOMP CONNECT/SUBSCRIBE/SEND
// sequence, and asserts the resulting MESSAGE frame carries the exact
// configured destination and body.
func TestE2E_STOMP_MockAndRequest(t *testing.T) {
	stompPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  stomp:
    enabled: true
    port: %d
    mocks:
      - id: m
        destination: /queue/test
        response:
          body: "hello"
`, stompPort)
	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", stompPort)
	conn, reader := dialSTOMP(t, addr)
	defer func() { _ = conn.Close() }()

	if _, err := io.WriteString(conn, "SUBSCRIBE\nid:1\ndestination:/queue/test\nack:auto\n\n\x00"); err != nil {
		t.Fatalf("write SUBSCRIBE: %v", err)
	}
	if _, err := io.WriteString(conn, "SEND\ndestination:/queue/test\ncontent-length:5\n\nhello\x00"); err != nil {
		t.Fatalf("write SEND: %v", err)
	}

	frame := readSTOMPFrame(t, reader)
	if !strings.HasPrefix(frame, "MESSAGE\n") {
		t.Fatalf("frame = %q, want MESSAGE", frame)
	}
	if !strings.Contains(frame, "destination:/queue/test") {
		t.Errorf("frame = %q, want destination header", frame)
	}
	if !strings.HasSuffix(frame, "\n\nhello") {
		t.Errorf("frame = %q, want body %q", frame, "hello")
	}
}

// ---------------------------------------------------------------------------
// Kafka: hand-rolled wire protocol framing, adapted from
// internal/protocols/kafkaserver's fault_test.go helpers.
// ---------------------------------------------------------------------------

func kafkaStringBytes(s string) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, int16(len(s)))
	buf.WriteString(s)
	return buf.Bytes()
}

func kafkaNullableBytes(b []byte) []byte {
	buf := new(bytes.Buffer)
	if b == nil {
		_ = binary.Write(buf, binary.BigEndian, int32(-1))
		return buf.Bytes()
	}
	_ = binary.Write(buf, binary.BigEndian, int32(len(b)))
	buf.Write(b)
	return buf.Bytes()
}

func kafkaBuildMessageSet(key, value string) []byte {
	payload := []byte{0, 0}
	if key == "" {
		payload = append(payload, kafkaNullableBytes(nil)...)
	} else {
		payload = append(payload, kafkaNullableBytes([]byte(key))...)
	}
	payload = append(payload, kafkaNullableBytes([]byte(value))...)
	crc := crc32.ChecksumIEEE(payload)
	msg := new(bytes.Buffer)
	_ = binary.Write(msg, binary.BigEndian, int64(0))
	_ = binary.Write(msg, binary.BigEndian, int32(len(payload)+4))
	_ = binary.Write(msg, binary.BigEndian, crc)
	msg.Write(payload)
	return msg.Bytes()
}

func kafkaBuildProduceRequest(topic, key, value string) []byte {
	recordSet := kafkaBuildMessageSet(key, value)
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.BigEndian, int16(0)) // apiKey: Produce
	_ = binary.Write(body, binary.BigEndian, int16(0)) // apiVersion
	_ = binary.Write(body, binary.BigEndian, int32(1)) // correlationID
	body.Write(kafkaStringBytes("e2e-client"))
	_ = binary.Write(body, binary.BigEndian, int16(1))    // requiredAcks
	_ = binary.Write(body, binary.BigEndian, int32(1000)) // timeout
	_ = binary.Write(body, binary.BigEndian, int32(1))    // numTopics
	body.Write(kafkaStringBytes(topic))
	_ = binary.Write(body, binary.BigEndian, int32(1)) // numPartitions
	_ = binary.Write(body, binary.BigEndian, int32(0)) // partition
	_ = binary.Write(body, binary.BigEndian, int32(len(recordSet)))
	body.Write(recordSet)
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

func kafkaBuildFetchRequest(topic string) []byte {
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.BigEndian, int16(1)) // apiKey: Fetch
	_ = binary.Write(body, binary.BigEndian, int16(0)) // apiVersion
	_ = binary.Write(body, binary.BigEndian, int32(2)) // correlationID
	body.Write(kafkaStringBytes("e2e-client"))
	_ = binary.Write(body, binary.BigEndian, int32(-1))   // replicaID
	_ = binary.Write(body, binary.BigEndian, int32(1000)) // maxWait
	_ = binary.Write(body, binary.BigEndian, int32(0))    // minBytes
	_ = binary.Write(body, binary.BigEndian, int32(1))    // numTopics
	body.Write(kafkaStringBytes(topic))
	_ = binary.Write(body, binary.BigEndian, int32(1))       // numPartitions
	_ = binary.Write(body, binary.BigEndian, int32(0))       // partition
	_ = binary.Write(body, binary.BigEndian, int64(0))       // fetchOffset
	_ = binary.Write(body, binary.BigEndian, int32(1000000)) // maxBytes
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// kafkaReadResponse writes req to conn and returns the raw response payload
// (everything after the 4-byte size and 4-byte correlation ID).
func kafkaReadResponse(t *testing.T, conn net.Conn, req []byte) []byte {
	t.Helper()
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("write Kafka request: %v", err)
	}
	var size int32
	if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
		t.Fatalf("read Kafka response size: %v", err)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read Kafka response payload: %v", err)
	}
	var correlationID int32
	r := bytes.NewReader(payload)
	_ = binary.Read(r, binary.BigEndian, &correlationID)
	rest := make([]byte, r.Len())
	_, _ = io.ReadFull(r, rest)
	return rest
}

// kafkaParseFetchedRecord decodes a Fetch response body produced by
// kafkaserver.buildMessageSet and returns the key/value of its first
// record (sufficient for a single-record mock).
func kafkaParseFetchedRecord(t *testing.T, body []byte) (key, value string) {
	t.Helper()
	r := bytes.NewReader(body)
	var topicCount int32
	_ = binary.Read(r, binary.BigEndian, &topicCount)
	if topicCount != 1 {
		t.Fatalf("Kafka Fetch topic count = %d, want 1", topicCount)
	}
	var topicLen int16
	_ = binary.Read(r, binary.BigEndian, &topicLen)
	topicBuf := make([]byte, topicLen)
	_, _ = io.ReadFull(r, topicBuf)
	var partitionCount int32
	_ = binary.Read(r, binary.BigEndian, &partitionCount)
	var partition int32
	var errorCode int16
	var highWaterMark int64
	var messageSetLen int32
	_ = binary.Read(r, binary.BigEndian, &partition)
	_ = binary.Read(r, binary.BigEndian, &errorCode)
	_ = binary.Read(r, binary.BigEndian, &highWaterMark)
	_ = binary.Read(r, binary.BigEndian, &messageSetLen)
	if errorCode != 0 {
		t.Fatalf("Kafka Fetch error code = %d, want 0", errorCode)
	}
	messageSet := make([]byte, messageSetLen)
	if _, err := io.ReadFull(r, messageSet); err != nil {
		t.Fatalf("read Kafka message set: %v", err)
	}
	mr := bytes.NewReader(messageSet)
	var offset int64
	var msgSize int32
	_ = binary.Read(mr, binary.BigEndian, &offset)
	_ = binary.Read(mr, binary.BigEndian, &msgSize)
	entry := make([]byte, msgSize)
	if _, err := io.ReadFull(mr, entry); err != nil {
		t.Fatalf("read Kafka message entry: %v", err)
	}
	er := bytes.NewReader(entry)
	var crc uint32
	var magic, attrs byte
	_ = binary.Read(er, binary.BigEndian, &crc)
	_ = binary.Read(er, binary.BigEndian, &magic)
	_ = binary.Read(er, binary.BigEndian, &attrs)
	key = string(kafkaReadBytesField(er))
	value = string(kafkaReadBytesField(er))
	return key, value
}

func kafkaReadBytesField(r *bytes.Reader) []byte {
	var n int32
	_ = binary.Read(r, binary.BigEndian, &n)
	if n < 0 {
		return nil
	}
	buf := make([]byte, n)
	_, _ = io.ReadFull(r, buf)
	return buf
}

// TestE2E_Kafka_MockAndRequest proves that mockly.yaml's protocols.kafka
// block is wired into cmd/mockly's startup path: it starts the real binary
// with a Kafka mock configured, sends a hand-crafted Produce request (any
// real client would do this first), then a Fetch request for the same
// topic, and asserts the fetched record matches the mock's configured
// key/value exactly.
func TestE2E_Kafka_MockAndRequest(t *testing.T) {
	kafkaPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  kafka:
    enabled: true
    port: %d
    mocks:
      - id: m
        topic: test-topic
        records:
          - key: "k"
            value: "v"
`, kafkaPort)
	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", kafkaPort)
	conn := dialWithRetry(t, "tcp", addr, 5*time.Second)
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	kafkaReadResponse(t, conn, kafkaBuildProduceRequest("test-topic", "pk", "pv"))

	fetchBody := kafkaReadResponse(t, conn, kafkaBuildFetchRequest("test-topic"))
	key, value := kafkaParseFetchedRecord(t, fetchBody)
	if key != "k" {
		t.Errorf("Kafka fetched record key = %q, want %q", key, "k")
	}
	if value != "v" {
		t.Errorf("Kafka fetched record value = %q, want %q", value, "v")
	}
}
