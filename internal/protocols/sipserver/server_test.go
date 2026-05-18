package sipserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestMatchSIPURI(t *testing.T) {
	if !matchSIPURI("sip:*@example.com", "sip:alice@example.com") {
		t.Fatal("expected wildcard URI match")
	}
	if !matchSIPURI(`re:^sip:[a-z]+@example\.com$`, "sip:alice@example.com") {
		t.Fatal("expected regex URI match")
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.SIPConfig{Enabled: true, Port: 5060, Mocks: []config.SIPMock{{ID: "1"}}}, nil, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "sip" || info["port"] != 5060 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
