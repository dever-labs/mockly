package dnsserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestMatchDNSNameExact(t *testing.T) {
	if !matchDNSName("example.com", "example.com.") {
		t.Fatal("expected exact DNS match")
	}
}

func TestMatchDNSNameWildcard(t *testing.T) {
	if !matchDNSName("*.example.com", "api.example.com") {
		t.Fatal("expected wildcard DNS match")
	}
	if matchDNSName("*.example.com", "example.com") {
		t.Fatal("wildcard should not match apex")
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.DNSConfig{Enabled: true, Port: 5353, Mocks: []config.DNSMock{{ID: "1"}}}, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "dns" || info["port"] != 5353 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
