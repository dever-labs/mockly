// Integration tests for the Redis server – sends raw RESP commands over TCP.
package redisserver

import (
	"bufio"
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

func newRedisServer(mocks []config.RedisMock) (*Server, int) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg := &config.RedisConfig{Enabled: true, Port: port, Mocks: mocks}
	return New(cfg, state.New(), logger.New(10)), port
}

func startRedis(t *testing.T, srv *Server) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Start(ctx) //nolint:errcheck

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

// respCmd encodes a RESP array command.
func respCmd(args ...string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&sb, "$%d\r\n%s\r\n", len(a), a)
	}
	return sb.String()
}

// respRoundTrip sends a command and returns the first response line.
func respRoundTrip(t *testing.T, port int, cmd string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial redis: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := conn.Write([]byte(cmd)); err != nil {
		t.Fatalf("write: %v", err)
	}

	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

func TestRedisServer_PING(t *testing.T) {
	srv, port := newRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("PING"))
	if line != "+PONG" {
		t.Errorf("want '+PONG', got %q", line)
	}
}

func TestRedisServer_PING_WithMessage(t *testing.T) {
	srv, port := newRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("PING", "hello"))
	// Response should be a bulk string $5\r\nhello
	if line != "$5" {
		t.Errorf("want '$5', got %q", line)
	}
}

func TestRedisServer_SELECT(t *testing.T) {
	srv, port := newRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("SELECT", "1"))
	if line != "+OK" {
		t.Errorf("want '+OK', got %q", line)
	}
}

func TestRedisServer_FLUSHDB(t *testing.T) {
	srv, port := newRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("FLUSHDB"))
	if line != "+OK" {
		t.Errorf("want '+OK', got %q", line)
	}
}

func TestRedisServer_COMMAND(t *testing.T) {
	srv, port := newRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("COMMAND"))
	if line != "+OK" {
		t.Errorf("want '+OK', got %q", line)
	}
}

func TestRedisServer_MockedGET(t *testing.T) {
	mocks := []config.RedisMock{
		{ID: "m1", Command: "GET", Key: "session:123",
			Response: config.RedisResponse{Type: "string", Value: "token-abc"}},
	}
	srv, port := newRedisServer(mocks)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("GET", "session:123"))
	// Bulk string response: $9
	if !strings.HasPrefix(line, "$") {
		t.Errorf("want bulk string response, got %q", line)
	}
}

func TestRedisServer_NoMatchReturnsError(t *testing.T) {
	srv, port := newRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("GET", "unknown-key"))
	if !strings.HasPrefix(line, "-ERR") {
		t.Errorf("want error response for unmatched key, got %q", line)
	}
}

func TestRedisServer_MockedGET_IntegerResponse(t *testing.T) {
	mocks := []config.RedisMock{
		{ID: "m1", Command: "GET", Key: "count",
			Response: config.RedisResponse{Type: "integer", Value: float64(42)}},
	}
	srv, port := newRedisServer(mocks)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("GET", "count"))
	if line != ":42" {
		t.Errorf("want ':42', got %q", line)
	}
}

func TestRedisServer_WildcardCommand(t *testing.T) {
	mocks := []config.RedisMock{
		{ID: "m1", Command: "*", Key: "any:key",
			Response: config.RedisResponse{Type: "string", Value: "wildcard-hit"}},
	}
	srv, port := newRedisServer(mocks)
	stop := startRedis(t, srv)
	defer stop()

	line := respRoundTrip(t, port, respCmd("SET", "any:key", "value"))
	if !strings.HasPrefix(line, "$") {
		t.Errorf("want bulk string for wildcard command, got %q", line)
	}
}

func TestRedisServer_SetMocks_GetMocks(t *testing.T) {
	srv, _ := newRedisServer(nil)
	mocks := []config.RedisMock{{ID: "m1", Command: "GET", Key: "x"}}
	srv.SetMocks(mocks)
	got := srv.GetMocks()
	if len(got) != 1 || got[0].ID != "m1" {
		t.Errorf("unexpected mocks: %+v", got)
	}
}
