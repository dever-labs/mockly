package amqpserver_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/amqpserver"
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

func writeFrame(t *testing.T, conn net.Conn, frameType byte, channel uint16, payload []byte) {
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

func writeMethodFrame(t *testing.T, conn net.Conn, channel, classID, methodID uint16, args []byte) {
	t.Helper()
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], classID)
	binary.BigEndian.PutUint16(payload[2:4], methodID)
	payload = append(payload, args...)
	writeFrame(t, conn, 1, channel, payload)
}

func writeHeaderFrame(t *testing.T, conn net.Conn, channel, classID uint16, body []byte) {
	t.Helper()
	payload := make([]byte, 14)
	binary.BigEndian.PutUint16(payload[0:2], classID)
	binary.BigEndian.PutUint64(payload[4:12], uint64(len(body)))
	writeFrame(t, conn, 2, channel, payload)
}

func readFrame(t *testing.T, conn net.Conn) (byte, uint16, []byte) {
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

func shortStr(s string) []byte {
	return append([]byte{byte(len(s))}, []byte(s)...)
}

func openAMQPConnection(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial AMQP: %v", err)
	}
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("AMQP\x00\x00\x09\x01")); err != nil {
		t.Fatalf("write AMQP protocol header: %v", err)
	}
	readFrame(t, conn)
	writeMethodFrame(t, conn, 0, 10, 11, nil)
	readFrame(t, conn)
	writeMethodFrame(t, conn, 0, 10, 31, make([]byte, 8))
	writeMethodFrame(t, conn, 0, 10, 40, shortStr(""))
	readFrame(t, conn)
	writeMethodFrame(t, conn, 1, 20, 10, nil)
	readFrame(t, conn)
	queueArgs := append([]byte{0, 0}, shortStr("fault-test")...)
	writeMethodFrame(t, conn, 1, 50, 10, queueArgs)
	readFrame(t, conn)
	consumeArgs := append([]byte{0, 0}, shortStr("fault-test")...)
	consumeArgs = append(consumeArgs, shortStr("ctag")...)
	writeMethodFrame(t, conn, 1, 60, 20, consumeArgs)
	readFrame(t, conn)
	return conn
}

func publishAMQP(t *testing.T, conn net.Conn) {
	t.Helper()
	args := append([]byte{0, 0}, shortStr("test")...)
	args = append(args, shortStr("rk")...)
	writeMethodFrame(t, conn, 1, 60, 40, args)
	body := []byte("ping")
	writeHeaderFrame(t, conn, 1, 60, body)
	writeFrame(t, conn, 3, 1, body)
}

func readDeliveryBody(t *testing.T, conn net.Conn) string {
	t.Helper()
	frameType, _, payload := readFrame(t, conn)
	if frameType != 1 || binary.BigEndian.Uint16(payload[0:2]) != 60 || binary.BigEndian.Uint16(payload[2:4]) != 60 {
		t.Fatalf("unexpected AMQP method frame: type=%d payload=%v", frameType, payload)
	}
	frameType, _, _ = readFrame(t, conn)
	if frameType != 2 {
		t.Fatalf("unexpected AMQP header frame type %d", frameType)
	}
	frameType, _, payload = readFrame(t, conn)
	if frameType != 3 {
		t.Fatalf("unexpected AMQP body frame type %d", frameType)
	}
	return string(payload)
}

func TestAMQPServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := amqpserver.New(&config.AMQPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.AMQPMock{{
			ID:         "m",
			Exchange:   "test",
			RoutingKey: "rk",
			Response:   &config.AMQPResponse{Body: "hello"},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	conn := openAMQPConnection(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck

	sc.SetDirectFaults(config.ProtocolFaults{AMQP: &config.AMQPFault{ErrorRate: 0}})
	publishAMQP(t, conn)
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set fault read deadline: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("faulted publish unexpectedly delivered data")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("faulted publish read error = %v, want timeout", err)
	}

	sc.ClearDirectFaults()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set normal read deadline: %v", err)
	}
	publishAMQP(t, conn)
	if body := readDeliveryBody(t, conn); body != "hello" {
		t.Fatalf("normal AMQP delivery = %q, want %q", body, "hello")
	}
}

func TestAMQPServer_AMQPFault_FromScenario(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New([]config.Scenario{{ID: "drop", Faults: &config.ProtocolFaults{AMQP: &config.AMQPFault{ErrorRate: 0}}}})
	srv := amqpserver.New(&config.AMQPConfig{Enabled: true, Port: port, Mocks: []config.AMQPMock{{ID: "m", Exchange: "test", RoutingKey: "rk", Response: &config.AMQPResponse{Body: "hello"}}}}, state.New(), sc, logger.New(100))
	startServer(t, srv)
	sc.Activate("drop")

	conn := openAMQPConnection(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck
	publishAMQP(t, conn)
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("scenario fault unexpectedly delivered data")
	}
}
