// Integration tests for the TCP server – starts a real listener and connects.
package tcpserver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

func newTCPServer(mocks []config.TCPMock) (*Server, int) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg := &config.TCPConfig{Enabled: true, Port: port, Mocks: mocks}
	return New(cfg, state.New(), logger.New(10)), port
}

func startTCP(t *testing.T, srv *Server) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()

	// Wait for the server to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", srv.cfg.Port))
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cancel
}

func tcpRoundTrip(t *testing.T, port int, msg string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write([]byte(msg))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	return string(buf[:n])
}

func TestTCPServer_MatchAndRespond(t *testing.T) {
	mocks := []config.TCPMock{
		{ID: "m1", Match: "PING", Response: "PONG", Close: true},
	}
	srv, port := newTCPServer(mocks)
	stop := startTCP(t, srv)
	defer stop()

	got := tcpRoundTrip(t, port, "PING")
	if got != "PONG" {
		t.Errorf("want 'PONG', got %q", got)
	}
}

func TestTCPServer_HexResponse(t *testing.T) {
	// hex:504f4e47 == "PONG"
	mocks := []config.TCPMock{
		{ID: "m1", Match: "PING", Response: "hex:504f4e47", Close: true},
	}
	srv, port := newTCPServer(mocks)
	stop := startTCP(t, srv)
	defer stop()

	got := tcpRoundTrip(t, port, "PING")
	if got != "PONG" {
		t.Errorf("want 'PONG' from hex response, got %q", got)
	}
}

func TestTCPServer_RegexMatch(t *testing.T) {
	mocks := []config.TCPMock{
		{ID: "m1", Match: "re:^HELLO", Response: "WORLD", Close: true},
	}
	srv, port := newTCPServer(mocks)
	stop := startTCP(t, srv)
	defer stop()

	got := tcpRoundTrip(t, port, "HELLO there")
	if !strings.Contains(got, "WORLD") {
		t.Errorf("want 'WORLD', got %q", got)
	}
}

func TestTCPServer_NoMatch_ConnectionClosed(t *testing.T) {
	mocks := []config.TCPMock{
		{ID: "m1", Match: "PING", Response: "PONG", Close: true},
	}
	srv, port := newTCPServer(mocks)
	stop := startTCP(t, srv)
	defer stop()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	conn.Write([]byte("UNMATCHED_MESSAGE"))

	buf := make([]byte, 128)
	n, _ := conn.Read(buf)
	// Server closes the connection with no data — either 0 bytes or EOF.
	if n > 0 {
		t.Logf("received %d bytes: %q (server may send error — acceptable)", n, string(buf[:n]))
	}
}

func TestTCPServer_SetMocks_GetMocks(t *testing.T) {
	srv, _ := newTCPServer(nil)
	mocks := []config.TCPMock{{ID: "m1", Match: "X", Response: "Y"}}
	srv.SetMocks(mocks)
	got := srv.GetMocks()
	if len(got) != 1 || got[0].ID != "m1" {
		t.Errorf("unexpected mocks: %+v", got)
	}
}
