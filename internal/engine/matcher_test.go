package engine_test

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/state"
)

func TestHTTPMatch_ExactPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/users"},
		Response: config.HTTPResponse{Status: 200, Body: `[]`},
	}}

	result, ok := engine.HTTPMatch(mocks, "GET", "/api/users", nil, "", nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.Status != 200 {
		t.Fatalf("want status 200, got %d", result.Status)
	}
	if result.Body != `[]` {
		t.Fatalf("unexpected body %q", result.Body)
	}
}

func TestHTTPMatch_NoMatch(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:      "m1",
		Request: config.HTTPRequest{Method: "GET", Path: "/api/users"},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/api/users", nil, "", nil)
	if ok {
		t.Fatal("expected no match for wrong method")
	}
}

func TestHTTPMatch_WildcardPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:      "m1",
		Request: config.HTTPRequest{Method: "GET", Path: "/api/*"},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "GET", "/api/users/123", nil, "", nil)
	if !ok {
		t.Fatal("expected match for wildcard path")
	}
}

func TestHTTPMatch_RegexPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:      "m1",
		Request: config.HTTPRequest{Method: "GET", Path: `re:^/api/users/\d+$`},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "GET", "/api/users/42", nil, "", nil)
	if !ok {
		t.Fatal("expected regex match")
	}
	_, no := engine.HTTPMatch(mocks, "GET", "/api/users/abc", nil, "", nil)
	if no {
		t.Fatal("expected no regex match for non-numeric id")
	}
}

func TestHTTPMatch_StateCondition(t *testing.T) {
	store := state.New()
	mocks := []config.HTTPMock{{
		ID:      "m-auth",
		Request: config.HTTPRequest{Method: "GET", Path: "/me"},
		Response: config.HTTPResponse{Status: 200, Body: `{"user":"alice"}`},
		State:   &config.StateCondition{Key: "authenticated", Value: "true"},
	}}

	// Should not match when state not set
	_, ok := engine.HTTPMatch(mocks, "GET", "/me", nil, "", store)
	if ok {
		t.Fatal("expected no match when state condition not met")
	}

	store.Set("authenticated", "true")

	// Should match now
	result, ok := engine.HTTPMatch(mocks, "GET", "/me", nil, "", store)
	if !ok {
		t.Fatal("expected match after state set")
	}
	if result.MockID != "m-auth" {
		t.Fatalf("unexpected mock ID %q", result.MockID)
	}
}

func TestHTTPMatch_Template(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:      "m1",
		Request: config.HTTPRequest{Method: "GET", Path: "/time"},
		Response: config.HTTPResponse{Status: 200, Body: `{"time":"{{now}}"}`},
	}}
	result, ok := engine.HTTPMatch(mocks, "GET", "/time", nil, "", nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.Body == `{"time":"{{now}}"}` {
		t.Fatal("template was not rendered")
	}
}

func TestWSMatch(t *testing.T) {
	rules := []config.WebSocketRule{
		{Match: "ping", Respond: "pong"},
		{Match: "re:^bye", Close: true},
	}

	r, ok := engine.WSMatch(rules, "ping")
	if !ok || r.Respond != "pong" {
		t.Fatalf("expected pong rule, got %+v ok=%v", r, ok)
	}

	r2, ok2 := engine.WSMatch(rules, "bye now")
	if !ok2 || !r2.Close {
		t.Fatalf("expected close rule, got %+v ok=%v", r2, ok2)
	}

	_, ok3 := engine.WSMatch(rules, "hello")
	if ok3 {
		t.Fatal("expected no match")
	}
}
