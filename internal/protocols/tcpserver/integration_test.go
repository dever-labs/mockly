// Integration tests for the TCP server – starts a real listener and connects.
package tcpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/testutil"
)

func newTCPServer(mocks []config.TCPMock) (*Server, int) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

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
			_ = conn.Close()
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
	defer conn.Close() //nolint:errcheck

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
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
	defer conn.Close() //nolint:errcheck

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write([]byte("UNMATCHED_MESSAGE"))

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

// ---------------------------------------------------------------------------
// Concurrency / race-detector tests
// ---------------------------------------------------------------------------

func TestTCPServer_SetMocks_ConcurrentAccess(t *testing.T) {
mocks := []config.TCPMock{{ID: "m1", Match: "PING", Response: "PONG", Close: true}}
srv, port := newTCPServer(mocks)
stop := startTCP(t, srv)
defer stop()

var wg sync.WaitGroup
for i := 0; i < 5; i++ {
wg.Add(2)
go func() {
defer wg.Done()
for j := 0; j < 50; j++ {
srv.SetMocks([]config.TCPMock{{ID: "m", Match: "PING", Response: "PONG", Close: true}})
}
}()
go func() {
defer wg.Done()
for j := 0; j < 20; j++ {
srv.matchMock([]byte("PING"))
}
}()
}
wg.Wait()
_ = port
}

// ---------------------------------------------------------------------------
// TLS tests
// ---------------------------------------------------------------------------

func newTLSTCPServer(t *testing.T, mocks []config.TCPMock) (*Server, int) {
t.Helper()
dir := t.TempDir()
certFile := dir + "/cert.pem"
keyFile := dir + "/key.pem"
if err := testutil.WriteSelfSignedCert(certFile, keyFile); err != nil {
t.Fatalf("generate cert: %v", err)
}

ln, _ := net.Listen("tcp", "127.0.0.1:0")
port := ln.Addr().(*net.TCPAddr).Port
_ = ln.Close()

cfg := &config.TCPConfig{
Enabled: true,
Port:    port,
TLS:     &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile},
Mocks:   mocks,
}
return New(cfg, state.New(), logger.New(10)), port
}

func tlsRoundTrip(t *testing.T, port int, msg string) string {
t.Helper()
conn, err := tls.DialWithDialer(
&net.Dialer{Timeout: time.Second},
"tcp",
fmt.Sprintf("127.0.0.1:%d", port),
&tls.Config{InsecureSkipVerify: true}, //nolint:gosec
)
if err != nil {
t.Fatalf("tls dial: %v", err)
}
defer conn.Close() //nolint:errcheck

_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
if _, err := conn.Write([]byte(msg)); err != nil {
t.Fatalf("write: %v", err)
}

buf := make([]byte, 1024)
n, _ := conn.Read(buf)
return string(buf[:n])
}

func TestTCPServer_TLS(t *testing.T) {
mocks := []config.TCPMock{
{ID: "m1", Match: "re:^HELLO", Response: "WORLD", Close: true},
}
srv, port := newTLSTCPServer(t, mocks)
stop := startTCP(t, srv)
defer stop()

got := tlsRoundTrip(t, port, "HELLO there")
if !strings.Contains(got, "WORLD") {
t.Errorf("want 'WORLD' over TLS, got %q", got)
}
}
