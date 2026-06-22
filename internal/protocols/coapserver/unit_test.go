// White-box unit tests for coapserver helpers.
package coapserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func newTestCoAPServer(mocks []config.CoAPMock) *Server {
	cfg := &config.CoAPConfig{Enabled: true, Port: 5683, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

// ---------------------------------------------------------------------------
// SetMocks / GetMocks
// ---------------------------------------------------------------------------

func TestCoAP_SetGetMocks(t *testing.T) {
	srv := newTestCoAPServer(nil)
	mocks := []config.CoAPMock{
		{ID: "m1", Method: "GET", Path: "/sensor/temp"},
		{ID: "m2", Method: "POST", Path: "/actuator"},
	}
	srv.SetMocks(mocks)
	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
}

func TestCoAP_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestCoAPServer([]config.CoAPMock{{ID: "m1"}})
	got := srv.GetMocks()
	got[0].ID = "mutated"
	if srv.GetMocks()[0].ID != "m1" {
		t.Error("GetMocks should return a copy")
	}
}

// ---------------------------------------------------------------------------
// coapMethod
// ---------------------------------------------------------------------------

func TestCoapMethod_AllCodes(t *testing.T) {
	cases := map[byte]string{
		0x01: "GET",
		0x02: "POST",
		0x03: "PUT",
		0x04: "DELETE",
		0x00: "",
		0x10: "",
	}
	for code, want := range cases {
		got := coapMethod(code)
		if got != want {
			t.Errorf("coapMethod(0x%02x) = %q, want %q", code, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// coapResponseCode
// ---------------------------------------------------------------------------

func TestCoapResponseCode_AllCodes(t *testing.T) {
	cases := map[string]byte{
		"2.01": 0x41,
		"2.04": 0x44,
		"2.05": 0x45,
		"4.00": 0x80,
		"4.04": 0x84,
		"5.00": 0xA0,
		"":     0x84, // default
		"9.99": 0x84, // unknown → default
	}
	for code, want := range cases {
		got := coapResponseCode(code)
		if got != want {
			t.Errorf("coapResponseCode(%q) = 0x%02x, want 0x%02x", code, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// matchMock
// ---------------------------------------------------------------------------

func TestCoAP_MatchMock_NoMocks(t *testing.T) {
	srv := newTestCoAPServer(nil)
	if _, ok, _ := srv.matchMock("GET", "/sensor/temp"); ok {
		t.Error("should not match when there are no mocks")
	}
}

func TestCoAP_MatchMock_MethodMismatch(t *testing.T) {
	srv := newTestCoAPServer([]config.CoAPMock{
		{ID: "m1", Method: "GET", Path: "/sensor/temp"},
	})
	if _, ok, _ := srv.matchMock("POST", "/sensor/temp"); ok {
		t.Error("should not match different method")
	}
}

func TestCoAP_MatchMock_PathMismatch(t *testing.T) {
	srv := newTestCoAPServer([]config.CoAPMock{
		{ID: "m1", Method: "GET", Path: "/sensor/temp"},
	})
	if _, ok, _ := srv.matchMock("GET", "/sensor/humidity"); ok {
		t.Error("should not match different path")
	}
}

func TestCoAP_MatchMock_Exact(t *testing.T) {
	srv := newTestCoAPServer([]config.CoAPMock{
		{ID: "m1", Method: "GET", Path: "/sensor/temp", Response: config.CoAPResponse{Code: "2.05"}},
	})
	m, ok, _ := srv.matchMock("GET", "/sensor/temp")
	if !ok {
		t.Fatal("should match")
	}
	if m.ID != "m1" {
		t.Errorf("unexpected mock ID: %s", m.ID)
	}
}

func TestCoAP_MatchMock_PathRegex(t *testing.T) {
	srv := newTestCoAPServer([]config.CoAPMock{
		{ID: "m1", Method: "GET", PathRegex: `^/sensors/[a-z]+$`, Response: config.CoAPResponse{Code: "2.05"}},
	})

	if _, ok, _ := srv.matchMock("GET", "/sensors/temp"); !ok {
		t.Fatal("expected path_regex to match /sensors/temp")
	}
	if _, ok, _ := srv.matchMock("GET", "/sensors/123"); ok {
		t.Fatal("expected path_regex to reject /sensors/123")
	}
}

func TestCoAP_MatchMock_NamedParam(t *testing.T) {
	srv := newTestCoAPServer([]config.CoAPMock{
		{ID: "m1", Method: "GET", Path: "/sensors/{type}", Response: config.CoAPResponse{Code: "2.05"}},
	})

	_, ok, params := srv.matchMock("GET", "/sensors/humidity")
	if !ok {
		t.Fatal("expected named param match")
	}
	if params["type"] != "humidity" {
		t.Errorf("want type=humidity, got %q", params["type"])
	}
}

func TestCoAP_MatchMock_StateCondition(t *testing.T) {
	st := state.New()
	cfg := &config.CoAPConfig{Enabled: true, Port: 5683, Mocks: []config.CoAPMock{
		{ID: "m1", Method: "GET", Path: "/sensor/*", State: &config.StateCondition{Key: "active", Value: "yes"}},
	}}
	srv := New(cfg, st, scenarios.New(nil), logger.New(100))

	if _, ok, _ := srv.matchMock("GET", "/sensor/temp"); ok {
		t.Error("should not match when state condition is unmet")
	}

	st.Set("active", "yes")
	if _, ok, _ := srv.matchMock("GET", "/sensor/temp"); !ok {
		t.Error("should match when state condition is met")
	}
}

// ---------------------------------------------------------------------------
// matchCoAPPath extras
// ---------------------------------------------------------------------------

func TestMatchCoAPPath_Empty(t *testing.T) {
	if ok, _ := engine.MatchPath("", "/any/path"); !ok {
		t.Error("empty pattern should match any path")
	}
}

func TestMatchCoAPPath_Exact(t *testing.T) {
	if ok, _ := engine.MatchPath("/sensor/temp", "/sensor/temp"); !ok {
		t.Error("exact pattern should match identical path")
	}
	if ok, _ := engine.MatchPath("/sensor/temp", "/sensor/humidity"); ok {
		t.Error("exact pattern should not match different path")
	}
}

func TestMatchCoAPPath_InvalidRegex(t *testing.T) {
	if ok, _ := engine.MatchPath("re:[bad", "/sensor/temp"); ok {
		t.Error("invalid regex should not match")
	}
}

func TestMatchCoAPPath_WildcardSuffix(t *testing.T) {
	if ok, _ := engine.MatchPath("/sensor/*", "/sensor/temp"); !ok {
		t.Error("wildcard should match suffix")
	}
	if ok, _ := engine.MatchPath("/sensor/*", "/other/temp"); ok {
		t.Error("wildcard prefix should not match wrong prefix")
	}
}
