// Internal package test — accesses unexported fields/methods directly.
package redisserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

func makeServer(mocks []config.RedisMock) *Server {
	cfg := &config.RedisConfig{Enabled: true, Port: 6399, Mocks: mocks}
	return New(cfg, state.New(), logger.New(10))
}

func TestMatchMock_FirstMatchByCommandAndKey(t *testing.T) {
	mocks := []config.RedisMock{
		{ID: "m1", Command: "GET", Key: "user:*"},
		{ID: "m2", Command: "GET", Key: "session:*"},
	}
	srv := makeServer(mocks)

	m, ok := srv.matchMock("GET", "user:123")
	if !ok {
		t.Fatal("expected a match")
	}
	if m.ID != "m1" {
		t.Errorf("want m1, got %q", m.ID)
	}

	m2, ok2 := srv.matchMock("GET", "session:abc")
	if !ok2 {
		t.Fatal("expected a match for session key")
	}
	if m2.ID != "m2" {
		t.Errorf("want m2, got %q", m2.ID)
	}
}

func TestMatchMock_CommandMismatch(t *testing.T) {
	mocks := []config.RedisMock{
		{ID: "m1", Command: "SET", Key: "foo"},
	}
	srv := makeServer(mocks)

	_, ok := srv.matchMock("GET", "foo")
	if ok {
		t.Fatal("should not match when command is different")
	}
}

func TestMatchMock_WildcardCommand(t *testing.T) {
	mocks := []config.RedisMock{
		{ID: "catchall", Command: "*"},
	}
	srv := makeServer(mocks)

	m, ok := srv.matchMock("HGET", "anything")
	if !ok || m.ID != "catchall" {
		t.Fatalf("command=* should match any command, ok=%v id=%q", ok, m.ID)
	}
}

func TestMatchMock_NoMocks(t *testing.T) {
	srv := makeServer(nil)
	_, ok := srv.matchMock("GET", "key")
	if ok {
		t.Fatal("expected no match with empty mock list")
	}
}

func TestMatchMock_StateGuard_SkipsWhenNotSet(t *testing.T) {
	mocks := []config.RedisMock{
		{
			ID:      "m1",
			Command: "GET",
			Key:     "user",
			State:   &config.StateCondition{Key: "auth", Value: "ok"},
		},
	}
	srv := makeServer(mocks)

	// State not set → mock should be skipped.
	_, ok := srv.matchMock("GET", "user")
	if ok {
		t.Fatal("should not match when state condition is not met")
	}
}

func TestMatchMock_StateGuard_MatchesWhenSet(t *testing.T) {
	st := state.New()
	cfg := &config.RedisConfig{
		Enabled: true,
		Port:    6399,
		Mocks: []config.RedisMock{
			{
				ID:      "m1",
				Command: "GET",
				Key:     "user",
				State:   &config.StateCondition{Key: "auth", Value: "ok"},
			},
		},
	}
	srv := New(cfg, st, logger.New(10))

	st.Set("auth", "ok")
	m, ok := srv.matchMock("GET", "user")
	if !ok || m.ID != "m1" {
		t.Fatalf("should match after state condition is satisfied, ok=%v id=%q", ok, m.ID)
	}
}

func TestMatchMock_StateGuard_WrongValue(t *testing.T) {
	st := state.New()
	cfg := &config.RedisConfig{
		Enabled: true,
		Port:    6399,
		Mocks: []config.RedisMock{
			{
				ID:      "m1",
				Command: "GET",
				Key:     "user",
				State:   &config.StateCondition{Key: "auth", Value: "ok"},
			},
		},
	}
	srv := New(cfg, st, logger.New(10))

	st.Set("auth", "wrong")
	_, ok := srv.matchMock("GET", "user")
	if ok {
		t.Fatal("should not match when state value is wrong")
	}
}
