package stompserver_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/stompserver"
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
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial stomp: %v", err)
	}
	conn.SetDeadline(time.Now().Add(2 * time.Second))
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

func TestSTOMPServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := stompserver.New(&config.STOMPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.STOMPMock{{
			ID:          "m",
			Destination: "/queue/test",
			Response:    &config.STOMPResponse{Body: "hello"},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{STOMP: &config.STOMPFault{ErrorRate: 0}})

	conn, reader := dialSTOMP(t, addr)
	if _, err := io.WriteString(conn, "SEND\ndestination:/queue/test\ncontent-length:5\n\nhello\x00"); err != nil {
		t.Fatalf("write fault SEND: %v", err)
	}
	faultFrame := readSTOMPFrame(t, reader)
	_ = conn.Close()
	if !strings.HasPrefix(faultFrame, "ERROR\n") {
		t.Fatalf("fault frame = %q, want ERROR", faultFrame)
	}
	if !strings.Contains(faultFrame, "message:fault injected") {
		t.Fatalf("fault frame = %q, want fault message header", faultFrame)
	}

	sc.ClearDirectFaults()

	conn, reader = dialSTOMP(t, addr)
	defer conn.Close() //nolint:errcheck
	if _, err := io.WriteString(conn, "SUBSCRIBE\nid:1\ndestination:/queue/test\nack:auto\n\n\x00"); err != nil {
		t.Fatalf("write SUBSCRIBE: %v", err)
	}
	if _, err := io.WriteString(conn, "SEND\ndestination:/queue/test\ncontent-length:5\n\nhello\x00"); err != nil {
		t.Fatalf("write normal SEND: %v", err)
	}
	normalFrame := readSTOMPFrame(t, reader)
	if !strings.HasPrefix(normalFrame, "MESSAGE\n") {
		t.Fatalf("normal frame = %q, want MESSAGE", normalFrame)
	}
	if !strings.Contains(normalFrame, "destination:/queue/test") {
		t.Fatalf("normal frame = %q, want destination header", normalFrame)
	}
	if !strings.HasSuffix(normalFrame, "\n\nhello") {
		t.Fatalf("normal frame = %q, want body hello", normalFrame)
	}
}

func TestSTOMPServer_STOMPFault_CustomMessage(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := stompserver.New(&config.STOMPConfig{Enabled: true, Port: port}, state.New(), sc, logger.New(100))
	startServer(t, srv)
	sc.SetDirectFaults(config.ProtocolFaults{STOMP: &config.STOMPFault{Message: "custom-error", ErrorRate: 0}})
	conn, reader := dialSTOMP(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck
	if _, err := io.WriteString(conn, "SEND\ndestination:/queue/test\ncontent-length:5\n\nhello\x00"); err != nil {
		t.Fatalf("write SEND: %v", err)
	}
	frame := readSTOMPFrame(t, reader)
	if !strings.Contains(frame, "message:custom-error") {
		t.Fatalf("fault frame = %q", frame)
	}
}
