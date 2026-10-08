//go:build e2e

package e2e

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestE2E_TCP_MockAndRequest proves that a `protocols.tcp` block in
// mockly.yaml is actually wired up by the real binary: it starts mockly
// with a single TCP mock, connects with a raw TCP client, sends the exact
// match pattern, and asserts the configured response bytes come back. This
// exercises cmd/mockly/main.go's `if cfg.Protocols.TCP.Enabled` startup
// block end-to-end, which no existing unit test (which constructs the
// tcpserver.Server directly) covers.
func TestE2E_TCP_MockAndRequest(t *testing.T) {
	tcpPort := freePort(t)
	cfg := fmt.Sprintf(`
protocols:
  tcp:
    enabled: true
    port: %d
    mocks:
      - id: ping-pong
        match: "PING"
        response: "PONG"
`, tcpPort)

	startMocklyConfig(t, cfg)

	conn := dialWithRetry(t, "tcp", fmt.Sprintf("127.0.0.1:%d", tcpPort), 5*time.Second)
	defer conn.Close() //nolint:errcheck

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("PING")); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "PONG" {
		t.Errorf("want 'PONG', got %q", got)
	}
}

// respCmd encodes a RESP array command, matching the wire format real Redis
// clients send and that internal/protocols/redisserver's own integration
// tests use to drive the server directly.
func respCmd(args ...string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&sb, "$%d\r\n%s\r\n", len(a), a)
	}
	return sb.String()
}

// TestE2E_Redis_MockAndRequest proves that a `protocols.redis` block in
// mockly.yaml is actually wired up by the real binary: it starts mockly
// with a GET mock for a specific key, connects with a hand-crafted RESP
// client over raw TCP (the same approach redisserver's own integration
// tests use), and asserts the exact RESP bulk-string reply configured in
// the mock. This exercises cmd/mockly/main.go's
// `if cfg.Protocols.Redis.Enabled` startup block end-to-end.
func TestE2E_Redis_MockAndRequest(t *testing.T) {
	redisPort := freePort(t)
	cfg := fmt.Sprintf(`
protocols:
  redis:
    enabled: true
    port: %d
    mocks:
      - id: get-session
        command: GET
        key: "session:123"
        response:
          type: string
          value: "token-abc"
`, redisPort)

	startMocklyConfig(t, cfg)

	conn := dialWithRetry(t, "tcp", fmt.Sprintf("127.0.0.1:%d", redisPort), 5*time.Second)
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := conn.Write([]byte(respCmd("GET", "session:123"))); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	// Bulk string reply: "$9\r\ntoken-abc\r\n" — read both lines to assert
	// the length header and the actual payload.
	lenLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read length line: %v", err)
	}
	lenLine = strings.TrimRight(lenLine, "\r\n")
	if lenLine != "$9" {
		t.Fatalf("want bulk length '$9', got %q", lenLine)
	}
	valLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read value line: %v", err)
	}
	valLine = strings.TrimRight(valLine, "\r\n")
	if valLine != "token-abc" {
		t.Errorf("want 'token-abc', got %q", valLine)
	}
}

// TestE2E_Memcached_MockAndRequest proves that a `protocols.memcached`
// block in mockly.yaml is actually wired up by the real binary: it starts
// mockly with a GET mock for a specific key, connects with a raw
// text-protocol client, and asserts the exact "VALUE ... / END" reply
// memcached clients expect. This exercises cmd/mockly/main.go's
// `if cfg.Protocols.Memcached.Enabled` startup block end-to-end.
func TestE2E_Memcached_MockAndRequest(t *testing.T) {
	mcPort := freePort(t)
	cfg := fmt.Sprintf(`
protocols:
  memcached:
    enabled: true
    port: %d
    mocks:
      - id: get-user
        command: get
        key: "user:42"
        response:
          value: "alice"
`, mcPort)

	startMocklyConfig(t, cfg)

	conn := dialWithRetry(t, "tcp", fmt.Sprintf("127.0.0.1:%d", mcPort), 5*time.Second)
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := conn.Write([]byte("get user:42\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	// Expected reply: "VALUE user:42 0 5\r\nalice\r\nEND\r\n"
	headerLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read header line: %v", err)
	}
	headerLine = strings.TrimRight(headerLine, "\r\n")
	if headerLine != "VALUE user:42 0 5" {
		t.Fatalf("want 'VALUE user:42 0 5', got %q", headerLine)
	}
	valLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read value line: %v", err)
	}
	valLine = strings.TrimRight(valLine, "\r\n")
	if valLine != "alice" {
		t.Errorf("want 'alice', got %q", valLine)
	}
	endLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read END line: %v", err)
	}
	endLine = strings.TrimRight(endLine, "\r\n")
	if endLine != "END" {
		t.Errorf("want 'END', got %q", endLine)
	}
}

// TestE2E_WebSocket_MockAndRequest proves that a `protocols.websocket`
// block in mockly.yaml is actually wired up by the real binary: it starts
// mockly with an on_message match/respond mock, connects with the real
// gorilla/websocket client library, sends a text message, and asserts the
// mocked reply comes back. This exercises cmd/mockly/main.go's
// `if cfg.Protocols.WebSocket.Enabled` startup block end-to-end.
func TestE2E_WebSocket_MockAndRequest(t *testing.T) {
	wsPort := freePort(t)
	cfg := fmt.Sprintf(`
protocols:
  websocket:
    enabled: true
    port: %d
    mocks:
      - id: echo
        path: /ws
        on_message:
          - match: "ping"
            respond: "pong"
`, wsPort)

	startMocklyConfig(t, cfg)

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", wsPort)
	var conn *websocket.Conn
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			conn = c
			break
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	if conn == nil {
		t.Fatalf("dial %s: %v", wsURL, lastErr)
	}
	defer conn.Close() //nolint:errcheck

	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("write message: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if string(msg) != "pong" {
		t.Errorf("want 'pong', got %q", msg)
	}
}
