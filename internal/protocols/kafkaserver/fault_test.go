package kafkaserver_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/kafkaserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func startServer(t *testing.T, srv interface{ Start(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(100 * time.Millisecond)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	time.Sleep(10 * time.Millisecond)
	return port
}

func kafkaString(s string) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, int16(len(s)))
	buf.WriteString(s)
	return buf.Bytes()
}

func kafkaBytes(b []byte) []byte {
	buf := new(bytes.Buffer)
	if b == nil {
		_ = binary.Write(buf, binary.BigEndian, int32(-1))
		return buf.Bytes()
	}
	_ = binary.Write(buf, binary.BigEndian, int32(len(b)))
	buf.Write(b)
	return buf.Bytes()
}

func buildMessageSet(key, value string) []byte {
	payload := []byte{0, 0}
	if key == "" {
		payload = append(payload, kafkaBytes(nil)...)
	} else {
		payload = append(payload, kafkaBytes([]byte(key))...)
	}
	payload = append(payload, kafkaBytes([]byte(value))...)
	crc := crc32.ChecksumIEEE(payload)
	msg := new(bytes.Buffer)
	_ = binary.Write(msg, binary.BigEndian, int64(0))
	_ = binary.Write(msg, binary.BigEndian, int32(len(payload)+4))
	_ = binary.Write(msg, binary.BigEndian, crc)
	msg.Write(payload)
	return msg.Bytes()
}

func buildProduceRequest(topic, key, value string) []byte {
	recordSet := buildMessageSet(key, value)
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.BigEndian, int16(0))
	_ = binary.Write(body, binary.BigEndian, int16(0))
	_ = binary.Write(body, binary.BigEndian, int32(1))
	body.Write(kafkaString("test"))
	_ = binary.Write(body, binary.BigEndian, int16(1))
	_ = binary.Write(body, binary.BigEndian, int32(1000))
	_ = binary.Write(body, binary.BigEndian, int32(1))
	body.Write(kafkaString(topic))
	_ = binary.Write(body, binary.BigEndian, int32(1))
	_ = binary.Write(body, binary.BigEndian, int32(0))
	_ = binary.Write(body, binary.BigEndian, int32(len(recordSet)))
	body.Write(recordSet)
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

func sendProduce(t *testing.T, addr string) int16 {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial Kafka: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(buildProduceRequest("test-topic", "k", "v")); err != nil {
		t.Fatalf("write Kafka produce request: %v", err)
	}
	var size int32
	if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
		t.Fatalf("read Kafka response size: %v", err)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read Kafka response payload: %v", err)
	}
	r := bytes.NewReader(payload)
	var correlationID int32
	var topicCount int32
	var partitionCount int32
	var partition int32
	var errorCode int16
	_ = binary.Read(r, binary.BigEndian, &correlationID)
	_ = binary.Read(r, binary.BigEndian, &topicCount)
	if topicCount != 1 {
		t.Fatalf("Kafka topic count = %d, want 1", topicCount)
	}
	var topicLen int16
	_ = binary.Read(r, binary.BigEndian, &topicLen)
	topic := make([]byte, topicLen)
	if _, err := io.ReadFull(r, topic); err != nil {
		t.Fatalf("read Kafka topic: %v", err)
	}
	_ = binary.Read(r, binary.BigEndian, &partitionCount)
	_ = binary.Read(r, binary.BigEndian, &partition)
	_ = binary.Read(r, binary.BigEndian, &errorCode)
	return errorCode
}

func TestKafkaServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := kafkaserver.New(&config.KafkaConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.KafkaMock{{
			ID:      "m",
			Topic:   "test-topic",
			Records: []config.KafkaRecord{{Key: "k", Value: "v"}},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{Kafka: &config.KafkaFault{ErrorRate: 0}})
	if code := sendProduce(t, addr); code != 5 {
		t.Fatalf("fault produce error code = %d, want 5", code)
	}

	sc.ClearDirectFaults()
	if code := sendProduce(t, addr); code != 0 {
		t.Fatalf("normal produce error code = %d, want 0", code)
	}
}

func TestKafkaServer_KafkaFault_CustomErrorCode(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := kafkaserver.New(&config.KafkaConfig{Enabled: true, Port: port}, state.New(), sc, logger.New(100))
	startServer(t, srv)
	sc.SetDirectFaults(config.ProtocolFaults{Kafka: &config.KafkaFault{ErrorCode: 7, ErrorRate: 0}})
	if code := sendProduce(t, fmt.Sprintf("127.0.0.1:%d", port)); code != 7 {
		t.Fatalf("fault produce error code = %d, want 7", code)
	}
}
