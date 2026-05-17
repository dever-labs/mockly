package amqpserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestMatchAMQPRoutingKey(t *testing.T) {
	if !matchAMQPRoutingKey("events.*", "events.created") {
		t.Fatal("expected wildcard match")
	}
	if !matchAMQPRoutingKey(`re:^events\.[a-z]+$`, "events.created") {
		t.Fatal("expected regex match")
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.AMQPConfig{Enabled: true, Port: 5672, Mocks: []config.AMQPMock{{ID: "1"}}}, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "amqp" || info["port"] != 5672 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
