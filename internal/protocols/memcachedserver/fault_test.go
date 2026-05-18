package memcachedserver_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/memcachedserver"
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

func memcachedRequest(t *testing.T, addr string, payload string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial memcached: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("write memcached request: %v", err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read memcached response: %v", err)
	}
	return string(buf[:n])
}

func TestMemcachedServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := memcachedserver.New(&config.MemcachedConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.MemcachedMock{{
			ID:       "m",
			Command:  "get",
			Key:      "foo",
			Response: config.MemcachedResponse{Value: "bar"},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{Memcached: &config.MemcachedFault{ErrorRate: 0}})
	faultResp := memcachedRequest(t, addr, "get foo\r\n")
	if !strings.Contains(faultResp, "SERVER_ERROR fault injected") {
		t.Fatalf("fault response = %q, want SERVER_ERROR", faultResp)
	}

	sc.ClearDirectFaults()
	normalResp := memcachedRequest(t, addr, "get foo\r\n")
	if !strings.Contains(normalResp, "VALUE foo 0 3\r\nbar\r\nEND\r\n") {
		t.Fatalf("normal response = %q, want VALUE payload", normalResp)
	}
}

func TestMemcachedServer_MemcachedFault_ClientError(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := memcachedserver.New(&config.MemcachedConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.MemcachedMock{{
			ID:      "m",
			Command: "get",
			Key:     "foo",
			Response: config.MemcachedResponse{
				Value: "bar",
			},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)
	sc.SetDirectFaults(config.ProtocolFaults{Memcached: &config.MemcachedFault{ErrorType: "CLIENT_ERROR", Message: "bad command", ErrorRate: 0}})
	resp := memcachedRequest(t, fmt.Sprintf("127.0.0.1:%d", port), "get foo\r\n")
	if !strings.Contains(resp, "CLIENT_ERROR bad command") {
		t.Fatalf("fault response = %q", resp)
	}
}
