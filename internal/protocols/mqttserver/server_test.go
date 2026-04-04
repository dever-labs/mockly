// Internal package test so we can access unexported helpers.
package mqttserver

import (
	"testing"
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
