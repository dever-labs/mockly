// White-box unit tests for dnsserver internals.
package dnsserver

import (
	"testing"

	"github.com/miekg/dns"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// New / SetMocks / GetMocks
// ---------------------------------------------------------------------------

func newTestDNSServer(t *testing.T, mocks []config.DNSMock) *Server {
	t.Helper()
	cfg := &config.DNSConfig{Enabled: true, Port: 0, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

func TestDNS_New_InitialMocks(t *testing.T) {
	mocks := []config.DNSMock{{ID: "m1", Name: "example.com", Type: "A"}}
	srv := newTestDNSServer(t, mocks)
	got := srv.GetMocks()
	if len(got) != 1 || got[0].ID != "m1" {
		t.Fatalf("unexpected mocks from New: %+v", got)
	}
}

func TestDNS_SetMocks_ReplacesList(t *testing.T) {
	srv := newTestDNSServer(t, nil)
	srv.SetMocks([]config.DNSMock{{ID: "a"}, {ID: "b"}})
	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
}

func TestDNS_SetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestDNSServer(t, nil)
	original := []config.DNSMock{{ID: "orig", Name: "x.com"}}
	srv.SetMocks(original)
	original[0].Name = "mutated.com"
	if srv.GetMocks()[0].Name != "x.com" {
		t.Error("SetMocks should copy the slice")
	}
}

func TestDNS_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestDNSServer(t, []config.DNSMock{{ID: "m1", Name: "example.com"}})
	got := srv.GetMocks()
	got[0].Name = "mutated.com"
	if srv.GetMocks()[0].Name != "example.com" {
		t.Error("GetMocks should return a copy")
	}
}

// ---------------------------------------------------------------------------
// normalizeDNSName
// ---------------------------------------------------------------------------

func TestNormalizeDNSName_Empty(t *testing.T) {
	if n := normalizeDNSName(""); n != "." {
		t.Errorf("empty should normalize to '.', got %q", n)
	}
}

func TestNormalizeDNSName_LowercasesAndTrims(t *testing.T) {
	if n := normalizeDNSName("  EXAMPLE.COM  "); n != "example.com." {
		t.Errorf("expected 'example.com.', got %q", n)
	}
}

func TestNormalizeDNSName_AlreadyDotTerminated(t *testing.T) {
	if n := normalizeDNSName("example.com."); n != "example.com." {
		t.Errorf("expected 'example.com.', got %q", n)
	}
}

func TestNormalizeDNSName_AddsDot(t *testing.T) {
	if n := normalizeDNSName("example.com"); n != "example.com." {
		t.Errorf("expected 'example.com.', got %q", n)
	}
}

// ---------------------------------------------------------------------------
// matchDNSName (extra cases beyond fault_test.go)
// ---------------------------------------------------------------------------

func TestMatchDNSName_CaseInsensitive(t *testing.T) {
	if !matchDNSName("EXAMPLE.COM", "example.com.") {
		t.Error("match should be case-insensitive")
	}
}

func TestMatchDNSName_WildcardNoMatchApex(t *testing.T) {
	if matchDNSName("*.example.com", "example.com") {
		t.Error("wildcard should not match the apex domain")
	}
}

func TestMatchDNSName_WildcardMatchesSubdomain(t *testing.T) {
	if !matchDNSName("*.example.com", "api.example.com") {
		t.Error("wildcard should match subdomain")
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestDNS_matchMock_TypeMismatch(t *testing.T) {
	srv := newTestDNSServer(t, []config.DNSMock{{ID: "m1", Name: "example.com", Type: "A"}})
	_, ok := srv.matchMock("example.com.", dns.TypeAAAA)
	if ok {
		t.Fatal("type mismatch should not match")
	}
}

func TestDNS_matchMock_ExactMatch(t *testing.T) {
	srv := newTestDNSServer(t, []config.DNSMock{{
		ID: "m1", Name: "example.com", Type: "A", Records: []string{"1.2.3.4"},
	}})
	m, ok := srv.matchMock("example.com.", dns.TypeA)
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match, got ok=%v", ok)
	}
}

func TestDNS_matchMock_StateCondition_NotMet(t *testing.T) {
	st := state.New()
	cfg := &config.DNSConfig{
		Mocks: []config.DNSMock{{
			ID: "m1", Name: "example.com", Type: "A",
			State: &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, scenarios.New(nil), logger.New(10))
	_, ok := srv.matchMock("example.com.", dns.TypeA)
	if ok {
		t.Fatal("should not match when state condition not met")
	}
}

func TestDNS_matchMock_StateCondition_Met(t *testing.T) {
	st := state.New()
	st.Set("mode", "on")
	cfg := &config.DNSConfig{
		Mocks: []config.DNSMock{{
			ID: "m1", Name: "example.com", Type: "A",
			State: &config.StateCondition{Key: "mode", Value: "on"},
		}},
	}
	srv := New(cfg, st, scenarios.New(nil), logger.New(10))
	m, ok := srv.matchMock("example.com.", dns.TypeA)
	if !ok || m.ID != "m1" {
		t.Fatalf("expected match when state condition met, got ok=%v", ok)
	}
}

// ---------------------------------------------------------------------------
// buildRRs
// ---------------------------------------------------------------------------

func TestBuildRRs_TypeA(t *testing.T) {
	mock := config.DNSMock{Records: []string{"1.2.3.4"}, TTL: 120}
	rrs := buildRRs("example.com", dns.TypeA, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 A record, got %d", len(rrs))
	}
	a, ok := rrs[0].(*dns.A)
	if !ok {
		t.Fatalf("want *dns.A, got %T", rrs[0])
	}
	if a.A.String() != "1.2.3.4" {
		t.Errorf("want 1.2.3.4, got %s", a.A.String())
	}
}

func TestBuildRRs_TypeAAAA(t *testing.T) {
	mock := config.DNSMock{Records: []string{"::1"}}
	rrs := buildRRs("example.com", dns.TypeAAAA, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 AAAA record, got %d", len(rrs))
	}
	_, ok := rrs[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("want *dns.AAAA, got %T", rrs[0])
	}
}

func TestBuildRRs_TypeCNAME(t *testing.T) {
	mock := config.DNSMock{Records: []string{"alias.example.com"}}
	rrs := buildRRs("example.com", dns.TypeCNAME, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 CNAME record, got %d", len(rrs))
	}
	c, ok := rrs[0].(*dns.CNAME)
	if !ok {
		t.Fatalf("want *dns.CNAME, got %T", rrs[0])
	}
	if c.Target != "alias.example.com." {
		t.Errorf("want 'alias.example.com.', got %q", c.Target)
	}
}

func TestBuildRRs_TypeMX(t *testing.T) {
	mock := config.DNSMock{Records: []string{"10 mail.example.com"}}
	rrs := buildRRs("example.com", dns.TypeMX, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 MX record, got %d", len(rrs))
	}
	m, ok := rrs[0].(*dns.MX)
	if !ok {
		t.Fatalf("want *dns.MX, got %T", rrs[0])
	}
	if m.Preference != 10 {
		t.Errorf("want preference 10, got %d", m.Preference)
	}
}

func TestBuildRRs_TypeMX_NoFields(t *testing.T) {
	mock := config.DNSMock{Records: []string{"mail.example.com"}}
	rrs := buildRRs("example.com", dns.TypeMX, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 MX record, got %d", len(rrs))
	}
	m := rrs[0].(*dns.MX)
	if m.Preference != 10 {
		t.Errorf("default MX preference should be 10, got %d", m.Preference)
	}
}

func TestBuildRRs_TypeTXT(t *testing.T) {
	mock := config.DNSMock{Records: []string{"v=spf1 include:example.com ~all"}}
	rrs := buildRRs("example.com", dns.TypeTXT, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 TXT record, got %d", len(rrs))
	}
	txt, ok := rrs[0].(*dns.TXT)
	if !ok || len(txt.Txt) == 0 {
		t.Fatalf("want *dns.TXT with content, got %T", rrs[0])
	}
}

func TestBuildRRs_TypeNS(t *testing.T) {
	mock := config.DNSMock{Records: []string{"ns1.example.com"}}
	rrs := buildRRs("example.com", dns.TypeNS, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 NS record, got %d", len(rrs))
	}
	ns, ok := rrs[0].(*dns.NS)
	if !ok {
		t.Fatalf("want *dns.NS, got %T", rrs[0])
	}
	if ns.Ns != "ns1.example.com." {
		t.Errorf("want 'ns1.example.com.', got %q", ns.Ns)
	}
}

func TestBuildRRs_TypeSRV(t *testing.T) {
	mock := config.DNSMock{Records: []string{"10 5 443 svc.example.com"}}
	rrs := buildRRs("_svc._tcp.example.com", dns.TypeSRV, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 SRV record, got %d", len(rrs))
	}
	srv, ok := rrs[0].(*dns.SRV)
	if !ok {
		t.Fatalf("want *dns.SRV, got %T", rrs[0])
	}
	if srv.Priority != 10 || srv.Weight != 5 || srv.Port != 443 {
		t.Errorf("unexpected SRV values: %+v", srv)
	}
}

func TestBuildRRs_TypeSRV_NoFields(t *testing.T) {
	mock := config.DNSMock{Records: []string{"svc.example.com"}}
	rrs := buildRRs("_svc._tcp.example.com", dns.TypeSRV, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 SRV record, got %d", len(rrs))
	}
	srv := rrs[0].(*dns.SRV)
	if srv.Priority != 10 {
		t.Errorf("default SRV priority should be 10, got %d", srv.Priority)
	}
}

func TestBuildRRs_InvalidA_Skipped(t *testing.T) {
	mock := config.DNSMock{Records: []string{"not-an-ip"}}
	rrs := buildRRs("example.com", dns.TypeA, mock)
	if len(rrs) != 0 {
		t.Errorf("invalid A record should be skipped, got %d records", len(rrs))
	}
}

func TestBuildRRs_DefaultTTL(t *testing.T) {
	mock := config.DNSMock{Records: []string{"1.2.3.4"}}
	rrs := buildRRs("example.com", dns.TypeA, mock)
	if len(rrs) != 1 {
		t.Fatalf("want 1 A record, got %d", len(rrs))
	}
	if rrs[0].Header().Ttl != 60 {
		t.Errorf("default TTL should be 60, got %d", rrs[0].Header().Ttl)
	}
}

func TestBuildRRs_MultipleRecords(t *testing.T) {
	mock := config.DNSMock{Records: []string{"1.2.3.4", "5.6.7.8"}}
	rrs := buildRRs("example.com", dns.TypeA, mock)
	if len(rrs) != 2 {
		t.Fatalf("want 2 A records, got %d", len(rrs))
	}
}

// ---------------------------------------------------------------------------
// StatusInfo
// ---------------------------------------------------------------------------

func TestDNS_StatusInfo(t *testing.T) {
	srv := newTestDNSServer(t, []config.DNSMock{{ID: "m1"}, {ID: "m2"}})
	info := srv.StatusInfo()
	if info["protocol"] != "dns" {
		t.Errorf("unexpected protocol %v", info["protocol"])
	}
	if info["mocks"] != 2 {
		t.Errorf("want mocks=2, got %v", info["mocks"])
	}
}

// ---------------------------------------------------------------------------
// isDNSServerClosed
// ---------------------------------------------------------------------------

func TestIsDNSServerClosed(t *testing.T) {
	if isDNSServerClosed(nil) {
		t.Error("nil error should not be 'closed'")
	}
}
