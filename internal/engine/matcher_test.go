package engine_test

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/state"
)

// helper to call HTTPMatch with nil query/headers for brevity in existing tests.
func match(mocks []config.HTTPMock, method, path string, store *state.Store) (engine.MatchResult, bool) {
	return engine.HTTPMatch(mocks, method, path, nil, nil, "", store)
}

func TestHTTPMatch_ExactPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/users"},
		Response: config.HTTPResponse{Status: 200, Body: `[]`},
	}}

	result, ok := match(mocks, "GET", "/api/users", nil)
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
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/users"},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := match(mocks, "POST", "/api/users", nil)
	if ok {
		t.Fatal("expected no match for wrong method")
	}
}

func TestHTTPMatch_WildcardPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/*"},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := match(mocks, "GET", "/api/users/123", nil)
	if !ok {
		t.Fatal("expected match for wildcard path")
	}
}

func TestHTTPMatch_RegexPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: `re:^/api/users/\d+$`},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := match(mocks, "GET", "/api/users/42", nil)
	if !ok {
		t.Fatal("expected regex match")
	}
	_, no := match(mocks, "GET", "/api/users/abc", nil)
	if no {
		t.Fatal("expected no regex match for non-numeric id")
	}
}

func TestHTTPMatch_StateCondition(t *testing.T) {
	store := state.New()
	mocks := []config.HTTPMock{{
		ID:       "m-auth",
		Request:  config.HTTPRequest{Method: "GET", Path: "/me"},
		Response: config.HTTPResponse{Status: 200, Body: `{"user":"alice"}`},
		State:    &config.StateCondition{Key: "authenticated", Value: "true"},
	}}

	_, ok := match(mocks, "GET", "/me", store)
	if ok {
		t.Fatal("expected no match when state condition not met")
	}

	store.Set("authenticated", "true")

	result, ok := match(mocks, "GET", "/me", store)
	if !ok {
		t.Fatal("expected match after state set")
	}
	if result.MockID != "m-auth" {
		t.Fatalf("unexpected mock ID %q", result.MockID)
	}
}

func TestHTTPMatch_Template(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/time"},
		Response: config.HTTPResponse{Status: 200, Body: `{"time":"{{now}}"}`},
	}}
	result, ok := match(mocks, "GET", "/time", nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.Body == `{"time":"{{now}}"}` {
		t.Fatal("template was not rendered")
	}
}

func TestHTTPMatch_QueryParams(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID:       "admin",
			Request:  config.HTTPRequest{Method: "GET", Path: "/users", Query: map[string]string{"role": "admin"}},
			Response: config.HTTPResponse{Status: 200, Body: `{"role":"admin"}`},
		},
		{
			ID:       "any",
			Request:  config.HTTPRequest{Method: "GET", Path: "/users"},
			Response: config.HTTPResponse{Status: 200, Body: `{"role":"any"}`},
		},
	}

	// Should match the admin mock specifically
	q := map[string]string{"role": "admin"}
	res, ok := engine.HTTPMatch(mocks, "GET", "/users", q, nil, "", nil)
	if !ok || res.MockID != "admin" {
		t.Fatalf("expected admin mock, got %q ok=%v", res.MockID, ok)
	}

	// No role → falls through to the wildcard mock
	res2, ok2 := engine.HTTPMatch(mocks, "GET", "/users", nil, nil, "", nil)
	if !ok2 || res2.MockID != "any" {
		t.Fatalf("expected any mock, got %q ok=%v", res2.MockID, ok2)
	}

	// Wildcard value
	mocks2 := []config.HTTPMock{{
		ID:       "page",
		Request:  config.HTTPRequest{Method: "GET", Path: "/items", Query: map[string]string{"page": "*"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok3 := engine.HTTPMatch(mocks2, "GET", "/items", map[string]string{"page": "3"}, nil, "", nil)
	if !ok3 {
		t.Fatal("expected wildcard query match")
	}
}

func TestHTTPMatch_BodyJSON(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "pay-gbp",
		Request:  config.HTTPRequest{Method: "POST", Path: "/pay", BodyJSON: map[string]string{"currency": "GBP", "user.role": "admin"}},
		Response: config.HTTPResponse{Status: 200},
	}}

	// Matching body
	body := `{"currency":"GBP","user":{"role":"admin"}}`
	res, ok := engine.HTTPMatch(mocks, "POST", "/pay", nil, nil, body, nil)
	if !ok || res.MockID != "pay-gbp" {
		t.Fatalf("expected pay-gbp match, got %q ok=%v", res.MockID, ok)
	}

	// Wrong currency
	body2 := `{"currency":"USD","user":{"role":"admin"}}`
	_, ok2 := engine.HTTPMatch(mocks, "POST", "/pay", nil, nil, body2, nil)
	if ok2 {
		t.Fatal("expected no match for wrong currency")
	}

	// Wildcard JSON value
	mocks2 := []config.HTTPMock{{
		ID:       "any-user",
		Request:  config.HTTPRequest{Method: "POST", Path: "/users", BodyJSON: map[string]string{"id": "*"}},
		Response: config.HTTPResponse{Status: 201},
	}}
	_, ok3 := engine.HTTPMatch(mocks2, "POST", "/users", nil, nil, `{"id":42}`, nil)
	if !ok3 {
		t.Fatal("expected wildcard body_json match")
	}
}

func TestHTTPMatch_QueryParamInBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "echo-param",
		Request:  config.HTTPRequest{Method: "GET", Path: "/echo"},
		Response: config.HTTPResponse{Status: 200, Body: `{"param":"{{.query.foo}}"}`},
	}}
	query := map[string]string{"foo": "bar"}
	result, ok := engine.HTTPMatch(mocks, "GET", "/echo", query, nil, "", nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.Body != `{"param":"bar"}` {
		t.Errorf("unexpected body: %q", result.Body)
	}
}

func TestHTTPMatch_QueryParamInResponseHeader(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:      "oauth-authorize",
		Request: config.HTTPRequest{Method: "GET", Path: "/authorize"},
		Response: config.HTTPResponse{
			Status: 302,
			Headers: map[string]string{
				"Location": "{{.query.redirect_uri}}?code=abc&state={{.query.state}}",
			},
		},
	}}
	query := map[string]string{"redirect_uri": "http://app.example.com/cb", "state": "xyz123"}
	result, ok := engine.HTTPMatch(mocks, "GET", "/authorize", query, nil, "", nil)
	if !ok {
		t.Fatal("expected match")
	}
	want := "http://app.example.com/cb?code=abc&state=xyz123"
	if result.Headers["Location"] != want {
		t.Errorf("want Location %q, got %q", want, result.Headers["Location"])
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

func TestHTTPMatch_QueryParams_MissingRequired(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "filtered",
		Request:  config.HTTPRequest{Method: "GET", Path: "/search", Query: map[string]string{"q": "*"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	// Request without the required query param should not match
	_, ok := engine.HTTPMatch(mocks, "GET", "/search", nil, nil, "", nil)
	if ok {
		t.Fatal("expected no match when required query param is absent")
	}
}

func TestHTTPMatch_QueryParams_ExtraParamsIgnored(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/items", Query: map[string]string{"type": "book"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	// Extra params in request beyond what mock requires should still match
	q := map[string]string{"type": "book", "page": "2", "limit": "10"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/items", q, nil, "", nil)
	if !ok {
		t.Fatal("expected match when request has extra query params beyond required ones")
	}
}

// TestHTTPMatch_OAuthAuthorize covers a realistic OAuth 2.0 authorization endpoint:
// the mock requires specific query params and reflects them in the redirect Location header.
func TestHTTPMatch_OAuthAuthorize(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID: "oauth-code-flow",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/oauth/authorize",
				Query: map[string]string{
					"response_type": "code",
					"client_id":     "my-client",
					"redirect_uri":  "*",
					"state":         "*",
				},
			},
			Response: config.HTTPResponse{
				Status: 302,
				Headers: map[string]string{
					"Location": "{{.query.redirect_uri}}?code=testcode&state={{.query.state}}",
				},
			},
		},
	}

	q := map[string]string{
		"response_type": "code",
		"client_id":     "my-client",
		"redirect_uri":  "https://app.example.com/callback",
		"state":         "random-state-42",
	}

	result, ok := engine.HTTPMatch(mocks, "GET", "/oauth/authorize", q, nil, "", nil)
	if !ok {
		t.Fatal("expected OAuth mock to match")
	}
	if result.Status != 302 {
		t.Errorf("want status 302, got %d", result.Status)
	}
	wantLocation := "https://app.example.com/callback?code=testcode&state=random-state-42"
	if result.Headers["Location"] != wantLocation {
		t.Errorf("want Location %q, got %q", wantLocation, result.Headers["Location"])
	}

	// Wrong client_id should not match
	qWrong := map[string]string{
		"response_type": "code",
		"client_id":     "other-client",
		"redirect_uri":  "https://app.example.com/callback",
		"state":         "s",
	}
	_, ok2 := engine.HTTPMatch(mocks, "GET", "/oauth/authorize", qWrong, nil, "", nil)
	if ok2 {
		t.Fatal("expected no match for wrong client_id")
	}
}

