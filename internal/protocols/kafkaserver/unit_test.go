// White-box unit tests for kafkaserver internals.
package kafkaserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// MessageStore
// ---------------------------------------------------------------------------

func TestKafka_NewMessageStore_Exported(t *testing.T) {
	ms := NewMessageStore(20)
	if ms == nil {
		t.Fatal("NewMessageStore returned nil")
	}
	if ms.maxSize != 20 {
		t.Errorf("want maxSize=20, got %d", ms.maxSize)
	}
}

func TestKafka_MessageStore_AddAndAll(t *testing.T) {
	ms := newMessageStore(100)
	ms.Add(config.ProducedKafkaMessage{ID: "1", Topic: "orders", Value: "hello"})
	ms.Add(config.ProducedKafkaMessage{ID: "2", Topic: "payments", Value: "world"})

	all := ms.All()
	if len(all) != 2 {
		t.Fatalf("want 2 messages, got %d", len(all))
	}
	if all[0].Topic != "orders" || all[1].Topic != "payments" {
		t.Errorf("unexpected messages: %+v", all)
	}
}

func TestKafka_MessageStore_Overflow(t *testing.T) {
	ms := newMessageStore(3)
	for i := 0; i < 5; i++ {
		ms.Add(config.ProducedKafkaMessage{ID: string(rune('0' + i)), Topic: "t"})
	}
	all := ms.All()
	if len(all) != 3 {
		t.Fatalf("want 3 (capacity), got %d", len(all))
	}
	// Oldest (0,1) should have been evicted.
	if all[0].ID != "2" {
		t.Errorf("expected oldest evicted; first ID should be '2', got %q", all[0].ID)
	}
}

func TestKafka_MessageStore_Clear(t *testing.T) {
	ms := newMessageStore(10)
	ms.Add(config.ProducedKafkaMessage{ID: "1"})
	ms.Clear()
	if len(ms.All()) != 0 {
		t.Fatal("expected empty after Clear")
	}
}

func TestKafka_MessageStore_DefaultCapacity(t *testing.T) {
	ms := newMessageStore(0)
	if ms == nil || ms.maxSize != 1000 {
		t.Fatalf("expected default capacity 1000, got %v", ms)
	}
}

func TestKafka_MessageStore_AllReturnsIsolatedSlice(t *testing.T) {
	ms := newMessageStore(10)
	ms.Add(config.ProducedKafkaMessage{ID: "1", Value: "original"})
	got := ms.All()
	got[0].Value = "mutated"
	if ms.All()[0].Value == "mutated" {
		t.Error("All() should return a copy; mutating the result should not affect internal state")
	}
}

// ---------------------------------------------------------------------------
// New / SetMocks / GetMocks / GetMessageStore
// ---------------------------------------------------------------------------

func newTestKafkaServer(t *testing.T, mocks []config.KafkaMock) *Server {
	t.Helper()
	cfg := &config.KafkaConfig{Enabled: true, Port: 0, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

func TestKafka_New_InitialMocks(t *testing.T) {
	mocks := []config.KafkaMock{{ID: "m1", Topic: "orders"}}
	srv := newTestKafkaServer(t, mocks)
	got := srv.GetMocks()
	if len(got) != 1 || got[0].ID != "m1" {
		t.Fatalf("unexpected mocks from New: %+v", got)
	}
}

func TestKafka_SetMocks_ReplacesList(t *testing.T) {
	srv := newTestKafkaServer(t, nil)
	srv.SetMocks([]config.KafkaMock{{ID: "a"}, {ID: "b"}})
	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
}

func TestKafka_SetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestKafkaServer(t, nil)
	original := []config.KafkaMock{{ID: "orig", Topic: "x"}}
	srv.SetMocks(original)
	original[0].Topic = "mutated"
	if srv.GetMocks()[0].Topic != "x" {
		t.Error("SetMocks should copy the slice")
	}
}

func TestKafka_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestKafkaServer(t, []config.KafkaMock{{ID: "m1", Topic: "t"}})
	got := srv.GetMocks()
	got[0].Topic = "mutated"
	if srv.GetMocks()[0].Topic != "t" {
		t.Error("GetMocks should return a copy")
	}
}

func TestKafka_GetMessageStore_Shared(t *testing.T) {
	srv := newTestKafkaServer(t, nil)
	ms := srv.GetMessageStore()
	if ms == nil {
		t.Fatal("GetMessageStore returned nil")
	}
	ms.Add(config.ProducedKafkaMessage{ID: "x"})
	if len(srv.GetMessageStore().All()) != 1 {
		t.Fatal("message store should be shared")
	}
}

// ---------------------------------------------------------------------------
// matchKafkaTopic (extra cases)
// ---------------------------------------------------------------------------

func TestMatchKafkaTopic_Empty_MatchesAll(t *testing.T) {
	if !matchKafkaTopic("", "anything") {
		t.Error("empty pattern should match everything")
	}
}

func TestMatchKafkaTopic_Wildcard_MatchesAll(t *testing.T) {
	if !matchKafkaTopic("*", "anything") {
		t.Error("* pattern should match everything")
	}
}

func TestMatchKafkaTopic_Exact(t *testing.T) {
	if !matchKafkaTopic("orders", "orders") {
		t.Error("exact should match")
	}
	if matchKafkaTopic("orders", "payments") {
		t.Error("exact should not match different topic")
	}
}

func TestMatchKafkaTopic_PrefixWildcard(t *testing.T) {
	if !matchKafkaTopic("orders-*", "orders-created") {
		t.Error("prefix wildcard should match")
	}
	if matchKafkaTopic("orders-*", "payments-created") {
		t.Error("prefix wildcard should not match wrong prefix")
	}
}

func TestMatchKafkaTopic_SuffixWildcard(t *testing.T) {
	if !matchKafkaTopic("*-created", "orders-created") {
		t.Error("suffix wildcard should match")
	}
	if matchKafkaTopic("*-created", "orders-deleted") {
		t.Error("suffix wildcard should not match wrong suffix")
	}
}

func TestMatchKafkaTopic_Regex(t *testing.T) {
	if !matchKafkaTopic(`re:^orders-[a-z]+$`, "orders-created") {
		t.Error("regex should match")
	}
	if matchKafkaTopic(`re:^orders-[a-z]+$`, "orders-123") {
		t.Error("regex should not match digits")
	}
}

func TestMatchKafkaTopic_InvalidRegex(t *testing.T) {
	if matchKafkaTopic(`re:[invalid`, "anything") {
		t.Error("invalid regex should not match")
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestKafka_matchMock_ExactTopic(t *testing.T) {
	srv := newTestKafkaServer(t, []config.KafkaMock{{ID: "m1", Topic: "orders"}})
	m, ok := srv.matchMock("orders")
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match, got ok=%v id=%q", ok, m.ID)
	}
}

func TestKafka_matchMock_NoMatch(t *testing.T) {
	srv := newTestKafkaServer(t, []config.KafkaMock{{ID: "m1", Topic: "orders"}})
	_, ok := srv.matchMock("payments")
	if ok {
		t.Fatal("expected no match for different topic")
	}
}

func TestKafka_matchMock_StateCondition_NotMet(t *testing.T) {
	st := state.New()
	cfg := &config.KafkaConfig{
		Mocks: []config.KafkaMock{{
			ID:    "m1",
			Topic: "orders",
			State: &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, scenarios.New(nil), logger.New(10))
	_, ok := srv.matchMock("orders")
	if ok {
		t.Fatal("expected no match when state condition not met")
	}
}

func TestKafka_matchMock_StateCondition_Met(t *testing.T) {
	st := state.New()
	st.Set("mode", "on")
	cfg := &config.KafkaConfig{
		Mocks: []config.KafkaMock{{
			ID:    "m1",
			Topic: "orders",
			State: &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, scenarios.New(nil), logger.New(10))
	m, ok := srv.matchMock("orders")
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match when state condition met, got ok=%v", ok)
	}
}

// ---------------------------------------------------------------------------
// nullableBytes
// ---------------------------------------------------------------------------

func TestNullableBytes_Empty(t *testing.T) {
	if nullableBytes("") != nil {
		t.Error("empty string should return nil")
	}
}

func TestNullableBytes_NonEmpty(t *testing.T) {
	b := nullableBytes("hello")
	if string(b) != "hello" {
		t.Errorf("want 'hello', got %q", string(b))
	}
}

// ---------------------------------------------------------------------------
// parseMessageSet
// ---------------------------------------------------------------------------

func TestParseMessageSet_ShortInput(t *testing.T) {
	k, v := parseMessageSet([]byte("short"))
	if k != "" {
		t.Errorf("short input key should be empty, got %q", k)
	}
	if v != "short" {
		t.Errorf("short input value should be raw bytes, got %q", v)
	}
}

// ---------------------------------------------------------------------------
// StatusInfo
// ---------------------------------------------------------------------------

func TestKafka_StatusInfo(t *testing.T) {
	srv := newTestKafkaServer(t, []config.KafkaMock{{ID: "m1"}, {ID: "m2"}})
	srv.GetMessageStore().Add(config.ProducedKafkaMessage{ID: "msg1"})
	info := srv.StatusInfo()
	if info["protocol"] != "kafka" {
		t.Errorf("unexpected protocol %v", info["protocol"])
	}
	if info["mocks"] != 2 {
		t.Errorf("want mocks=2, got %v", info["mocks"])
	}
	if info["messages"] != 1 {
		t.Errorf("want messages=1, got %v", info["messages"])
	}
}
