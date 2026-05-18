package kafkaserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestMatchKafkaTopic(t *testing.T) {
	if !matchKafkaTopic("orders-*", "orders-created") {
		t.Fatal("expected wildcard match")
	}
	if !matchKafkaTopic(`re:^orders-[a-z]+$`, "orders-created") {
		t.Fatal("expected regex match")
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.KafkaConfig{Enabled: true, Port: 9092, Mocks: []config.KafkaMock{{ID: "1"}}}, nil, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "kafka" || info["port"] != 9092 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
