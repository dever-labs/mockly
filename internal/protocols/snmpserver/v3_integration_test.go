// Integration tests for SNMPv3 (USM) requests — sends real authenticated
// (and encrypted) SNMP requests using gosnmp, exercising the engine
// discovery handshake and USM auth/priv handling end to end. mockly's
// previous GoSNMPServer-based implementation had no equivalent coverage;
// see issue #113.
package snmpserver

import (
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func v3Client(port int, user config.SNMPUser) *gosnmp.GoSNMP {
	return &gosnmp.GoSNMP{
		Target:        "127.0.0.1",
		Port:          uint16(port),
		Version:       gosnmp.Version3,
		Timeout:       2 * time.Second,
		Retries:       1,
		SecurityModel: gosnmp.UserSecurityModel,
		MsgFlags:      gosnmp.AuthPriv,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName:                 user.Username,
			AuthenticationProtocol:   authProtocol(user.AuthProtocol),
			AuthenticationPassphrase: user.AuthPassphrase,
			PrivacyProtocol:          privProtocol(user.PrivProtocol),
			PrivacyPassphrase:        user.PrivPassphrase,
		},
	}
}

func TestSNMPServer_V3_GetAuthPriv(t *testing.T) {
	port := freeUDPPort()
	user := config.SNMPUser{
		Username:       "admin",
		AuthProtocol:   "sha",
		AuthPassphrase: "authpass1234",
		PrivProtocol:   "aes",
		PrivPassphrase: "privpass1234",
	}
	srv := New(&config.SNMPConfig{
		Enabled: true,
		Port:    port,
		Mocks:   defaultMocks(),
		V3Users: []config.SNMPUser{user},
	}, state.New(), scenarios.New(nil), logger.New(100))
	defer startSNMP(t, srv)()

	g := v3Client(port, user)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	result, err := g.Get([]string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(result.Variables) != 1 {
		t.Fatalf("want 1 variable, got %d", len(result.Variables))
	}
	got := string(result.Variables[0].Value.([]byte))
	if got != "Mockly Test Device" {
		t.Fatalf("value = %q, want %q", got, "Mockly Test Device")
	}
}

func TestSNMPServer_V3_SetAuthPriv(t *testing.T) {
	port := freeUDPPort()
	user := config.SNMPUser{
		Username:       "admin",
		AuthProtocol:   "sha256",
		AuthPassphrase: "authpass1234",
		PrivProtocol:   "aes256",
		PrivPassphrase: "privpass1234",
	}
	srv := New(&config.SNMPConfig{
		Enabled: true,
		Port:    port,
		Mocks:   defaultMocks(),
		V3Users: []config.SNMPUser{user},
	}, state.New(), scenarios.New(nil), logger.New(100))
	defer startSNMP(t, srv)()

	g := v3Client(port, user)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	pdu := gosnmp.SnmpPDU{Name: "1.3.6.1.2.1.1.1.0", Type: gosnmp.OctetString, Value: "Updated Device"}
	if _, err := g.Set([]gosnmp.SnmpPDU{pdu}); err != nil {
		t.Fatalf("set: %v", err)
	}

	result, err := g.Get([]string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	got := string(result.Variables[0].Value.([]byte))
	if got != "Updated Device" {
		t.Fatalf("value = %q, want %q", got, "Updated Device")
	}
}

func TestSNMPServer_V3_WrongPassphrase_NoResponse(t *testing.T) {
	port := freeUDPPort()
	user := config.SNMPUser{
		Username:       "admin",
		AuthProtocol:   "sha",
		AuthPassphrase: "authpass1234",
		PrivProtocol:   "aes",
		PrivPassphrase: "privpass1234",
	}
	srv := New(&config.SNMPConfig{
		Enabled: true,
		Port:    port,
		Mocks:   defaultMocks(),
		V3Users: []config.SNMPUser{user},
	}, state.New(), scenarios.New(nil), logger.New(100))
	defer startSNMP(t, srv)()

	wrong := user
	wrong.AuthPassphrase = "wrongpassphrase"
	g := v3Client(port, wrong)
	g.Retries = 0
	g.Timeout = 500 * time.Millisecond
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	if _, err := g.Get([]string{"1.3.6.1.2.1.1.1.0"}); err == nil {
		t.Fatal("expected a timeout/auth error for wrong passphrase, got none")
	}
}

func TestSNMPServer_V3Only_RejectsV2c(t *testing.T) {
	port := freeUDPPort()
	user := config.SNMPUser{
		Username:       "admin",
		AuthProtocol:   "sha",
		AuthPassphrase: "authpass1234",
		PrivProtocol:   "aes",
		PrivPassphrase: "privpass1234",
	}
	srv := New(&config.SNMPConfig{
		Enabled:         true,
		Port:            port,
		Community:       "public",
		ProtocolVersion: "v3",
		Mocks:           defaultMocks(),
		V3Users:         []config.SNMPUser{user},
	}, state.New(), scenarios.New(nil), logger.New(100))
	defer startSNMP(t, srv)()

	g := snmpClient(port)
	g.Retries = 0
	g.Timeout = 500 * time.Millisecond
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	if _, err := g.Get([]string{"1.3.6.1.2.1.1.1.0"}); err == nil {
		t.Fatal("expected v2c request to be rejected/dropped when ProtocolVersion=v3")
	}
}
