// White-box unit tests for stompserver helpers.
package stompserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func newTestSTOMPServer(mocks []config.STOMPMock) *Server {
	cfg := &config.STOMPConfig{Enabled: true, Port: 61613, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

// ---------------------------------------------------------------------------
// NewMessageStore / MessageStore
// ---------------------------------------------------------------------------

func TestSTOMP_NewMessageStore_DefaultCapacity(t *testing.T) {
	ms := NewMessageStore(0)
	if ms == nil {
		t.Fatal("NewMessageStore returned nil")
	}
	if ms.maxSize <= 0 {
		t.Errorf("expected positive maxSize, got %d", ms.maxSize)
	}
}

func TestSTOMP_NewMessageStore_CustomCapacity(t *testing.T) {
	ms := NewMessageStore(30)
	if ms.maxSize != 30 {
		t.Errorf("want maxSize=30, got %d", ms.maxSize)
	}
}

func TestSTOMP_MessageStore_AddAndAll(t *testing.T) {
	ms := NewMessageStore(10)
	ms.Add(config.ReceivedSTOMPMessage{ID: "m1", Destination: "/queue/orders", Body: "hello"})
	ms.Add(config.ReceivedSTOMPMessage{ID: "m2", Destination: "/topic/events", Body: "world"})
	all := ms.All()
	if len(all) != 2 {
		t.Fatalf("want 2 messages, got %d", len(all))
	}
	if all[0].ID != "m1" || all[1].ID != "m2" {
		t.Errorf("unexpected messages: %+v", all)
	}
}

func TestSTOMP_MessageStore_Overflow(t *testing.T) {
	ms := NewMessageStore(2)
	ms.Add(config.ReceivedSTOMPMessage{ID: "m1"})
	ms.Add(config.ReceivedSTOMPMessage{ID: "m2"})
	ms.Add(config.ReceivedSTOMPMessage{ID: "m3"})
	all := ms.All()
	if len(all) != 2 {
		t.Fatalf("want 2 messages after overflow, got %d", len(all))
	}
	if all[0].ID != "m2" || all[1].ID != "m3" {
		t.Errorf("oldest message should be dropped: %+v", all)
	}
}

func TestSTOMP_MessageStore_Clear(t *testing.T) {
	ms := NewMessageStore(10)
	ms.Add(config.ReceivedSTOMPMessage{ID: "m1"})
	ms.Clear()
	if len(ms.All()) != 0 {
		t.Error("Clear should remove all messages")
	}
}

func TestSTOMP_MessageStore_AllIsolatesSlice(t *testing.T) {
	ms := NewMessageStore(10)
	ms.Add(config.ReceivedSTOMPMessage{ID: "m1"})
	all := ms.All()
	all[0].ID = "mutated"
	if ms.All()[0].ID != "m1" {
		t.Error("All should return a copy")
	}
}

// ---------------------------------------------------------------------------
// SetMocks / GetMocks / GetMessageStore
// ---------------------------------------------------------------------------

func TestSTOMP_SetGetMocks(t *testing.T) {
	srv := newTestSTOMPServer(nil)
	mocks := []config.STOMPMock{
		{ID: "m1", Destination: "/queue/orders"},
		{ID: "m2", Destination: "/topic/events"},
	}
	srv.SetMocks(mocks)
	got := srv.GetMocks()
	if len(got) != 2 || got[0].ID != "m1" || got[1].ID != "m2" {
		t.Errorf("unexpected mocks: %+v", got)
	}
}

func TestSTOMP_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestSTOMPServer([]config.STOMPMock{{ID: "m1"}})
	got := srv.GetMocks()
	got[0].ID = "mutated"
	if srv.GetMocks()[0].ID != "m1" {
		t.Error("GetMocks should return a copy")
	}
}

func TestSTOMP_GetMessageStore(t *testing.T) {
	srv := newTestSTOMPServer(nil)
	ms := srv.GetMessageStore()
	if ms == nil {
		t.Fatal("GetMessageStore should return non-nil store")
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestSTOMP_MatchMock_NoMocks(t *testing.T) {
	srv := newTestSTOMPServer(nil)
	if _, ok := srv.matchMock("/queue/orders"); ok {
		t.Error("should not match when there are no mocks")
	}
}

func TestSTOMP_MatchMock_DestinationMismatch(t *testing.T) {
	srv := newTestSTOMPServer([]config.STOMPMock{
		{ID: "m1", Destination: "/queue/orders"},
	})
	if _, ok := srv.matchMock("/queue/events"); ok {
		t.Error("should not match different destination")
	}
}

func TestSTOMP_MatchMock_Match(t *testing.T) {
	srv := newTestSTOMPServer([]config.STOMPMock{
		{ID: "m1", Destination: "/queue/orders"},
	})
	m, ok := srv.matchMock("/queue/orders")
	if !ok {
		t.Fatal("should match")
	}
	if m.ID != "m1" {
		t.Errorf("unexpected mock ID: %s", m.ID)
	}
}

func TestSTOMP_MatchMock_StateCondition(t *testing.T) {
	st := state.New()
	cfg := &config.STOMPConfig{Enabled: true, Port: 61613, Mocks: []config.STOMPMock{
		{ID: "m1", Destination: "/queue/orders", State: &config.StateCondition{Key: "mode", Value: "active"}},
	}}
	srv := New(cfg, st, scenarios.New(nil), logger.New(100))

	if _, ok := srv.matchMock("/queue/orders"); ok {
		t.Error("should not match when state condition is unmet")
	}
	st.Set("mode", "active")
	if _, ok := srv.matchMock("/queue/orders"); !ok {
		t.Error("should match when state condition is met")
	}
}

// ---------------------------------------------------------------------------
// matchDestination extras
// ---------------------------------------------------------------------------

func TestMatchDestination_Empty(t *testing.T) {
	if !matchDestination("", "/queue/anything") {
		t.Error("empty pattern should match any destination")
	}
}

func TestMatchDestination_Exact(t *testing.T) {
	if !matchDestination("/queue/orders", "/queue/orders") {
		t.Error("exact pattern should match identical destination")
	}
	if matchDestination("/queue/orders", "/queue/events") {
		t.Error("exact pattern should not match different destination")
	}
}

func TestMatchDestination_InvalidRegex(t *testing.T) {
	if matchDestination("re:[bad", "/queue/anything") {
		t.Error("invalid regex should not match")
	}
}

func TestMatchDestination_WildcardSuffix(t *testing.T) {
	if !matchDestination("/queue/*", "/queue/orders") {
		t.Error("wildcard should match suffix")
	}
	if matchDestination("/queue/*", "/topic/orders") {
		t.Error("wildcard prefix should not match wrong prefix")
	}
}
