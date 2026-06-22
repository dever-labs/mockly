// White-box unit tests for sipserver helpers.
package sipserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func newTestSIPServer(mocks []config.SIPMock) *Server {
	cfg := &config.SIPConfig{Enabled: true, Port: 5060, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

// ---------------------------------------------------------------------------
// SetMocks / GetMocks
// ---------------------------------------------------------------------------

func TestSIP_SetGetMocks(t *testing.T) {
	srv := newTestSIPServer(nil)
	mocks := []config.SIPMock{
		{ID: "m1", Method: "INVITE", URI: "sip:alice@example.com"},
		{ID: "m2", Method: "BYE"},
	}
	srv.SetMocks(mocks)
	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
	if got[0].ID != "m1" || got[1].ID != "m2" {
		t.Errorf("unexpected mocks: %+v", got)
	}
}

func TestSIP_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestSIPServer([]config.SIPMock{{ID: "m1"}})
	got := srv.GetMocks()
	got[0].ID = "mutated"
	if srv.GetMocks()[0].ID != "m1" {
		t.Error("GetMocks should return a copy")
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestSIP_MatchMock_NoMocks(t *testing.T) {
	srv := newTestSIPServer(nil)
	if _, ok, _ := srv.matchMock("INVITE", "sip:bob@example.com"); ok {
		t.Error("should not match when there are no mocks")
	}
}

func TestSIP_MatchMock_MethodMismatch(t *testing.T) {
	srv := newTestSIPServer([]config.SIPMock{
		{ID: "m1", Method: "INVITE", URI: "sip:alice@example.com"},
	})
	if _, ok, _ := srv.matchMock("BYE", "sip:alice@example.com"); ok {
		t.Error("should not match different method")
	}
}

func TestSIP_MatchMock_URIMismatch(t *testing.T) {
	srv := newTestSIPServer([]config.SIPMock{
		{ID: "m1", Method: "INVITE", URI: "sip:alice@example.com"},
	})
	if _, ok, _ := srv.matchMock("INVITE", "sip:bob@example.com"); ok {
		t.Error("should not match different URI")
	}
}

func TestSIP_MatchMock_Exact(t *testing.T) {
	srv := newTestSIPServer([]config.SIPMock{
		{ID: "m1", Method: "INVITE", URI: "sip:alice@example.com", Response: config.SIPResponse{Status: 200}},
	})
	m, ok, _ := srv.matchMock("INVITE", "sip:alice@example.com")
	if !ok {
		t.Fatal("should match")
	}
	if m.ID != "m1" {
		t.Errorf("unexpected mock ID: %s", m.ID)
	}
}

func TestSIP_MatchMock_URIRegex(t *testing.T) {
	srv := newTestSIPServer([]config.SIPMock{
		{ID: "m1", Method: "INVITE", URIRegex: `^sip:[a-z]+@example\.com$`, Response: config.SIPResponse{Status: 200}},
	})

	if _, ok, _ := srv.matchMock("INVITE", "sip:alice@example.com"); !ok {
		t.Fatal("expected uri_regex to match sip:alice@example.com")
	}
	if _, ok, _ := srv.matchMock("INVITE", "sip:alice@other.com"); ok {
		t.Fatal("expected uri_regex to reject sip:alice@other.com")
	}
}

func TestSIP_MatchMock_WildcardMethod(t *testing.T) {
	srv := newTestSIPServer([]config.SIPMock{
		{ID: "m1", Method: "*", URI: ""},
	})
	if _, ok, _ := srv.matchMock("OPTIONS", "sip:x@y.com"); !ok {
		t.Error("wildcard method should match any method")
	}
}

func TestSIP_MatchMock_StateCondition(t *testing.T) {
	st := state.New()
	cfg := &config.SIPConfig{Enabled: true, Port: 5060, Mocks: []config.SIPMock{
		{ID: "m1", Method: "*", State: &config.StateCondition{Key: "mode", Value: "busy"}},
	}}
	srv := New(cfg, st, scenarios.New(nil), logger.New(100))

	// State not set — should not match.
	if _, ok, _ := srv.matchMock("INVITE", "sip:a@b.com"); ok {
		t.Error("should not match when state condition is unmet")
	}

	// State set — should match.
	st.Set("mode", "busy")
	if _, ok, _ := srv.matchMock("INVITE", "sip:a@b.com"); !ok {
		t.Error("should match when state condition is met")
	}
}

// ---------------------------------------------------------------------------
// matchSIPURI extras
// ---------------------------------------------------------------------------

func TestMatchSIPURI_Empty(t *testing.T) {
	if ok, _ := matchSIPURI("", "sip:any@example.com"); !ok {
		t.Error("empty pattern should match any URI")
	}
}

func TestMatchSIPURI_Exact(t *testing.T) {
	if ok, _ := matchSIPURI("sip:alice@example.com", "sip:alice@example.com"); !ok {
		t.Error("exact pattern should match identical URI")
	}
	if ok, _ := matchSIPURI("sip:alice@example.com", "sip:bob@example.com"); ok {
		t.Error("exact pattern should not match different URI")
	}
}

func TestMatchSIPURI_InvalidRegex(t *testing.T) {
	if ok, _ := matchSIPURI("re:[bad", "sip:any@example.com"); ok {
		t.Error("invalid regex should not match")
	}
}

func TestMatchSIPURI_WildcardNoSuffix(t *testing.T) {
	if ok, _ := matchSIPURI("sip:*@example.com", "sip:alice@example.com"); !ok {
		t.Error("wildcard prefix should match")
	}
	if ok, _ := matchSIPURI("sip:*@example.com", "sip:alice@other.com"); ok {
		t.Error("wildcard prefix should not match wrong suffix")
	}
}

func TestMatchSIPURI_NamedSegment(t *testing.T) {
	ok, params := matchSIPURI("sip:host/{resource}", "sip:host/room42")
	if !ok {
		t.Fatal("expected named segment URI to match")
	}
	if params["resource"] != "room42" {
		t.Errorf("want resource=room42, got %q", params["resource"])
	}
}

// ---------------------------------------------------------------------------
// defaultSIPReason
// ---------------------------------------------------------------------------

func TestDefaultSIPReason_AllCodes(t *testing.T) {
	cases := map[int]string{
		100: "Trying",
		180: "Ringing",
		200: "OK",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		486: "Busy Here",
		487: "Request Terminated",
		500: "Server Internal Error",
		503: "Service Unavailable",
		999: "OK", // default
	}
	for code, want := range cases {
		got := defaultSIPReason(code)
		if got != want {
			t.Errorf("defaultSIPReason(%d) = %q, want %q", code, got, want)
		}
	}
}
