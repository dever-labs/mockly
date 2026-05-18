package mqttserver_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func startServer(t *testing.T, srv interface{ Start(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(150 * time.Millisecond)
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

func connectMQTT(t *testing.T, addr, clientID string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial MQTT: %v", err)
	}
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	body := append(mqttString("MQTT"), 0x04, 0x02, 0x00, 0x1e)
	body = append(body, mqttString(clientID)...)
	writeMQTTPacket(t, conn, 0x10, body)
	header, resp := readMQTTPacket(t, conn)
	if header>>4 != 2 || len(resp) != 2 || resp[1] != 0 {
		t.Fatalf("unexpected CONNACK: header=%#x body=%v", header, resp)
	}
	return conn
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

func readPublish(t *testing.T, conn net.Conn) (string, string) {
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

func TestMQTTServer_GlobalFault(t *testing.T) {
	t.Skip("TODO: implement MQTT response delivery assertion with a compatible client")

	port := freePort(t)
	sc := scenarios.New(nil)
	log := logger.New(100)
	srv := mqttserver.New(&config.MQTTConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.MQTTMock{{
			ID:       "m",
			Topic:    "test/topic",
			Response: &config.MQTTResponse{Topic: "test/response", Payload: "world"},
		}},
	}, state.New(), sc, log)
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sub := connectMQTT(t, addr, fmt.Sprintf("sub-%d", time.Now().UnixNano()))
	defer sub.Close() //nolint:errcheck
	subscribeMQTT(t, sub, "test/response")
	pub := connectMQTT(t, addr, fmt.Sprintf("pub-%d", time.Now().UnixNano()))
	defer pub.Close() //nolint:errcheck

	sc.SetDirectFaults(config.ProtocolFaults{MQTT: &config.MQTTFault{ErrorRate: 0}})
	publishMQTT(t, pub, "test/topic", "hello")
	if err := sub.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set MQTT fault deadline: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := sub.Read(buf); err == nil {
		t.Fatal("faulted publish unexpectedly produced MQTT data")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("faulted publish read error = %v, want timeout", err)
	}

	sc.ClearDirectFaults()
	if err := sub.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set MQTT normal deadline: %v", err)
	}
	publishMQTT(t, pub, "test/topic", "hello")
	topic, payload := readPublish(t, sub)
	_ = log
	if topic != "test/response" {
		t.Fatalf("normal MQTT topic = %q, want %q", topic, "test/response")
	}
	if payload != "world" {
		t.Fatalf("normal MQTT payload = %q, want %q", payload, "world")
	}
}

func TestMQTTServer_MQTTFault_FromScenario(t *testing.T) {
	t.Skip("TODO: implement MQTT scenario fault assertion with a compatible client")
}
