package ldapserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestBerLen(t *testing.T) {
	if got := berLen(10); len(got) != 1 || got[0] != 10 {
		t.Fatalf("unexpected short BER length: %v", got)
	}
	if got := berLen(200); len(got) != 2 {
		t.Fatalf("unexpected long BER length: %v", got)
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.LDAPConfig{Enabled: true, Port: 3893, Mocks: []config.LDAPMock{{ID: "1"}}}, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "ldap" || info["port"] != 3893 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
