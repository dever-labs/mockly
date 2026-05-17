package coapserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestMatchCoAPPath(t *testing.T) {
	if !matchCoAPPath("/sensor/*", "/sensor/temp") {
		t.Fatal("expected wildcard path match")
	}
	if !matchCoAPPath(`re:^/sensor/[a-z]+$`, "/sensor/temp") {
		t.Fatal("expected regex path match")
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.CoAPConfig{Enabled: true, Port: 5683, Mocks: []config.CoAPMock{{ID: "1"}}}, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "coap" || info["port"] != 5683 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
