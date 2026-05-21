// Internal package test so we can access unexported helpers.
package mqttserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

func TestMatchMQTTTopic_Exact(t *testing.T) {
	if !matchMQTTTopic("sensors/temp", "sensors/temp") {
		t.Error("exact topic should match")
	}
	if matchMQTTTopic("sensors/temp", "sensors/humidity") {
		t.Error("exact topic should not match different topic")
	}
}

func TestMatchMQTTTopic_Hash_MatchAll(t *testing.T) {
	if !matchMQTTTopic("#", "anything/at/all") {
		t.Error("# should match any topic")
	}
	if !matchMQTTTopic("#", "single") {
		t.Error("# should match single level")
	}
}

func TestMatchMQTTTopic_SingleLevelWildcard(t *testing.T) {
	if !matchMQTTTopic("sensors/+", "sensors/temp") {
		t.Error("+ should match single level")
	}
	if !matchMQTTTopic("sensors/+", "sensors/humidity") {
		t.Error("+ should match any single level segment")
	}
	if matchMQTTTopic("sensors/+", "sensors/room/temp") {
		t.Error("+ should not match multiple levels")
	}
}

func TestMatchMQTTTopic_MultiLevelWildcard(t *testing.T) {
	if !matchMQTTTopic("sensors/#", "sensors/room/temp") {
		t.Error("# in filter should match multi-level topic")
	}
	if !matchMQTTTopic("sensors/#", "sensors/temp") {
		t.Error("# should match single remaining level")
	}
}

func TestMatchMQTTTopic_Mixed(t *testing.T) {
	if !matchMQTTTopic("home/+/temperature", "home/living/temperature") {
		t.Error("mixed filter should match")
	}
	if matchMQTTTopic("home/+/temperature", "home/living/humidity") {
		t.Error("mixed filter should not match wrong leaf")
	}
}

func TestMatchMQTTTopic_LevelCountMismatch(t *testing.T) {
	if matchMQTTTopic("a/b/c", "a/b") {
		t.Error("different depth without wildcard should not match")
	}
	if matchMQTTTopic("a/b", "a/b/c") {
		t.Error("filter shallower than topic should not match without wildcard")
	}
}

func TestMessageStore_AddAndAll(t *testing.T) {
	ms := newMessageStore(100)
	ms.Add(ReceivedMessage{ID: "1", Topic: "t/1", Payload: "hello"})
	ms.Add(ReceivedMessage{ID: "2", Topic: "t/2", Payload: "world"})

	all := ms.All()
	if len(all) != 2 {
		t.Fatalf("want 2 messages, got %d", len(all))
	}
	if all[0].Topic != "t/1" {
		t.Errorf("unexpected first message: %v", all[0])
	}
}

func TestMessageStore_Capacity(t *testing.T) {
	ms := newMessageStore(3)
	for i := 0; i < 5; i++ {
		ms.Add(ReceivedMessage{ID: string(rune('0' + i)), Topic: "t"})
	}
	all := ms.All()
	if len(all) != 3 {
		t.Fatalf("want 3 messages (capacity), got %d", len(all))
	}
	// Oldest should have been evicted.
	if all[0].ID != "2" {
		t.Errorf("expected oldest evicted; first remaining ID should be '2', got %q", all[0].ID)
	}
}

func TestMessageStore_Clear(t *testing.T) {
	ms := newMessageStore(10)
	ms.Add(ReceivedMessage{ID: "1"})
	ms.Clear()
	if len(ms.All()) != 0 {
		t.Fatal("expected empty store after Clear")
	}
}

// ---------------------------------------------------------------------------
// NewMessageStore (exported constructor)
// ---------------------------------------------------------------------------

func TestNewMessageStore_PositiveCapacity(t *testing.T) {
	ms := NewMessageStore(5)
	if ms == nil {
		t.Fatal("NewMessageStore returned nil")
	}
	for i := range 7 {
		ms.Add(ReceivedMessage{ID: string(rune('0' + i))})
	}
	if len(ms.All()) != 5 {
		t.Fatalf("expected capacity 5, got %d", len(ms.All()))
	}
}

func TestNewMessageStore_DefaultCapacity(t *testing.T) {
	ms := NewMessageStore(0)
	if ms == nil {
		t.Fatal("NewMessageStore(0) returned nil")
	}
}

// ---------------------------------------------------------------------------
// New / SetMocks / GetMocks / GetMessageStore
// ---------------------------------------------------------------------------

func newTestMQTTServer(t *testing.T, mocks []config.MQTTMock) *Server {
	t.Helper()
	cfg := &config.MQTTConfig{Enabled: true, Port: 0, Mocks: mocks}
	return New(cfg, state.New(), nil, logger.New(100))
}

func TestMQTT_New_InitialMocks(t *testing.T) {
	mocks := []config.MQTTMock{{ID: "m1", Topic: "sensors/temp"}}
	srv := newTestMQTTServer(t, mocks)
	got := srv.GetMocks()
	if len(got) != 1 || got[0].ID != "m1" {
		t.Fatalf("unexpected mocks from New: %+v", got)
	}
}

func TestMQTT_SetMocks_ReplacesList(t *testing.T) {
	srv := newTestMQTTServer(t, nil)
	srv.SetMocks([]config.MQTTMock{{ID: "a"}, {ID: "b"}})
	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
}

func TestMQTT_SetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestMQTTServer(t, nil)
	original := []config.MQTTMock{{ID: "orig", Topic: "x"}}
	srv.SetMocks(original)
	original[0].Topic = "mutated"
	if srv.GetMocks()[0].Topic != "x" {
		t.Error("SetMocks should copy the slice")
	}
}

func TestMQTT_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestMQTTServer(t, []config.MQTTMock{{ID: "m1", Topic: "t"}})
	got := srv.GetMocks()
	got[0].Topic = "mutated"
	if srv.GetMocks()[0].Topic != "t" {
		t.Error("GetMocks should return a copy")
	}
}

func TestMQTT_GetMessageStore(t *testing.T) {
	srv := newTestMQTTServer(t, nil)
	ms := srv.GetMessageStore()
	if ms == nil {
		t.Fatal("GetMessageStore returned nil")
	}
	ms.Add(ReceivedMessage{ID: "x"})
	if len(srv.GetMessageStore().All()) != 1 {
		t.Fatal("message store should be shared")
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestMQTT_matchMock_ExactTopic(t *testing.T) {
	srv := newTestMQTTServer(t, []config.MQTTMock{
		{ID: "m1", Topic: "sensors/temp"},
	})
	m, ok := srv.matchMock("sensors/temp")
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match, got ok=%v id=%q", ok, m.ID)
	}
}

func TestMQTT_matchMock_NoMatch(t *testing.T) {
	srv := newTestMQTTServer(t, []config.MQTTMock{
		{ID: "m1", Topic: "sensors/temp"},
	})
	_, ok := srv.matchMock("sensors/humidity")
	if ok {
		t.Fatal("expected no match for different topic")
	}
}

func TestMQTT_matchMock_WildcardTopic(t *testing.T) {
	srv := newTestMQTTServer(t, []config.MQTTMock{
		{ID: "all", Topic: "#"},
	})
	_, ok := srv.matchMock("anything/at/all")
	if !ok {
		t.Fatal("expected wildcard match")
	}
}

func TestMQTT_matchMock_StateCondition_NotMet(t *testing.T) {
	st := state.New()
	cfg := &config.MQTTConfig{
		Enabled: true,
		Port:    0,
		Mocks: []config.MQTTMock{{
			ID:    "m1",
			Topic: "t",
			State: &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, nil, logger.New(10))
	_, ok := srv.matchMock("t")
	if ok {
		t.Fatal("expected no match when state condition is not met")
	}
}

func TestMQTT_matchMock_StateCondition_Met(t *testing.T) {
	st := state.New()
	st.Set("mode", "on")
	cfg := &config.MQTTConfig{
		Enabled: true,
		Port:    0,
		Mocks: []config.MQTTMock{{
			ID:    "m1",
			Topic: "t",
			State: &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, nil, logger.New(10))
	m, ok := srv.matchMock("t")
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match when state condition met, got ok=%v", ok)
	}
}

// ---------------------------------------------------------------------------
// StatusInfo
// ---------------------------------------------------------------------------

func TestMQTT_StatusInfo(t *testing.T) {
	srv := newTestMQTTServer(t, []config.MQTTMock{{ID: "m1"}, {ID: "m2"}})
	srv.GetMessageStore().Add(ReceivedMessage{ID: "msg1"})
	info := srv.StatusInfo()
	if info["protocol"] != "mqtt" {
		t.Errorf("unexpected protocol %v", info["protocol"])
	}
	if info["mocks"] != 2 {
		t.Errorf("want mocks=2, got %v", info["mocks"])
	}
	if info["messages"] != 1 {
		t.Errorf("want messages=1, got %v", info["messages"])
	}
}

