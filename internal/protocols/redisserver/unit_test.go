// White-box unit tests for redisserver helpers.
package redisserver

import (
	"net"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/tidwall/redcon"
)

// ---------------------------------------------------------------------------
// mockConn satisfies redcon.Conn for testing writeRedisResponse.
// ---------------------------------------------------------------------------

type mockConn struct {
	nulls  int
	bulks  []string
	ints   []int
	int64s []int64
	errors []string
	arrays []int
}

func (m *mockConn) WriteNull()                  { m.nulls++ }
func (m *mockConn) WriteBulkString(s string)    { m.bulks = append(m.bulks, s) }
func (m *mockConn) WriteInt(n int)              { m.ints = append(m.ints, n) }
func (m *mockConn) WriteInt64(n int64)          { m.int64s = append(m.int64s, n) }
func (m *mockConn) WriteError(msg string)       { m.errors = append(m.errors, msg) }
func (m *mockConn) WriteArray(count int)        { m.arrays = append(m.arrays, count) }
func (m *mockConn) RemoteAddr() string          { return "127.0.0.1:0" }
func (m *mockConn) Close() error                { return nil }
func (m *mockConn) WriteString(s string)        {}
func (m *mockConn) WriteBulk(b []byte)          {}
func (m *mockConn) WriteUint64(n uint64)        {}
func (m *mockConn) WriteRaw(data []byte)        {}
func (m *mockConn) WriteAny(any interface{})    {}
func (m *mockConn) Context() interface{}        { return nil }
func (m *mockConn) SetContext(v interface{})    {}
func (m *mockConn) SetReadBuffer(bytes int)     {}
func (m *mockConn) Detach() redcon.DetachedConn      { return nil }
func (m *mockConn) NetConn() net.Conn                { return nil }
func (m *mockConn) PeekPipeline() []redcon.Command   { return nil }
func (m *mockConn) ReadPipeline() []redcon.Command   { return nil }

var _ redcon.Conn = (*mockConn)(nil)

// ---------------------------------------------------------------------------
// StatusInfo
// ---------------------------------------------------------------------------

func TestRedis_StatusInfo(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: true, Port: 6379, Mocks: []config.RedisMock{{ID: "m1"}, {ID: "m2"}}}
	srv := New(cfg, state.New(), scenarios.New(nil), logger.New(100))
	info := srv.StatusInfo()
	if info["protocol"] != "redis" {
		t.Errorf("unexpected protocol: %v", info["protocol"])
	}
	if info["port"] != 6379 {
		t.Errorf("unexpected port: %v", info["port"])
	}
	if info["mocks"] != 2 {
		t.Errorf("want mocks=2, got %v", info["mocks"])
	}
}

// ---------------------------------------------------------------------------
// writeRedisResponse
// ---------------------------------------------------------------------------

func TestWriteRedisResponse_NilBulk(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "bulk", Value: nil})
	if c.nulls != 1 {
		t.Errorf("nil bulk should call WriteNull, got nulls=%d", c.nulls)
	}
}

func TestWriteRedisResponse_BulkString(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "string", Value: "hello"})
	if len(c.bulks) != 1 || c.bulks[0] != "hello" {
		t.Errorf("bulk string: unexpected writes %v", c.bulks)
	}
}

func TestWriteRedisResponse_IntType_Int(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "integer", Value: 42})
	if len(c.ints) != 1 || c.ints[0] != 42 {
		t.Errorf("int type: unexpected ints %v", c.ints)
	}
}

func TestWriteRedisResponse_IntType_Int64(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "int", Value: int64(100)})
	if len(c.int64s) != 1 || c.int64s[0] != 100 {
		t.Errorf("int64 type: unexpected int64s %v", c.int64s)
	}
}

func TestWriteRedisResponse_IntType_Float64(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "integer", Value: float64(7)})
	if len(c.int64s) != 1 || c.int64s[0] != 7 {
		t.Errorf("float64 as int64: unexpected int64s %v", c.int64s)
	}
}

func TestWriteRedisResponse_IntType_DefaultZero(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "int", Value: "not-a-number"})
	if len(c.ints) != 1 || c.ints[0] != 0 {
		t.Errorf("unknown value type should write 0, got %v", c.ints)
	}
}

func TestWriteRedisResponse_Error(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "error", Value: "ERR bad command"})
	if len(c.errors) != 1 || c.errors[0] != "ERR bad command" {
		t.Errorf("error type: unexpected errors %v", c.errors)
	}
}

func TestWriteRedisResponse_ErrorNilValue(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "err", Value: nil})
	if len(c.errors) != 1 || c.errors[0] != "ERR" {
		t.Errorf("nil error should write 'ERR', got %v", c.errors)
	}
}

func TestWriteRedisResponse_NullType(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "null"})
	if c.nulls != 1 {
		t.Errorf("null type should call WriteNull, got nulls=%d", c.nulls)
	}
}

func TestWriteRedisResponse_NilType(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "nil"})
	if c.nulls != 1 {
		t.Errorf("nil type should call WriteNull, got nulls=%d", c.nulls)
	}
}

func TestWriteRedisResponse_Array(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "array", Value: []interface{}{"a", "b", "c"}})
	if len(c.arrays) != 1 || c.arrays[0] != 3 {
		t.Errorf("array: unexpected arrays %v", c.arrays)
	}
	if len(c.bulks) != 3 {
		t.Errorf("array items: expected 3 bulk strings, got %d", len(c.bulks))
	}
}

func TestWriteRedisResponse_DefaultNilValue(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "unknown", Value: nil})
	if c.nulls != 1 {
		t.Errorf("unknown type with nil value should WriteNull, got nulls=%d", c.nulls)
	}
}

func TestWriteRedisResponse_DefaultNonNil(t *testing.T) {
	c := &mockConn{}
	writeRedisResponse(c, config.RedisResponse{Type: "unknown", Value: "data"})
	if len(c.bulks) != 1 || c.bulks[0] != "data" {
		t.Errorf("unknown type with value should WriteBulkString, got %v", c.bulks)
	}
}
