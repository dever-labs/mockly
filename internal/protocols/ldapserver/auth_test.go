package ldapserver_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/ldapserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// buildBindRequestWithCreds builds a simple-auth BindRequest with an
// explicit bind DN and password, unlike fault_test.go's buildBindRequest
// (which always sends an anonymous bind).
func buildBindRequestWithCreds(msgID int, dn, password string) []byte {
	content := append(berTLV(0x02, []byte{0x03}), berTLV(0x04, []byte(dn))...)
	content = append(content, berTLV(0x80, []byte(password))...)
	return buildLDAPMessage(msgID, 0x60, content)
}

func dialLDAP(t *testing.T, port int) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial LDAP: %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	return conn
}

func TestLDAPServer_Auth_UnconfiguredUsers_BindAlwaysSucceeds(t *testing.T) {
	port := freePort(t)
	srv := ldapserver.New(&config.LDAPConfig{Enabled: true, Port: port}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn := dialLDAP(t, port)
	defer conn.Close() //nolint:errcheck

	if _, err := conn.Write(buildBindRequestWithCreds(1, "anything", "wrong")); err != nil {
		t.Fatalf("write bind: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x61 || code != 0x00 {
		t.Fatalf("bind response tag/code = %#x/%#x, want 0x61/0x00 (unconfigured users should always succeed)", tag, code)
	}
}

func TestLDAPServer_Auth_WrongCredentials_Rejected(t *testing.T) {
	port := freePort(t)
	srv := ldapserver.New(&config.LDAPConfig{
		Enabled: true,
		Port:    port,
		Users:   []config.LDAPUser{{Username: "alice", Password: "secret123"}},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn := dialLDAP(t, port)
	defer conn.Close() //nolint:errcheck

	if _, err := conn.Write(buildBindRequestWithCreds(1, "alice", "wrong")); err != nil {
		t.Fatalf("write bind: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x61 || code != 49 {
		t.Fatalf("bind response tag/code = %#x/%d, want 0x61/49 (invalidCredentials)", tag, code)
	}
}

func TestLDAPServer_Auth_CorrectCredentials_Succeeds(t *testing.T) {
	port := freePort(t)
	srv := ldapserver.New(&config.LDAPConfig{
		Enabled: true,
		Port:    port,
		Users:   []config.LDAPUser{{Username: "alice", Password: "secret123"}},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn := dialLDAP(t, port)
	defer conn.Close() //nolint:errcheck

	if _, err := conn.Write(buildBindRequestWithCreds(1, "alice", "secret123")); err != nil {
		t.Fatalf("write bind: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x61 || code != 0x00 {
		t.Fatalf("bind response tag/code = %#x/%#x, want 0x61/0x00", tag, code)
	}
}

func TestLDAPServer_Auth_SearchBeforeBind_Rejected(t *testing.T) {
	port := freePort(t)
	srv := ldapserver.New(&config.LDAPConfig{
		Enabled: true,
		Port:    port,
		Users:   []config.LDAPUser{{Username: "alice", Password: "secret123"}},
		Mocks: []config.LDAPMock{{
			ID:         "m1",
			BaseDN:     "dc=example,dc=com",
			Attributes: map[string][]string{"cn": {"testuser"}},
		}},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn := dialLDAP(t, port)
	defer conn.Close() //nolint:errcheck

	// Search without a prior successful bind should be rejected, not
	// silently served, once Users is configured.
	if _, err := conn.Write(buildSearchRequest(1)); err != nil {
		t.Fatalf("write search: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x65 || code != 50 {
		t.Fatalf("search response tag/code = %#x/%d, want 0x65/50 (insufficientAccessRights)", tag, code)
	}
}

func TestLDAPServer_Auth_AllowedMockIDs_RestrictsSearch(t *testing.T) {
	port := freePort(t)
	srv := ldapserver.New(&config.LDAPConfig{
		Enabled: true,
		Port:    port,
		Users: []config.LDAPUser{
			{Username: "alice", Password: "secret123", AllowedMockIDs: []string{"m-alice"}},
		},
		Mocks: []config.LDAPMock{
			{ID: "m-alice", BaseDN: "dc=example,dc=com", Attributes: map[string][]string{"cn": {"alice"}}},
			{ID: "m-bob", BaseDN: "dc=example,dc=com", Attributes: map[string][]string{"cn": {"bob"}}},
		},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn := dialLDAP(t, port)
	defer conn.Close() //nolint:errcheck

	if _, err := conn.Write(buildBindRequestWithCreds(1, "alice", "secret123")); err != nil {
		t.Fatalf("write bind: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x61 || code != 0x00 {
		t.Fatalf("bind response tag/code = %#x/%#x, want 0x61/0x00", tag, code)
	}

	if _, err := conn.Write(buildSearchRequest(2)); err != nil {
		t.Fatalf("write search: %v", err)
	}
	// Only m-alice should be returned, then the SearchResultDone.
	if tag, _ := ldapResultCode(readBERPacket(t, conn)); tag != 0x64 {
		t.Fatalf("expected one SearchResultEntry (0x64), got tag %#x", tag)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x65 || code != 0x00 {
		t.Fatalf("search done tag/code = %#x/%#x, want 0x65/0x00 (should not include m-bob)", tag, code)
	}
}
