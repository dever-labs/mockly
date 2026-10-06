// Internal package test so we can access unexported helpers.
package natsserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func TestMatchNATSSubject_Exact(t *testing.T) {
	if ok, _ := matchNATSSubject("sensors.temp", "sensors.temp"); !ok {
		t.Error("exact subject should match")
	}
	if ok, _ := matchNATSSubject("sensors.temp", "sensors.humidity"); ok {
		t.Error("exact subject should not match different subject")
	}
}

func TestMatchNATSSubject_GreaterThan_MatchAll(t *testing.T) {
	if ok, _ := matchNATSSubject(">", "anything.at.all"); !ok {
		t.Error("> should match any subject")
	}
	if ok, _ := matchNATSSubject(">", "single"); !ok {
		t.Error("> should match single token")
	}
}

func TestMatchNATSSubject_SingleTokenWildcard(t *testing.T) {
	if ok, _ := matchNATSSubject("sensors.*", "sensors.temp"); !ok {
		t.Error("* should match single token")
	}
	if ok, _ := matchNATSSubject("sensors.*", "sensors.room.temp"); ok {
		t.Error("* should not match multiple tokens")
	}
}

func TestMatchNATSSubject_MultiTokenWildcard(t *testing.T) {
	if ok, _ := matchNATSSubject("sensors.>", "sensors.room.temp"); !ok {
		t.Error("> in pattern should match multi-token subject")
	}
	if ok, _ := matchNATSSubject("sensors.>", "sensors.temp"); !ok {
		t.Error("> should match single remaining token")
	}
}

func TestMatchNATSSubject_Mixed(t *testing.T) {
	if ok, _ := matchNATSSubject("home.*.temperature", "home.living.temperature"); !ok {
		t.Error("mixed pattern should match")
	}
	if ok, _ := matchNATSSubject("home.*.temperature", "home.living.humidity"); ok {
		t.Error("mixed pattern should not match wrong leaf")
	}
}

func TestMatchNATSSubject_NamedCapture(t *testing.T) {
	ok, params := matchNATSSubject("orders.{id}.created", "orders.42.created")
	if !ok {
		t.Fatal("named capture pattern should match")
	}
	if params["id"] != "42" {
		t.Errorf("expected id=42, got %q", params["id"])
	}
}

func TestMatchNATSSubject_NamedCapture_NoMatch(t *testing.T) {
	ok, _ := matchNATSSubject("orders.{id}.created", "orders.42.cancelled")
	if ok {
		t.Error("should not match different leaf")
	}
}

func TestMatchNATSSubject_TokenCountMismatch(t *testing.T) {
	if ok, _ := matchNATSSubject("a.b", "a.b.c"); ok {
		t.Error("should not match when subject has more tokens than pattern")
	}
	if ok, _ := matchNATSSubject("a.b.c", "a.b"); ok {
		t.Error("should not match when subject has fewer tokens than pattern")
	}
}

func TestMessageStore_AddAndAll(t *testing.T) {
	ms := newMessageStore(10)
	ms.Add(ReceivedMessage{Subject: "a"})
	ms.Add(ReceivedMessage{Subject: "b"})
	all := ms.All()
	if len(all) != 2 || all[0].Subject != "a" || all[1].Subject != "b" {
		t.Errorf("unexpected messages: %+v", all)
	}
}

func TestMessageStore_Capacity(t *testing.T) {
	ms := newMessageStore(2)
	ms.Add(ReceivedMessage{Subject: "a"})
	ms.Add(ReceivedMessage{Subject: "b"})
	ms.Add(ReceivedMessage{Subject: "c"})
	all := ms.All()
	if len(all) != 2 || all[0].Subject != "b" || all[1].Subject != "c" {
		t.Errorf("expected ring buffer to drop oldest, got %+v", all)
	}
}

func TestMessageStore_Clear(t *testing.T) {
	ms := newMessageStore(10)
	ms.Add(ReceivedMessage{Subject: "a"})
	ms.Clear()
	if len(ms.All()) != 0 {
		t.Error("expected empty store after Clear")
	}
}

func TestNewMessageStore_PositiveCapacity(t *testing.T) {
	ms := NewMessageStore(5)
	if ms.maxSize != 5 {
		t.Errorf("expected capacity 5, got %d", ms.maxSize)
	}
}

func TestNewMessageStore_DefaultCapacity(t *testing.T) {
	ms := NewMessageStore(0)
	if ms.maxSize != config.DefaultMessageStoreSize {
		t.Errorf("expected default capacity, got %d", ms.maxSize)
	}
}

func newTestServer(mocks []config.NATSMock) *Server {
	cfg := &config.NATSConfig{Enabled: true, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(10))
}

func TestNATS_New_InitialMocks(t *testing.T) {
	s := newTestServer([]config.NATSMock{{ID: "m1", Subject: "a"}})
	if len(s.GetMocks()) != 1 {
		t.Fatal("expected initial mock from config")
	}
}

func TestNATS_SetMocks_ReplacesList(t *testing.T) {
	s := newTestServer(nil)
	s.SetMocks([]config.NATSMock{{ID: "m1", Subject: "a"}, {ID: "m2", Subject: "b"}})
	if len(s.GetMocks()) != 2 {
		t.Fatal("expected 2 mocks after SetMocks")
	}
}

func TestNATS_GetMocks_IsolatesSlice(t *testing.T) {
	s := newTestServer([]config.NATSMock{{ID: "m1", Subject: "a"}})
	got := s.GetMocks()
	got[0].ID = "mutated"
	if s.GetMocks()[0].ID == "mutated" {
		t.Error("GetMocks should return a copy, not the internal slice")
	}
}

func TestNATS_matchPlainMock_ExactSubject(t *testing.T) {
	s := newTestServer([]config.NATSMock{{ID: "m1", Subject: "orders.created"}})
	mock, ok, _ := s.matchPlainMock("orders.created")
	if !ok || mock.ID != "m1" {
		t.Fatal("expected exact subject to match")
	}
}

func TestNATS_matchPlainMock_NoMatch(t *testing.T) {
	s := newTestServer([]config.NATSMock{{ID: "m1", Subject: "orders.created"}})
	_, ok, _ := s.matchPlainMock("orders.cancelled")
	if ok {
		t.Error("expected no match")
	}
}

func TestNATS_matchPlainMock_SkipsQueueGrouped(t *testing.T) {
	s := newTestServer([]config.NATSMock{{ID: "m1", Subject: "work.task", QueueGroup: "workers"}})
	_, ok, _ := s.matchPlainMock("work.task")
	if ok {
		t.Error("queue-grouped mocks should not be matched by the catch-all plain matcher")
	}
}

func TestNATS_matchPlainMock_StateCondition(t *testing.T) {
	st := state.New()
	s := newTestServer(nil)
	s.store = st
	s.SetMocks([]config.NATSMock{{
		ID:      "m1",
		Subject: "orders.created",
		State:   &config.StateCondition{Key: "mode", Value: "demo"},
	}})

	if _, ok, _ := s.matchPlainMock("orders.created"); ok {
		t.Error("should not match before state condition is met")
	}
	st.Set("mode", "demo")
	if _, ok, _ := s.matchPlainMock("orders.created"); !ok {
		t.Error("should match once state condition is met")
	}
}

func TestNATS_StatusInfo(t *testing.T) {
	s := newTestServer([]config.NATSMock{{ID: "m1", Subject: "a"}})
	info := s.StatusInfo()
	if info["protocol"] != "nats" {
		t.Errorf("unexpected protocol: %v", info["protocol"])
	}
	if info["mocks"] != 1 {
		t.Errorf("expected 1 mock, got %v", info["mocks"])
	}
}
