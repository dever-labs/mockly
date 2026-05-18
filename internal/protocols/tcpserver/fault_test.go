package tcpserver_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/tcpserver"
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

func TestTCPServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := tcpserver.New(&config.TCPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.TCPMock{{
			ID:       "m",
			Match:    "PING",
			Response: "PONG",
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{TCP: &config.TCPFault{ErrorRate: 0}})

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial faulted server: %v", err)
	}
	conn.SetDeadline(time.Now().Add(time.Second))
	_, err = conn.Write([]byte("PING"))
	if err != nil {
		t.Fatalf("write faulted request: %v", err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	_ = conn.Close()
	if n != 0 {
		t.Fatalf("faulted read returned %d bytes, want 0", n)
	}
	if err == nil {
		t.Fatal("faulted read returned nil error, want connection close")
	}
	if err != io.EOF {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			t.Fatalf("faulted read timed out instead of closing: %v", err)
		}
	}

	sc.ClearDirectFaults()

	conn, err = net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial normal server: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte("PING")); err != nil {
		t.Fatalf("write normal request: %v", err)
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("read normal response: %v", err)
	}
	if got := string(buf[:n]); got != "PONG" {
		t.Fatalf("normal response = %q, want %q", got, "PONG")
	}
}

func TestTCPServer_GlobalFault_Delay(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := tcpserver.New(&config.TCPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.TCPMock{{
			ID:       "m",
			Match:    "PING",
			Response: "PONG",
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	sc.SetDirectFaults(config.ProtocolFaults{TCP: &config.TCPFault{Delay: config.Duration{Duration: 100 * time.Millisecond}}})
	defer sc.ClearDirectFaults()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	t0 := time.Now()
	if _, err := conn.Write([]byte("PING")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	elapsed := time.Since(t0)
	if err != nil && err != io.EOF {
		t.Fatalf("read: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected delayed close, got %q", string(buf[:n]))
	}
	if elapsed < 80*time.Millisecond {
		t.Fatalf("expected at least 80ms delay, got %v", elapsed)
	}
}

func TestTCPServer_TCPFault_ResponseBeforeClose(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := tcpserver.New(&config.TCPConfig{Enabled: true, Port: port, Mocks: []config.TCPMock{{ID: "m", Match: "PING", Response: "PONG"}}}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	sc.SetDirectFaults(config.ProtocolFaults{TCP: &config.TCPFault{Response: "ERR", ErrorRate: 0}})
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte("PING")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "ERR" {
		t.Fatalf("response = %q, want ERR", got)
	}
}
