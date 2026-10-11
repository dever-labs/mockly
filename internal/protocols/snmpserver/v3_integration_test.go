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

// TestSNMPServer_V3_StaleReplayedTime_Dropped is a white-box test for the
// USM anti-replay check (RFC 3414 §3.2 step 7): a validly-signed request
// claiming an older AuthoritativeEngineTime than one already accepted from
// the same user (simulating a captured-and-replayed older message) must be
// rejected, even though its digest is perfectly valid. RFC 3414 §3.2 step
// 7(a) requires a signed Report PDU carrying usmStatsNotInTimeWindows (not
// a silent drop), so a legitimate client can resynchronize and retry.
func TestSNMPServer_V3_StaleReplayedTime_Dropped(t *testing.T) {
	const passphrase = "realpassphrase123"
	user := config.SNMPUser{Username: "admin", AuthProtocol: "sha", AuthPassphrase: passphrase}
	srv := New(&config.SNMPConfig{
		Enabled: true, Mocks: defaultMocks(), V3Users: []config.SNMPUser{user},
	}, state.New(), scenarios.New(nil), logger.New(100))
	users, err := srv.buildUsers()
	if err != nil {
		t.Fatal(err)
	}
	// Fixed engineTime so the test exercises replay detection specifically,
	// independent of the (already covered) ±150s window check.
	ag := newAgent("", true, users, "deadbeefdead", func() uint32 { return 100 }, srv.buildOIDs(defaultMocks()))

	buildAndSend := func(t *testing.T, requestID uint32, claimedTime uint32) []byte {
		t.Helper()
		usp := &gosnmp.UsmSecurityParameters{
			UserName:                 user.Username,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: passphrase,
			AuthoritativeEngineID:    "deadbeefdead",
			AuthoritativeEngineBoots: 1,
			AuthoritativeEngineTime:  claimedTime,
		}
		if err := usp.InitSecurityKeys(); err != nil {
			t.Fatal(err)
		}
		pkt := &gosnmp.SnmpPacket{
			Version: gosnmp.Version3, MsgFlags: gosnmp.AuthNoPriv, SecurityModel: gosnmp.UserSecurityModel,
			SecurityParameters: usp, PDUType: gosnmp.GetRequest, MsgID: requestID, RequestID: requestID,
			Variables: []gosnmp.SnmpPDU{{Name: defaultMocks()[0].OID, Type: gosnmp.Null}},
		}
		raw, err := pkt.MarshalMsg()
		if err != nil {
			t.Fatal(err)
		}
		respRaw, err := ag.respond(raw)
		if err != nil {
			t.Fatalf("respond: %v", err)
		}
		return respRaw
	}

	if resp := buildAndSend(t, 1, 100); resp == nil {
		t.Fatal("a valid, well-timed request was unexpectedly dropped")
	}
	resp := buildAndSend(t, 2, 50)
	if resp == nil {
		t.Fatal("want a signed Report PDU for a stale/replayed time, got a silent drop")
	}
	probe := &gosnmp.GoSNMP{
		Version: gosnmp.Version3, SecurityModel: gosnmp.UserSecurityModel,
		SecurityParameters: &gosnmp.UsmSecurityParameters{UserName: user.Username, AuthenticationProtocol: gosnmp.SHA, AuthenticationPassphrase: passphrase, PrivacyProtocol: gosnmp.NoPriv},
	}
	decoded, err := probe.SnmpDecodePacket(resp)
	if err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if decoded.PDUType != gosnmp.Report {
		t.Errorf("want a Report PDU, got %v", decoded.PDUType)
	}
	if len(decoded.Variables) != 1 || decoded.Variables[0].Name != ".1.3.6.1.6.3.15.1.1.2.0" {
		t.Errorf("want usmStatsNotInTimeWindows varbind, got %+v", decoded.Variables)
	}
}
// server's USM authentication check: it crafts a GetRequest for a real,
// configured username but signs it with the WRONG passphrase, submitting
// the raw bytes directly to agent.respond (bypassing gosnmp's client-side
// request/response verification entirely, so a prior bug here — where the
// server never validated the digest at all — couldn't hide behind the
// client rejecting its own expected response). The server must drop the
// forged request (nil response), not process it.
func TestSNMPServer_V3_ForgedAuthDigest_Dropped(t *testing.T) {
	const realPassphrase = "realpassphrase123"
	user := config.SNMPUser{Username: "admin", AuthProtocol: "sha", AuthPassphrase: realPassphrase}
	srv := New(&config.SNMPConfig{
		Enabled: true, Mocks: defaultMocks(), V3Users: []config.SNMPUser{user},
	}, state.New(), scenarios.New(nil), logger.New(100))
	users, err := srv.buildUsers()
	if err != nil {
		t.Fatal(err)
	}
	ag := newAgent("", true, users, "deadbeefdead", func() uint32 { return 1 }, srv.buildOIDs(defaultMocks()))

	// Discovery handshake, to learn the agent's engine ID/boots/time.
	discPkt := &gosnmp.SnmpPacket{
		Version: gosnmp.Version3, MsgFlags: gosnmp.Reportable | gosnmp.NoAuthNoPriv,
		SecurityModel: gosnmp.UserSecurityModel, SecurityParameters: &gosnmp.UsmSecurityParameters{},
		PDUType: gosnmp.GetRequest, MsgID: 1, RequestID: 1,
	}
	discRaw, err := discPkt.MarshalMsg()
	if err != nil {
		t.Fatal(err)
	}
	discRespRaw, err := ag.respond(discRaw)
	if err != nil {
		t.Fatalf("discovery respond: %v", err)
	}
	probe := &gosnmp.GoSNMP{
		Version: gosnmp.Version3, SecurityModel: gosnmp.UserSecurityModel,
		SecurityParameters: &gosnmp.UsmSecurityParameters{UserName: user.Username},
	}
	discResp, err := probe.SnmpDecodePacket(discRespRaw)
	if err != nil {
		t.Fatalf("decode discovery response: %v", err)
	}
	engineUSP, ok := discResp.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if !ok {
		t.Fatalf("discovery response has no USM security parameters")
	}

	// A GetRequest for the real username, authenticated with the WRONG
	// passphrase but a correctly-learned engine ID/boots/time.
	forgedUSP := &gosnmp.UsmSecurityParameters{
		UserName:                 user.Username,
		AuthenticationProtocol:   gosnmp.SHA,
		AuthenticationPassphrase: realPassphrase + "-wrong",
		AuthoritativeEngineID:    engineUSP.AuthoritativeEngineID,
		AuthoritativeEngineBoots: engineUSP.AuthoritativeEngineBoots,
		AuthoritativeEngineTime:  engineUSP.AuthoritativeEngineTime,
	}
	if err := forgedUSP.InitSecurityKeys(); err != nil {
		t.Fatal(err)
	}
	forgedPkt := &gosnmp.SnmpPacket{
		Version: gosnmp.Version3, MsgFlags: gosnmp.AuthNoPriv, SecurityModel: gosnmp.UserSecurityModel,
		SecurityParameters: forgedUSP, PDUType: gosnmp.GetRequest, MsgID: 2, RequestID: 2,
		Variables: []gosnmp.SnmpPDU{{Name: defaultMocks()[0].OID, Type: gosnmp.Null}},
	}
	forgedRaw, err := forgedPkt.MarshalMsg()
	if err != nil {
		t.Fatal(err)
	}

	respRaw, err := ag.respond(forgedRaw)
	if err != nil {
		t.Fatalf("respond returned an error (acceptable, but expected a nil response): %v", err)
	}
	if respRaw != nil {
		t.Fatal("server responded to a request signed with the wrong auth passphrase; USM authentication is not being enforced")
	}
}
