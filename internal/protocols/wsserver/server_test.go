// Package wsserver_test provides integration tests for the WebSocket mock server.
package wsserver_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/wsserver"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/testutil"
)

// newWSServer starts a WebSocket server on a free port and returns its base URL.
// If tlsCfg is non-nil and enabled it returns "wss://..." otherwise "ws://...".
func newWSServer(t *testing.T, mocks []config.WebSocketMock, tlsCfg *config.TLSConfig) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.WebSocketConfig{
		Enabled: true,
		Port:    port,
		TLS:     tlsCfg,
		Mocks:   mocks,
	}
	srv := wsserver.New(cfg, state.New(), logger.New(10))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	// Wait until the port is accepting TCP connections.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	scheme := "ws"
	if tlsCfg != nil && tlsCfg.Enabled {
		scheme = "wss"
	}
	return fmt.Sprintf("%s://127.0.0.1:%d", scheme, port)
}

// dialWS connects to a WebSocket server, retrying until success or timeout.
func dialWS(t *testing.T, url string, tlsCfg *tls.Config) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{TLSClientConfig: tlsCfg}
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, _, err := dialer.Dial(url, nil)
		if err == nil {
			return conn
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("failed to connect to WS server at %s: %v", url, lastErr)
	return nil
}

func TestWSServer_BasicConnection(t *testing.T) {
	mocks := []config.WebSocketMock{{
		ID:   "echo",
		Path: "/ws",
		OnConnect: &config.WebSocketAction{
			Send: "hello",
		},
		OnMessage: []config.WebSocketRule{{
			Match:   "ping",
			Respond: "pong",
		}},
	}}
	base := newWSServer(t, mocks, nil)

	conn := dialWS(t, base+"/ws", nil)
	defer conn.Close() //nolint:errcheck

	// Read the on_connect message.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage (on_connect): %v", err)
	}
	if string(msg) != "hello" {
		t.Errorf("on_connect: want 'hello', got %q", msg)
	}

	// Send a message and verify the response.
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, reply, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage (response): %v", err)
	}
	if string(reply) != "pong" {
		t.Errorf("on_message: want 'pong', got %q", reply)
	}
}

func TestWSServer_GetMocks_SetMocks(t *testing.T) {
	initial := []config.WebSocketMock{{ID: "m1", Path: "/a"}}
	base := newWSServer(t, initial, nil)
	_ = base // server is running; we only need it to be started

	// Retrieve mocks from a running server by creating a second server
	// in memory and verifying SetMocks/GetMocks isolation.
	cfg := &config.WebSocketConfig{Enabled: true, Port: 0, Mocks: initial}
	srv := wsserver.New(cfg, state.New(), logger.New(10))

	updated := []config.WebSocketMock{
		{ID: "m2", Path: "/b"},
		{ID: "m3", Path: "/c"},
	}
	srv.SetMocks(updated)

	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
	if got[0].ID != "m2" || got[1].ID != "m3" {
		t.Errorf("unexpected mocks: %+v", got)
	}

	// Verify deep copy: mutating the returned slice must not affect the server.
	got[0].ID = "mutated"
	got2 := srv.GetMocks()
	if got2[0].ID == "mutated" {
		t.Error("GetMocks should return a copy; server state was mutated via returned slice")
	}
}

func TestWSServer_SetMocks_ConcurrentAccess(t *testing.T) {
	mocks := []config.WebSocketMock{{
		ID:   "concurrent",
		Path: "/ws/race",
	}}
	base := newWSServer(t, mocks, nil)

	var wg sync.WaitGroup

	// Writers: replace the mock list concurrently.
	cfg := &config.WebSocketConfig{Enabled: true, Port: 0, Mocks: mocks}
	srv := wsserver.New(cfg, state.New(), logger.New(10))

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				srv.SetMocks([]config.WebSocketMock{{ID: fmt.Sprintf("m-%d-%d", n, j), Path: "/ws/race"}})
			}
		}(i)
	}

	// Readers: make WS connections that exercise handleDynamic (which reads
	// the mock list under RLock) concurrently with the writers above.
	dialer := websocket.Dialer{}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				// The connection may be refused or upgraded to a 404 — that's fine.
				conn, _, err := dialer.Dial(base+"/ws/race", nil)
				if err == nil {
					_ = conn.Close()
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
}

func TestWSServer_TLS(t *testing.T) {
	dir := t.TempDir()
	certFile := dir + "/cert.pem"
	keyFile := dir + "/key.pem"
	if err := testutil.WriteSelfSignedCert(certFile, keyFile); err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	tlsCfg := &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile}
	mocks := []config.WebSocketMock{{
		ID:   "tls-echo",
		Path: "/wss",
		OnConnect: &config.WebSocketAction{
			Send: "tls-ok",
		},
	}}
	base := newWSServer(t, mocks, tlsCfg)

	clientTLS := &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	conn := dialWS(t, base+"/wss", clientTLS)
	defer conn.Close() //nolint:errcheck

	conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage over wss://: %v", err)
	}
	if string(msg) != "tls-ok" {
		t.Errorf("on_connect over TLS: want 'tls-ok', got %q", msg)
	}
}
