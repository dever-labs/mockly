package stompserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestMatchDestination(t *testing.T) {
	if !matchDestination("/queue/*", "/queue/jobs") {
		t.Fatal("expected wildcard destination match")
	}
	if !matchDestination(`re:^/topic/[a-z]+$`, "/topic/orders") {
		t.Fatal("expected regex destination match")
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.STOMPConfig{Enabled: true, Port: 61613, Mocks: []config.STOMPMock{{ID: "1"}}}, nil, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "stomp" || info["port"] != 61613 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
