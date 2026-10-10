// White-box unit tests for amqpserver helpers.
package amqpserver

import (
	"bytes"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func newTestAMQPServer(mocks []config.AMQPMock) *Server {
	cfg := &config.AMQPConfig{Enabled: true, Port: 5672, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

// ---------------------------------------------------------------------------
// NewMessageStore / MessageStore
// ---------------------------------------------------------------------------

func TestAMQP_NewMessageStore_DefaultCapacity(t *testing.T) {
	ms := NewMessageStore(0)
	if ms == nil {
		t.Fatal("NewMessageStore returned nil")
	}
	if ms.maxSize <= 0 {
		t.Errorf("expected positive maxSize, got %d", ms.maxSize)
	}
}

func TestAMQP_NewMessageStore_CustomCapacity(t *testing.T) {
	ms := NewMessageStore(25)
	if ms.maxSize != 25 {
		t.Errorf("want maxSize=25, got %d", ms.maxSize)
	}
}

func TestAMQP_MessageStore_AddAndAll(t *testing.T) {
	ms := NewMessageStore(10)
	ms.Add(config.ReceivedAMQPMessage{ID: "m1", Exchange: "orders", RoutingKey: "created", Body: "test"})
	ms.Add(config.ReceivedAMQPMessage{ID: "m2", Exchange: "events", RoutingKey: "updated", Body: "body2"})
	all := ms.All()
	if len(all) != 2 {
		t.Fatalf("want 2 messages, got %d", len(all))
	}
	if all[0].ID != "m1" || all[1].ID != "m2" {
		t.Errorf("unexpected message IDs: %+v", all)
	}
}

func TestAMQP_MessageStore_Overflow(t *testing.T) {
	ms := NewMessageStore(2)
	ms.Add(config.ReceivedAMQPMessage{ID: "m1"})
	ms.Add(config.ReceivedAMQPMessage{ID: "m2"})
	ms.Add(config.ReceivedAMQPMessage{ID: "m3"})
	all := ms.All()
	if len(all) != 2 {
		t.Fatalf("want 2 messages after overflow, got %d", len(all))
	}
	if all[0].ID != "m2" || all[1].ID != "m3" {
		t.Errorf("oldest message should be dropped: %+v", all)
	}
}

func TestAMQP_MessageStore_Clear(t *testing.T) {
	ms := NewMessageStore(10)
	ms.Add(config.ReceivedAMQPMessage{ID: "m1"})
	ms.Clear()
	if len(ms.All()) != 0 {
		t.Error("Clear should remove all messages")
	}
}

func TestAMQP_MessageStore_AllIsolatesSlice(t *testing.T) {
	ms := NewMessageStore(10)
	ms.Add(config.ReceivedAMQPMessage{ID: "m1"})
	all := ms.All()
	all[0].ID = "mutated"
	if ms.All()[0].ID != "m1" {
		t.Error("All should return a copy")
	}
}

// ---------------------------------------------------------------------------
// SetMocks / GetMocks / GetMessageStore
// ---------------------------------------------------------------------------

func TestAMQP_SetGetMocks(t *testing.T) {
	srv := newTestAMQPServer(nil)
	mocks := []config.AMQPMock{
		{ID: "m1", Exchange: "orders"},
		{ID: "m2", Exchange: "events"},
	}
	srv.SetMocks(mocks)
	got := srv.GetMocks()
	if len(got) != 2 || got[0].ID != "m1" || got[1].ID != "m2" {
		t.Errorf("unexpected mocks: %+v", got)
	}
}

func TestAMQP_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestAMQPServer([]config.AMQPMock{{ID: "m1"}})
	got := srv.GetMocks()
	got[0].ID = "mutated"
	if srv.GetMocks()[0].ID != "m1" {
		t.Error("GetMocks should return a copy")
	}
}

func TestAMQP_GetMessageStore(t *testing.T) {
	srv := newTestAMQPServer(nil)
	ms := srv.GetMessageStore()
	if ms == nil {
		t.Fatal("GetMessageStore should return non-nil store")
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestAMQP_MatchMock_NoMocks(t *testing.T) {
	srv := newTestAMQPServer(nil)
	if _, ok := srv.matchMock("orders", "created"); ok {
		t.Error("should not match when there are no mocks")
	}
}

func TestAMQP_MatchMock_ExchangeMismatch(t *testing.T) {
	srv := newTestAMQPServer([]config.AMQPMock{
		{ID: "m1", Exchange: "orders", RoutingKey: "created"},
	})
	if _, ok := srv.matchMock("events", "created"); ok {
		t.Error("should not match different exchange")
	}
}

func TestAMQP_MatchMock_RoutingKeyMismatch(t *testing.T) {
	srv := newTestAMQPServer([]config.AMQPMock{
		{ID: "m1", Exchange: "orders", RoutingKey: "created"},
	})
	if _, ok := srv.matchMock("orders", "deleted"); ok {
		t.Error("should not match different routing key")
	}
}

func TestAMQP_MatchMock_Exact(t *testing.T) {
	srv := newTestAMQPServer([]config.AMQPMock{
		{ID: "m1", Exchange: "orders", RoutingKey: "created"},
	})
	m, ok := srv.matchMock("orders", "created")
	if !ok {
		t.Fatal("should match")
	}
	if m.ID != "m1" {
		t.Errorf("unexpected mock ID: %s", m.ID)
	}
}

func TestAMQP_MatchMock_StateCondition(t *testing.T) {
	st := state.New()
	cfg := &config.AMQPConfig{Enabled: true, Port: 5672, Mocks: []config.AMQPMock{
		{ID: "m1", Exchange: "orders", State: &config.StateCondition{Key: "active", Value: "yes"}},
	}}
	srv := New(cfg, st, scenarios.New(nil), logger.New(100))

	if _, ok := srv.matchMock("orders", "created"); ok {
		t.Error("should not match when state condition is unmet")
	}
	st.Set("active", "yes")
	if _, ok := srv.matchMock("orders", "created"); !ok {
		t.Error("should match when state condition is met")
	}
}

// ---------------------------------------------------------------------------
// matchAMQPRoutingKey extras
// ---------------------------------------------------------------------------

func TestMatchAMQPRoutingKey_Empty(t *testing.T) {
	if !matchAMQPRoutingKey("", "anything") {
		t.Error("empty pattern should match any key")
	}
}

func TestMatchAMQPRoutingKey_Exact(t *testing.T) {
	if !matchAMQPRoutingKey("order.created", "order.created") {
		t.Error("exact pattern should match identical key")
	}
	if matchAMQPRoutingKey("order.created", "order.deleted") {
		t.Error("exact pattern should not match different key")
	}
}

func TestMatchAMQPRoutingKey_InvalidRegex(t *testing.T) {
	if matchAMQPRoutingKey("re:[bad", "anything") {
		t.Error("invalid regex should not match")
	}
}

// ---------------------------------------------------------------------------
// encodeShortStr / readShortStr
// ---------------------------------------------------------------------------

func TestEncodeShortStr_Normal(t *testing.T) {
	encoded := encodeShortStr("hello")
	if len(encoded) != 6 || encoded[0] != 5 {
		t.Errorf("encodeShortStr(hello) = %v, want [5, 'h', 'e', 'l', 'l', 'o']", encoded)
	}
}

func TestEncodeShortStr_TruncatesLongString(t *testing.T) {
	long := string(make([]byte, 300))
	encoded := encodeShortStr(long)
	if len(encoded) != 256 {
		t.Errorf("long string should be truncated to 255 chars, got len=%d", len(encoded))
	}
	if encoded[0] != 255 {
		t.Errorf("length byte should be 255, got %d", encoded[0])
	}
}

func TestReadShortStr_Empty(t *testing.T) {
	buf := &bytes.Buffer{}
	got := readShortStr(buf)
	if got != "" {
		t.Errorf("readShortStr on empty buffer should return empty, got %q", got)
	}
}

func TestReadShortStr_Normal(t *testing.T) {
	buf := &bytes.Buffer{}
	buf.Write([]byte{5, 'h', 'e', 'l', 'l', 'o'})
	got := readShortStr(buf)
	if got != "hello" {
		t.Errorf("readShortStr = %q, want 'hello'", got)
	}
}

// ---------------------------------------------------------------------------
// amqpTopicMatches / bindingMatches
// ---------------------------------------------------------------------------

func TestAmqpTopicMatches(t *testing.T) {
	cases := []struct {
		pattern, key string
		want         bool
	}{
		{"orders.created", "orders.created", true},
		{"orders.created", "orders.deleted", false},
		{"orders.*", "orders.created", true},
		{"orders.*", "orders.created.extra", false},
		{"orders.*.created", "orders.us.created", true},
		{"orders.#", "orders", true}, // "#" matches zero or more trailing words
		{"orders.#", "orders.us.created", true},
		{"#", "anything.at.all", true},
		{"#", "", true},
		{"orders.#.created", "orders.us.west.created", true},
		{"orders.#.created", "orders.created", true},
	}
	for _, c := range cases {
		if got := amqpTopicMatches(c.pattern, c.key); got != c.want {
			t.Errorf("amqpTopicMatches(%q, %q) = %v, want %v", c.pattern, c.key, got, c.want)
		}
	}
}

func TestBindingMatches_Fanout(t *testing.T) {
	if !bindingMatches("fanout", "anything", "whatever") {
		t.Error("fanout should match regardless of pattern/key")
	}
	if !bindingMatches("fanout", "", "whatever") {
		t.Error("fanout with empty pattern should still match")
	}
}

func TestBindingMatches_Direct(t *testing.T) {
	if !bindingMatches("direct", "orders.created", "orders.created") {
		t.Error("direct should match exact routing key")
	}
	if bindingMatches("direct", "orders.created", "orders.deleted") {
		t.Error("direct should not match a different routing key")
	}
	if !bindingMatches("direct", "", "whatever") {
		t.Error("direct with empty pattern should match any key")
	}
}

func TestBindingMatches_Topic(t *testing.T) {
	if !bindingMatches("topic", "orders.*", "orders.created") {
		t.Error("topic should apply word-wildcard matching")
	}
	if bindingMatches("topic", "orders.*", "orders.created.extra") {
		t.Error("topic '*' should match exactly one word")
	}
}

// ---------------------------------------------------------------------------
// routeToQueues / exchangeType
// ---------------------------------------------------------------------------

func newTopologyTestServer(exchanges []config.AMQPExchange, bindings []config.AMQPBinding, mocks []config.AMQPMock) *Server {
	cfg := &config.AMQPConfig{Enabled: true, Port: 5672, Exchanges: exchanges, Bindings: bindings, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

func TestRouteToQueues_Direct(t *testing.T) {
	srv := newTopologyTestServer(nil, []config.AMQPBinding{
		{Exchange: "orders", RoutingKeyPattern: "orders.created", Queue: "q1"},
		{Exchange: "orders", RoutingKeyPattern: "orders.deleted", Queue: "q2"},
	}, nil)
	queues := srv.routeToQueues("orders", "orders.created")
	if len(queues) != 1 || queues[0] != "q1" {
		t.Errorf("routeToQueues = %v, want [q1]", queues)
	}
}

func TestRouteToQueues_Fanout(t *testing.T) {
	srv := newTopologyTestServer(
		[]config.AMQPExchange{{Name: "broadcast", Type: "fanout"}},
		[]config.AMQPBinding{
			{Exchange: "broadcast", Queue: "q1"},
			{Exchange: "broadcast", Queue: "q2"},
		}, nil)
	queues := srv.routeToQueues("broadcast", "anything")
	if len(queues) != 2 {
		t.Fatalf("routeToQueues = %v, want 2 queues", queues)
	}
}

func TestRouteToQueues_Topic(t *testing.T) {
	srv := newTopologyTestServer(
		[]config.AMQPExchange{{Name: "orders", Type: "topic"}},
		[]config.AMQPBinding{
			{Exchange: "orders", RoutingKeyPattern: "orders.*.created", Queue: "created-queue"},
			{Exchange: "orders", RoutingKeyPattern: "orders.#", Queue: "audit-queue"},
		}, nil)
	queues := srv.routeToQueues("orders", "orders.us.created")
	if len(queues) != 2 {
		t.Fatalf("routeToQueues = %v, want both queues matched", queues)
	}
}

func TestRouteToQueues_NoMatch(t *testing.T) {
	srv := newTopologyTestServer(nil, []config.AMQPBinding{
		{Exchange: "orders", RoutingKeyPattern: "orders.created", Queue: "q1"},
	}, nil)
	if queues := srv.routeToQueues("orders", "orders.deleted"); len(queues) != 0 {
		t.Errorf("routeToQueues = %v, want no matches", queues)
	}
	if queues := srv.routeToQueues("events", "orders.created"); len(queues) != 0 {
		t.Errorf("routeToQueues = %v, want no matches for different exchange", queues)
	}
}

func TestRouteToQueues_DedupesQueue(t *testing.T) {
	srv := newTopologyTestServer(nil, []config.AMQPBinding{
		{Exchange: "orders", RoutingKeyPattern: "orders.*", Queue: "q1"},
		{Exchange: "orders", RoutingKeyPattern: "", Queue: "q1"},
	}, nil)
	queues := srv.routeToQueues("orders", "orders.created")
	if len(queues) != 1 {
		t.Errorf("routeToQueues = %v, want deduped single q1", queues)
	}
}

func TestExchangeType_Default(t *testing.T) {
	srv := newTopologyTestServer(nil, nil, nil)
	if got := srv.exchangeType("undeclared"); got != "direct" {
		t.Errorf("exchangeType = %q, want direct for undeclared exchange", got)
	}
}

func TestExchangeType_Declared(t *testing.T) {
	srv := newTopologyTestServer([]config.AMQPExchange{{Name: "orders", Type: "topic"}}, nil, nil)
	if got := srv.exchangeType("orders"); got != "topic" {
		t.Errorf("exchangeType = %q, want topic", got)
	}
}

// ---------------------------------------------------------------------------
// matchMockByQueue
// ---------------------------------------------------------------------------

func TestMatchMockByQueue(t *testing.T) {
	srv := newTopologyTestServer(nil, nil, []config.AMQPMock{
		{ID: "m1", Queue: "q1", Response: &config.AMQPResponse{Body: "hello"}},
	})
	mock, ok := srv.matchMockByQueue("q1")
	if !ok || mock.ID != "m1" {
		t.Errorf("matchMockByQueue(q1) = %+v, %v", mock, ok)
	}
	if _, ok := srv.matchMockByQueue("q2"); ok {
		t.Error("matchMockByQueue(q2) should not match")
	}
}
