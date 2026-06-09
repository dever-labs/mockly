// White-box unit and integration tests for memcachedserver.
package memcachedserver

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
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// New / SetMocks / GetMocks
// ---------------------------------------------------------------------------

func newTestMemcachedServer(t *testing.T, mocks []config.MemcachedMock) *Server {
	t.Helper()
	cfg := &config.MemcachedConfig{Enabled: true, Port: 0, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

func TestMemcached_New_InitialMocks(t *testing.T) {
	mocks := []config.MemcachedMock{{ID: "m1", Command: "get", Key: "user:*"}}
	srv := newTestMemcachedServer(t, mocks)
	got := srv.GetMocks()
	if len(got) != 1 || got[0].ID != "m1" {
		t.Fatalf("unexpected mocks from New: %+v", got)
	}
}

func TestMemcached_SetMocks_ReplacesList(t *testing.T) {
	srv := newTestMemcachedServer(t, nil)
	srv.SetMocks([]config.MemcachedMock{{ID: "a"}, {ID: "b"}})
	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
}

func TestMemcached_SetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestMemcachedServer(t, nil)
	original := []config.MemcachedMock{{ID: "orig", Key: "x"}}
	srv.SetMocks(original)
	original[0].Key = "mutated"
	if srv.GetMocks()[0].Key != "x" {
		t.Error("SetMocks should copy the slice")
	}
}

func TestMemcached_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestMemcachedServer(t, []config.MemcachedMock{{ID: "m1", Key: "orig"}})
	got := srv.GetMocks()
	got[0].Key = "mutated"
	if srv.GetMocks()[0].Key != "orig" {
		t.Error("GetMocks should return a copy")
	}
}

// ---------------------------------------------------------------------------
// matchMemcachedKey (extra cases)
// ---------------------------------------------------------------------------

func TestMatchMemcachedKey_Empty_MatchesAll(t *testing.T) {
	if !matchMemcachedKey("", "anything") {
		t.Error("empty pattern should match everything")
	}
}

func TestMatchMemcachedKey_WildcardOnly_MatchesAll(t *testing.T) {
	if !matchMemcachedKey("*", "anything") {
		t.Error("'*' should match everything")
	}
}

func TestMatchMemcachedKey_Exact(t *testing.T) {
	if !matchMemcachedKey("user:1", "user:1") {
		t.Error("exact should match")
	}
	if matchMemcachedKey("user:1", "user:2") {
		t.Error("exact should not match different key")
	}
}

func TestMatchMemcachedKey_InvalidRegex(t *testing.T) {
	if matchMemcachedKey(`re:[invalid`, "anything") {
		t.Error("invalid regex should not match")
	}
}

func TestMatchMemcachedKey_WildcardSuffix(t *testing.T) {
	if !matchMemcachedKey("cache:*", "cache:user") {
		t.Error("prefix wildcard should match")
	}
	if matchMemcachedKey("cache:*", "other:user") {
		t.Error("prefix wildcard should not match wrong prefix")
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestMemcached_matchMock_CommandMatch(t *testing.T) {
	srv := newTestMemcachedServer(t, []config.MemcachedMock{
		{ID: "m1", Command: "get", Key: "user:*", Response: config.MemcachedResponse{Value: "alice"}},
	})
	m, ok := srv.matchMock("get", "user:1")
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match, got ok=%v id=%q", ok, m.ID)
	}
}

func TestMemcached_matchMock_CommandMismatch(t *testing.T) {
	srv := newTestMemcachedServer(t, []config.MemcachedMock{
		{ID: "m1", Command: "get", Key: "user:*", Response: config.MemcachedResponse{Value: "alice"}},
	})
	_, ok := srv.matchMock("set", "user:1")
	if ok {
		t.Fatal("should not match different command")
	}
}

func TestMemcached_matchMock_WildcardCommand(t *testing.T) {
	srv := newTestMemcachedServer(t, []config.MemcachedMock{
		{ID: "m1", Command: "*", Key: "user:*", Response: config.MemcachedResponse{Value: "any"}},
	})
	m, ok := srv.matchMock("delete", "user:1")
	if !ok || m.ID != "m1" {
		t.Fatalf("wildcard command should match any command, got ok=%v", ok)
	}
}

func TestMemcached_matchMock_StateCondition_NotMet(t *testing.T) {
	st := state.New()
	cfg := &config.MemcachedConfig{
		Mocks: []config.MemcachedMock{{
			ID:      "m1",
			Command: "get",
			Key:     "key",
			State:   &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, scenarios.New(nil), logger.New(10))
	_, ok := srv.matchMock("get", "key")
	if ok {
		t.Fatal("should not match when state condition not met")
	}
}

func TestMemcached_matchMock_StateCondition_Met(t *testing.T) {
	st := state.New()
	st.Set("mode", "on")
	cfg := &config.MemcachedConfig{
		Mocks: []config.MemcachedMock{{
			ID:      "m1",
			Command: "get",
			Key:     "key",
			State:   &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, scenarios.New(nil), logger.New(10))
	m, ok := srv.matchMock("get", "key")
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match when state condition met, got ok=%v", ok)
	}
}

// ---------------------------------------------------------------------------
// StatusInfo
// ---------------------------------------------------------------------------

func TestMemcached_StatusInfo(t *testing.T) {
	srv := newTestMemcachedServer(t, []config.MemcachedMock{{ID: "m1"}, {ID: "m2"}})
	info := srv.StatusInfo()
	if info["protocol"] != "memcached" {
		t.Errorf("unexpected protocol %v", info["protocol"])
	}
	if info["mocks"] != 2 {
		t.Errorf("want mocks=2, got %v", info["mocks"])
	}
}

// ---------------------------------------------------------------------------
// Integration: Memcached server over TCP
// ---------------------------------------------------------------------------

func startMemcachedServer(t *testing.T, mocks []config.MemcachedMock) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	time.Sleep(10 * time.Millisecond)

	cfg := &config.MemcachedConfig{Enabled: true, Port: port, Mocks: mocks}
	srv := New(cfg, state.New(), scenarios.New(nil), logger.New(100))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(100 * time.Millisecond)
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func memcachedCmd(t *testing.T, conn net.Conn, r *bufio.Reader, cmd string) string {
	t.Helper()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = fmt.Fprintf(conn, "%s\r\n", cmd)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("memcachedCmd %q read error: %v", cmd, err)
	}
	return strings.TrimRight(line, "\r\n")
}

func TestMemcachedServer_Version(t *testing.T) {
	addr := startMemcachedServer(t, nil)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	resp := memcachedCmd(t, conn, r, "version")
	if !strings.HasPrefix(resp, "VERSION") {
		t.Errorf("want VERSION response, got %q", resp)
	}
}

func TestMemcachedServer_Stats(t *testing.T) {
	addr := startMemcachedServer(t, nil)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	memcachedCmd(t, conn, r, "stats")
	// Read all stats lines until END.
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading stats: %v", err)
		}
		if strings.TrimRight(line, "\r\n") == "END" {
			break
		}
	}
}

func TestMemcachedServer_FlushAll(t *testing.T) {
	addr := startMemcachedServer(t, nil)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	resp := memcachedCmd(t, conn, r, "flush_all")
	if resp != "OK" {
		t.Errorf("want OK, got %q", resp)
	}
}

func TestMemcachedServer_Get_WithMock(t *testing.T) {
	addr := startMemcachedServer(t, []config.MemcachedMock{
		{
			ID:      "m1",
			Command: "get",
			Key:     "user:*",
			Response: config.MemcachedResponse{
				Value: "alice",
				Flags: 0,
			},
		},
	})
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = fmt.Fprintf(conn, "get user:1\r\n")

	// Read VALUE line.
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read VALUE: %v", err)
	}
	if !strings.HasPrefix(strings.TrimRight(line, "\r\n"), "VALUE user:1") {
		t.Errorf("want VALUE user:1, got %q", line)
	}
	// Read value data.
	data, _ := r.ReadString('\n')
	if strings.TrimRight(data, "\r\n") != "alice" {
		t.Errorf("want 'alice', got %q", data)
	}
	// Read END.
	end, _ := r.ReadString('\n')
	if strings.TrimRight(end, "\r\n") != "END" {
		t.Errorf("want END, got %q", end)
	}
}

func TestMemcachedServer_Get_NoMock(t *testing.T) {
	addr := startMemcachedServer(t, nil)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = fmt.Fprintf(conn, "get nokey\r\n")
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.TrimRight(line, "\r\n") != "END" {
		t.Errorf("missing key should return END, got %q", line)
	}
}

func TestMemcachedServer_Set(t *testing.T) {
	addr := startMemcachedServer(t, []config.MemcachedMock{
		{
			ID:       "m1",
			Command:  "set",
			Key:      "*",
			Response: config.MemcachedResponse{Status: "STORED"},
		},
	})
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = fmt.Fprintf(conn, "set mykey 0 0 5\r\nhello\r\n")
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read SET response: %v", err)
	}
	if strings.TrimRight(line, "\r\n") != "STORED" {
		t.Errorf("want STORED, got %q", line)
	}
}

func TestMemcachedServer_Delete(t *testing.T) {
	addr := startMemcachedServer(t, []config.MemcachedMock{
		{
			ID:       "m1",
			Command:  "delete",
			Key:      "user:*",
			Response: config.MemcachedResponse{Status: "DELETED"},
		},
	})
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	resp := memcachedCmd(t, conn, r, "delete user:1")
	if resp != "DELETED" {
		t.Errorf("want DELETED, got %q", resp)
	}
}

func TestMemcachedServer_Delete_NoMock(t *testing.T) {
	addr := startMemcachedServer(t, nil)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	resp := memcachedCmd(t, conn, r, "delete anything")
	if resp != "DELETED" {
		t.Errorf("delete without mock: want DELETED, got %q", resp)
	}
}

func TestMemcachedServer_UnknownCommand(t *testing.T) {
	addr := startMemcachedServer(t, nil)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	resp := memcachedCmd(t, conn, r, "frobnicate key")
	if resp != "ERROR" {
		t.Errorf("unknown cmd: want ERROR, got %q", resp)
	}
}

func TestMemcachedServer_Quit(t *testing.T) {
	addr := startMemcachedServer(t, nil)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	r := bufio.NewReader(conn)

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = fmt.Fprintf(conn, "quit\r\n")
	// After quit the server closes the connection.
	buf := make([]byte, 1)
	_, err = r.Read(buf)
	if err == nil {
		t.Fatal("expected connection closed after quit")
	}
}
