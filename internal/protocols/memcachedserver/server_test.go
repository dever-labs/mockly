package memcachedserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestMatchMemcachedKey(t *testing.T) {
	if !matchMemcachedKey("cache:*", "cache:user") {
		t.Fatal("expected wildcard match")
	}
	if !matchMemcachedKey(`re:^cache:[a-z]+$`, "cache:user") {
		t.Fatal("expected regex match")
	}
	if matchMemcachedKey("cache:*", "other:user") {
		t.Fatal("unexpected wildcard match")
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.MemcachedConfig{Enabled: true, Port: 11211, Mocks: []config.MemcachedMock{{ID: "1"}}}, nil, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "memcached" || info["port"] != 11211 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
