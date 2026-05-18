package redisserver_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/redisserver"
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

func redisRequest(t *testing.T, addr string, payload string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial redis: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("write redis request: %v", err)
	}
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read redis response: %v", err)
	}
	return string(buf[:n])
}

func TestRedisServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := redisserver.New(&config.RedisConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.RedisMock{{
			ID:       "m",
			Command:  "GET",
			Key:      "foo",
			Response: config.RedisResponse{Type: "bulk", Value: "bar"},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	payload := "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"

	sc.SetDirectFaults(config.ProtocolFaults{Redis: &config.RedisFault{ErrorRate: 0}})
	faultResp := redisRequest(t, addr, payload)
	if !strings.HasPrefix(faultResp, "-ERR fault injected") {
		t.Fatalf("fault response = %q, want RESP error", faultResp)
	}

	sc.ClearDirectFaults()
	normalResp := redisRequest(t, addr, payload)
	if !strings.HasPrefix(normalResp, "$3\r\nbar\r\n") {
		t.Fatalf("normal response = %q, want bulk string", normalResp)
	}
}

func TestRedisServer_RedisFault_CustomError(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := redisserver.New(&config.RedisConfig{Enabled: true, Port: port}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	sc.SetDirectFaults(config.ProtocolFaults{Redis: &config.RedisFault{Error: "LOADING redis is loading", ErrorRate: 0}})
	resp := redisRequest(t, fmt.Sprintf("127.0.0.1:%d", port), "*1\r\n$4\r\nPING\r\n")
	if !strings.HasPrefix(resp, "-ERR LOADING redis is loading") {
		t.Fatalf("fault response = %q", resp)
	}
}
