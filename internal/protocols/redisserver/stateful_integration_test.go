// Integration tests for Redis "stateful" mode — sends raw RESP commands
// over TCP against the real in-memory datastore.
package redisserver

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

func newStatefulRedisServer(mocks []config.RedisMock) (*Server, int) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.RedisConfig{Enabled: true, Port: port, Mode: "stateful", Mocks: mocks}
	return New(cfg, state.New(), nil, logger.New(10)), port
}

// respRoundTripLines sends cmd and reads exactly n lines of the response,
// for replies that span multiple RESP lines (e.g. bulk strings, arrays).
func respRoundTripLines(t *testing.T, port int, cmd string, n int) []string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial redis: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := conn.Write([]byte(cmd)); err != nil {
		t.Fatalf("write: %v", err)
	}

	scanner := bufio.NewScanner(conn)
	lines := make([]string, 0, n)
	for i := 0; i < n && scanner.Scan(); i++ {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func startStateful(t *testing.T) (port int, stop func()) {
	t.Helper()
	srv, port := newStatefulRedisServer(nil)
	stop = startRedis(t, srv)
	return port, stop
}

func TestStateful_SetGet_RoundTrips(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	line := respRoundTrip(t, port, respCmd("SET", "foo", "bar"))
	if line != "+OK" {
		t.Fatalf("SET: want '+OK', got %q", line)
	}

	lines := respRoundTripLines(t, port, respCmd("GET", "foo"), 2)
	if len(lines) != 2 || lines[0] != "$3" || lines[1] != "bar" {
		t.Fatalf("GET: want [$3 bar], got %v", lines)
	}
}

func TestStateful_Get_MissingKeyReturnsNull(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	line := respRoundTrip(t, port, respCmd("GET", "missing"))
	if line != "$-1" {
		t.Fatalf("want '$-1' (nil), got %q", line)
	}
}

func TestStateful_Del_Exists(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "v"))

	line := respRoundTrip(t, port, respCmd("EXISTS", "k"))
	if line != ":1" {
		t.Fatalf("EXISTS present: want ':1', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("DEL", "k"))
	if line != ":1" {
		t.Fatalf("DEL: want ':1', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("EXISTS", "k"))
	if line != ":0" {
		t.Fatalf("EXISTS after delete: want ':0', got %q", line)
	}
}

func TestStateful_Incr_Decr(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	line := respRoundTrip(t, port, respCmd("INCR", "counter"))
	if line != ":1" {
		t.Fatalf("INCR from absent: want ':1', got %q", line)
	}
	line = respRoundTrip(t, port, respCmd("INCRBY", "counter", "5"))
	if line != ":6" {
		t.Fatalf("INCRBY: want ':6', got %q", line)
	}
	line = respRoundTrip(t, port, respCmd("DECR", "counter"))
	if line != ":5" {
		t.Fatalf("DECR: want ':5', got %q", line)
	}
	line = respRoundTrip(t, port, respCmd("DECRBY", "counter", "2"))
	if line != ":3" {
		t.Fatalf("DECRBY: want ':3', got %q", line)
	}
}

func TestStateful_Incr_NonIntegerValueErrors(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "not-a-number"))
	line := respRoundTrip(t, port, respCmd("INCR", "k"))
	if line[0] != '-' {
		t.Fatalf("expected RESP error for non-integer INCR, got %q", line)
	}
}

func TestStateful_Expire_TTL_Persist(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "v"))

	line := respRoundTrip(t, port, respCmd("TTL", "k"))
	if line != ":-1" {
		t.Fatalf("TTL with no expiry: want ':-1', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("EXPIRE", "k", "100"))
	if line != ":1" {
		t.Fatalf("EXPIRE: want ':1', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("TTL", "k"))
	if line == ":-1" || line == ":-2" {
		t.Fatalf("TTL after EXPIRE: want a positive remaining TTL, got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("PERSIST", "k"))
	if line != ":1" {
		t.Fatalf("PERSIST: want ':1', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("TTL", "k"))
	if line != ":-1" {
		t.Fatalf("TTL after PERSIST: want ':-1', got %q", line)
	}
}

func TestStateful_SetWithEX_ExpiresAfterTTL(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "v", "EX", "1"))
	time.Sleep(1100 * time.Millisecond)

	line := respRoundTrip(t, port, respCmd("GET", "k"))
	if line != "$-1" {
		t.Fatalf("expected key to have expired via SET EX, got %q", line)
	}
}

func TestStateful_TTL_AbsentKey(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	line := respRoundTrip(t, port, respCmd("TTL", "nope"))
	if line != ":-2" {
		t.Fatalf("TTL for absent key: want ':-2', got %q", line)
	}
}

func TestStateful_Append(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "Hello"))
	line := respRoundTrip(t, port, respCmd("APPEND", "k", " World"))
	if line != ":11" {
		t.Fatalf("APPEND: want ':11', got %q", line)
	}
	lines := respRoundTripLines(t, port, respCmd("GET", "k"), 2)
	if len(lines) != 2 || lines[1] != "Hello World" {
		t.Fatalf("GET after APPEND: want 'Hello World', got %v", lines)
	}
}

func TestStateful_Hash_SetGetDelExists(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	line := respRoundTrip(t, port, respCmd("HSET", "h", "f1", "v1", "f2", "v2"))
	if line != ":2" {
		t.Fatalf("HSET new fields: want ':2', got %q", line)
	}

	// Updating an existing field shouldn't count as "new".
	line = respRoundTrip(t, port, respCmd("HSET", "h", "f1", "updated"))
	if line != ":0" {
		t.Fatalf("HSET update existing field: want ':0', got %q", line)
	}

	lines := respRoundTripLines(t, port, respCmd("HGET", "h", "f1"), 2)
	if len(lines) != 2 || lines[1] != "updated" {
		t.Fatalf("HGET: want 'updated', got %v", lines)
	}

	line = respRoundTrip(t, port, respCmd("HEXISTS", "h", "f2"))
	if line != ":1" {
		t.Fatalf("HEXISTS present: want ':1', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("HDEL", "h", "f2"))
	if line != ":1" {
		t.Fatalf("HDEL: want ':1', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("HEXISTS", "h", "f2"))
	if line != ":0" {
		t.Fatalf("HEXISTS after delete: want ':0', got %q", line)
	}
}

func TestStateful_List_PushRangeLen(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	line := respRoundTrip(t, port, respCmd("RPUSH", "l", "a", "b", "c"))
	if line != ":3" {
		t.Fatalf("RPUSH: want ':3', got %q", line)
	}

	line = respRoundTrip(t, port, respCmd("LLEN", "l"))
	if line != ":3" {
		t.Fatalf("LLEN: want ':3', got %q", line)
	}

	lines := respRoundTripLines(t, port, respCmd("LRANGE", "l", "0", "-1"), 7)
	// *3\r\n $1 a $1 b $1 c  => 7 lines total.
	want := []string{"*3", "$1", "a", "$1", "b", "$1", "c"}
	if len(lines) != len(want) {
		t.Fatalf("LRANGE: want %v, got %v", want, lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("LRANGE line %d: want %q, got %q", i, want[i], lines[i])
		}
	}
}

func TestStateful_HGetAll(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("HSET", "h", "f1", "v1", "f2", "v2"))

	// *4\r\n + 4 bulk strings (2 lines each) = 9 lines total.
	lines := respRoundTripLines(t, port, respCmd("HGETALL", "h"), 9)
	if len(lines) != 9 || lines[0] != "*4" {
		t.Fatalf("HGETALL array header: want '*4', got %v", lines)
	}
	// Pair up field/value by scanning the flat bulk-string sequence directly,
	// since map iteration order of the hash is non-deterministic.
	fields := map[string]string{}
	for i := 1; i+3 < len(lines); i += 4 {
		fields[lines[i+1]] = lines[i+3]
	}
	if fields["f1"] != "v1" || fields["f2"] != "v2" {
		t.Fatalf("HGETALL fields: want f1=v1 f2=v2, got %v (lines=%v)", fields, lines)
	}
}

func TestStateful_HGetAll_MissingKeyReturnsEmptyArray(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	line := respRoundTrip(t, port, respCmd("HGETALL", "nosuchkey"))
	if line != "*0" {
		t.Fatalf("HGETALL missing key: want '*0', got %q", line)
	}
}

func TestStateful_HGetAll_WrongType(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "v"))
	line := respRoundTrip(t, port, respCmd("HGETALL", "k"))
	if line[0] != '-' {
		t.Fatalf("expected WRONGTYPE error for HGETALL on a string key, got %q", line)
	}
}

// TestStateful_WrongArity proves every stateful command's arity check
// (wrongArgsErr) actually runs and returns a RESP error, not just an
// internal panic or silently-wrong response, when called with too few
// arguments.
func TestStateful_WrongArity(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	cases := []string{"SET", "GET", "DEL", "EXISTS", "EXPIRE", "TTL", "PERSIST", "APPEND", "HSET", "HGET", "HGETALL", "HDEL", "HEXISTS", "LPUSH", "RPUSH", "LRANGE", "LLEN"}
	for _, cmd := range cases {
		t.Run(cmd, func(t *testing.T) {
			line := respRoundTrip(t, port, respCmd(cmd))
			if len(line) == 0 || line[0] != '-' {
				t.Fatalf("%s with no args: want RESP error, got %q", cmd, line)
			}
		})
	}
}

// TestStateful_ResetData proves Server.ResetData actually clears the
// in-memory datastore (the codepath POST /api/reset relies on via
// internal/api.Server.reset to stop stateful Redis keys leaking between
// test runs/scenarios).
func TestStateful_ResetData(t *testing.T) {
	srv, port := newStatefulRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "v"))
	respRoundTrip(t, port, respCmd("HSET", "h", "f", "v"))
	respRoundTrip(t, port, respCmd("LPUSH", "l", "a"))

	srv.ResetData()

	if line := respRoundTrip(t, port, respCmd("GET", "k")); line != "$-1" {
		t.Fatalf("GET after ResetData: want '$-1', got %q", line)
	}
	if line := respRoundTrip(t, port, respCmd("HGETALL", "h")); line != "*0" {
		t.Fatalf("HGETALL after ResetData: want '*0', got %q", line)
	}
	if line := respRoundTrip(t, port, respCmd("LLEN", "l")); line != ":0" {
		t.Fatalf("LLEN after ResetData: want ':0', got %q", line)
	}
}

func TestStateful_WrongType(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "v"))
	line := respRoundTrip(t, port, respCmd("HGET", "k", "f"))
	if line[0] != '-' {
		t.Fatalf("expected WRONGTYPE error, got %q", line)
	}
}

func TestStateful_FlushdbClearsDataStore(t *testing.T) {
	port, stop := startStateful(t)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "k", "v"))
	respRoundTrip(t, port, respCmd("FLUSHDB"))

	line := respRoundTrip(t, port, respCmd("GET", "k"))
	if line != "$-1" {
		t.Fatalf("expected key gone after FLUSHDB in stateful mode, got %q", line)
	}
}

func TestStateful_StaticMockStillUsedForUncoveredCommands(t *testing.T) {
	srv, port := newStatefulRedisServer([]config.RedisMock{{
		ID:       "custom-ping-like",
		Command:  "TYPE",
		Key:      "k",
		Response: config.RedisResponse{Type: "string", Value: "string"},
	}})
	stop := startRedis(t, srv)
	defer stop()

	lines := respRoundTripLines(t, port, respCmd("TYPE", "k"), 2)
	if len(lines) != 2 || lines[1] != "string" {
		t.Fatalf("expected static mock to still handle uncovered TYPE command, got %v", lines)
	}
}

func TestStateful_NonStatefulModeUnaffected(t *testing.T) {
	// Default (non-stateful) servers must not round-trip SET/GET against a
	// real datastore — only static mocks apply, exactly as before.
	srv, port := newRedisServer(nil)
	stop := startRedis(t, srv)
	defer stop()

	respRoundTrip(t, port, respCmd("SET", "foo", "bar"))
	line := respRoundTrip(t, port, respCmd("GET", "foo"))
	if line[0] != '-' {
		t.Fatalf("expected 'no mock matched' error for GET in non-stateful mode, got %q", line)
	}
}
