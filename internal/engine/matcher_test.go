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

func TestHTTPMatch_MidSegmentWildcard(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/regions/*/emails"},
		Response: config.HTTPResponse{Status: 200},
	}}

	// Should match: wildcard replaces exactly one segment.
	if _, ok := match(mocks, "GET", "/regions/fr-par/emails", nil); !ok {
		t.Error("expected match for /regions/fr-par/emails")
	}
	if _, ok := match(mocks, "GET", "/regions/us-east/emails", nil); !ok {
		t.Error("expected match for /regions/us-east/emails")
	}

	// Should not match: different segment count.
	if _, ok := match(mocks, "GET", "/regions/fr-par/other/emails", nil); ok {
		t.Error("expected no match for /regions/fr-par/other/emails (extra segment)")
	}
	if _, ok := match(mocks, "GET", "/regions/emails", nil); ok {
		t.Error("expected no match for /regions/emails (missing segment)")
	}
}

func TestHTTPMatch_DeepMidSegmentWildcard(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/*/users/*/profile"},
		Response: config.HTTPResponse{Status: 200},
	}}

	if _, ok := match(mocks, "GET", "/api/v1/users/42/profile", nil); !ok {
		t.Error("expected match for /api/v1/users/42/profile")
	}
	if _, ok := match(mocks, "GET", "/api/v2/users/99/profile", nil); !ok {
		t.Error("expected match for /api/v2/users/99/profile")
	}
	if _, ok := match(mocks, "GET", "/api/v1/users/profile", nil); ok {
		t.Error("expected no match: segment count mismatch")
	}
}

func TestHTTPMatch_NamedWildcard(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/regions/{region}/emails"},
		Response: config.HTTPResponse{Status: 200},
	}}

	result, ok := match(mocks, "GET", "/regions/fr-par/emails", nil)
	if !ok {
		t.Fatal("expected match for named wildcard path")
	}
	if result.PathParams["region"] != "fr-par" {
		t.Errorf("expected PathParams[region]=fr-par, got %q", result.PathParams["region"])
	}

	result2, ok2 := match(mocks, "GET", "/regions/nl-ams/emails", nil)
	if !ok2 {
		t.Fatal("expected match for nl-ams")
	}
	if result2.PathParams["region"] != "nl-ams" {
		t.Errorf("expected PathParams[region]=nl-ams, got %q", result2.PathParams["region"])
	}

	// Segment count mismatch should not match.
	if _, ok3 := match(mocks, "GET", "/regions/fr-par/other/emails", nil); ok3 {
		t.Error("expected no match for extra segment")
	}
}

func TestHTTPMatch_MultipleNamedWildcards(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/{version}/users/{id}/profile"},
		Response: config.HTTPResponse{Status: 200},
	}}

	result, ok := match(mocks, "GET", "/api/v2/users/42/profile", nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.PathParams["version"] != "v2" {
		t.Errorf("expected version=v2, got %q", result.PathParams["version"])
	}
	if result.PathParams["id"] != "42" {
		t.Errorf("expected id=42, got %q", result.PathParams["id"])
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

// ---------------------------------------------------------------------------
// Header matching
// ---------------------------------------------------------------------------

func TestHTTPMatch_HeaderMatch(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "auth",
		Request:  config.HTTPRequest{Method: "GET", Path: "/secure", Headers: map[string]string{"X-Auth": "secret"}},
		Response: config.HTTPResponse{Status: 200},
	}}

	// Matching header value → should match.
	_, ok := engine.HTTPMatch(mocks, "GET", "/secure", nil, map[string]string{"X-Auth": "secret"}, "", nil)
	if !ok {
		t.Fatal("expected match with correct header")
	}

	// Wrong header value → should not match.
	_, ok2 := engine.HTTPMatch(mocks, "GET", "/secure", nil, map[string]string{"X-Auth": "wrong"}, "", nil)
	if ok2 {
		t.Fatal("expected no match with wrong header value")
	}

	// Missing header → should not match.
	_, ok3 := engine.HTTPMatch(mocks, "GET", "/secure", nil, nil, "", nil)
	if ok3 {
		t.Fatal("expected no match when required header is absent")
	}
}

// ---------------------------------------------------------------------------
// Body pattern matching
// ---------------------------------------------------------------------------

func TestHTTPMatch_BodyPattern(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "subscribe",
		Request:  config.HTTPRequest{Method: "POST", Path: "/events", Body: "subscribe"},
		Response: config.HTTPResponse{Status: 200},
	}}

	// Body contains the pattern → match.
	_, ok := engine.HTTPMatch(mocks, "POST", "/events", nil, nil, "please subscribe now", nil)
	if !ok {
		t.Fatal("expected match when body contains pattern")
	}

	// Body does not contain the pattern → no match.
	_, ok2 := engine.HTTPMatch(mocks, "POST", "/events", nil, nil, "cancel registration", nil)
	if ok2 {
		t.Fatal("expected no match when body does not contain pattern")
	}
}

func TestHTTPMatch_BodyPattern_Regex(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/data", Body: `re:\d{4}`},
		Response: config.HTTPResponse{Status: 200},
	}}

	_, ok := engine.HTTPMatch(mocks, "POST", "/data", nil, nil, "code 1234 issued", nil)
	if !ok {
		t.Fatal("expected regex body match")
	}

	_, ok2 := engine.HTTPMatch(mocks, "POST", "/data", nil, nil, "no digits here", nil)
	if ok2 {
		t.Fatal("expected no regex body match")
	}
}

// ---------------------------------------------------------------------------
// matchBodyJSON edge cases
// ---------------------------------------------------------------------------

func TestHTTPMatch_BodyJSON_EmptyBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyJSON: map[string]string{"key": "val"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, "", nil)
	if ok {
		t.Fatal("expected no match for empty body with body_json requirement")
	}
}

func TestHTTPMatch_BodyJSON_InvalidJSON(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyJSON: map[string]string{"key": "val"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, "not json at all", nil)
	if ok {
		t.Fatal("expected no match for invalid JSON body")
	}
}

func TestHTTPMatch_BodyJSON_MissingNestedKey(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyJSON: map[string]string{"user.role": "admin"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	// Flat body without nested structure.
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, `{"user":"alice"}`, nil)
	if ok {
		t.Fatal("expected no match when nested key path does not exist")
	}
}

func TestHTTPMatch_BodyJSON_NumericValue(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyJSON: map[string]string{"count": "42"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, `{"count":42}`, nil)
	if !ok {
		t.Fatal("expected match for numeric JSON value compared as string")
	}
}

func TestHTTPMatch_BodyJSON_BoolValue(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "POST", Path: "/x", BodyJSON: map[string]string{"active": "true"}},
		Response: config.HTTPResponse{Status: 200},
	}}
	_, ok := engine.HTTPMatch(mocks, "POST", "/x", nil, nil, `{"active":true}`, nil)
	if !ok {
		t.Fatal("expected match for bool JSON value compared as string")
	}
}

// ---------------------------------------------------------------------------
// Default status 200 when response.status is 0
// ---------------------------------------------------------------------------

func TestHTTPMatch_DefaultStatus200(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/no-status"},
		Response: config.HTTPResponse{}, // status is 0 → should default to 200
	}}
	result, ok := engine.HTTPMatch(mocks, "GET", "/no-status", nil, nil, "", nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.Status != 200 {
		t.Errorf("expected default status 200, got %d", result.Status)
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestRender_WithTemplate(t *testing.T) {
	out := engine.Render(`{"id":"{{uuid}}"}`, engine.RequestContext{})
	if out == `{"id":"{{uuid}}"}` {
		t.Error("Render: template was not rendered")
	}
	if len(out) < 10 {
		t.Errorf("Render: output looks too short: %q", out)
	}
}

func TestRender_NoTemplate(t *testing.T) {
	in := `{"static":"value"}`
	out := engine.Render(in, engine.RequestContext{})
	if out != in {
		t.Errorf("Render: plain string modified unexpectedly, got %q", out)
	}
}

func TestRender_InvalidTemplate_ReturnsOriginal(t *testing.T) {
	in := `{{invalid template syntax`
	out := engine.Render(in, engine.RequestContext{})
	// On template error Render returns the original string unchanged.
	if out != in {
		t.Errorf("Render: expected original on error, got %q", out)
	}
}

func TestRender_QueryParam(t *testing.T) {
	out := engine.Render(`hello {{.query.name}}`, engine.RequestContext{Query: map[string]string{"name": "world"}})
	if out != "hello world" {
		t.Errorf("Render: unexpected output %q", out)
	}
}

// ---------------------------------------------------------------------------
// {name} brace syntax for named path params
// ---------------------------------------------------------------------------

func TestHTTPMatch_BraceNamedParam(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/regions/{region}/emails"},
		Response: config.HTTPResponse{Status: 200},
	}}

	result, ok := match(mocks, "GET", "/regions/fr-par/emails", nil)
	if !ok {
		t.Fatal("expected match for brace-style named param")
	}
	if result.PathParams["region"] != "fr-par" {
		t.Errorf("expected region=fr-par, got %q", result.PathParams["region"])
	}
}

func TestHTTPMatch_MultipleBraceParams(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/{version}/users/{id}"},
		Response: config.HTTPResponse{Status: 200},
	}}

	result, ok := match(mocks, "GET", "/api/v2/users/42", nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.PathParams["version"] != "v2" {
		t.Errorf("expected version=v2, got %q", result.PathParams["version"])
	}
	if result.PathParams["id"] != "42" {
		t.Errorf("expected id=42, got %q", result.PathParams["id"])
	}
}

// ---------------------------------------------------------------------------
// path_regex field
// ---------------------------------------------------------------------------

func TestHTTPMatch_PathRegex(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", PathRegex: `^/regions/[a-z-]+/emails$`},
		Response: config.HTTPResponse{Status: 200},
	}}

	if _, ok := match(mocks, "GET", "/regions/fr-par/emails", nil); !ok {
		t.Error("expected match for path_regex")
	}
	if _, ok := match(mocks, "GET", "/regions/UPPER/emails", nil); ok {
		t.Error("expected no match: uppercase region")
	}
	if _, ok := match(mocks, "GET", "/regions/fr-par/other", nil); ok {
		t.Error("expected no match: wrong suffix")
	}
}

func TestHTTPMatch_PathRegex_NoMatch(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", PathRegex: `^/users/\d+$`},
		Response: config.HTTPResponse{Status: 200},
	}}

	if _, ok := match(mocks, "GET", "/users/alice", nil); ok {
		t.Fatal("expected no match for non-numeric path")
	}
}

// ---------------------------------------------------------------------------
// Request context templating: {{.request.*}}
// ---------------------------------------------------------------------------

func TestRender_RequestMethod(t *testing.T) {
	out := engine.Render(`{{.request.method}}`, engine.RequestContext{Method: "POST"})
	if out != "POST" {
		t.Errorf("expected POST, got %q", out)
	}
}

func TestRender_RequestPath(t *testing.T) {
	out := engine.Render(`{{.request.path}}`, engine.RequestContext{Path: "/users/42"})
	if out != "/users/42" {
		t.Errorf("expected /users/42, got %q", out)
	}
}

func TestRender_RequestPathParam(t *testing.T) {
	out := engine.Render(`{{.request.params.region}}`, engine.RequestContext{
		PathParams: map[string]string{"region": "fr-par"},
	})
	if out != "fr-par" {
		t.Errorf("expected fr-par, got %q", out)
	}
}

func TestRender_RequestQuery(t *testing.T) {
	ctx := engine.RequestContext{
		Query: map[string]string{"page": "3"},
	}
	got := engine.Render("page={{.request.query.page}}", ctx)
	if got != "page=3" {
		t.Errorf("want page=3, got %q", got)
	}
}

func TestRender_RequestHeader(t *testing.T) {
	ctx := engine.RequestContext{
		Headers: map[string]string{"X-Token": "secret-123"},
	}
	got := engine.Render(`token={{index .request.headers "X-Token"}}`, ctx)
	if got != "token=secret-123" {
		t.Errorf("want token=secret-123, got %q", got)
	}
}

func TestRender_RequestBodyField(t *testing.T) {
	out := engine.Render(`{{.request.body.project_id}}`, engine.RequestContext{
		Body: `{"project_id":"proj-abc"}`,
	})
	if out != "proj-abc" {
		t.Errorf("expected proj-abc, got %q", out)
	}
}

func TestRender_RequestBodyField_NonJSON(t *testing.T) {
	// Non-JSON body falls back to empty map; missing key renders as "<no value>".
	out := engine.Render(`{{.request.body.x}}`, engine.RequestContext{Body: "not-json"})
	if out != "<no value>" {
		t.Errorf("expected <no value> for non-JSON body missing key, got %q", out)
	}
}

func TestHTTPMatch_ResponseTemplateWithRequestBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "create",
		Request:  config.HTTPRequest{Method: "POST", Path: "/emails"},
		Response: config.HTTPResponse{Status: 201, Body: `{"id":"{{.request.body.project_id}}"}`},
	}}

	result, ok := engine.HTTPMatch(mocks, "POST", "/emails", nil, nil, `{"project_id":"proj-xyz"}`, nil)
	if !ok {
		t.Fatal("expected match")
	}
	if result.Body != `{"id":"proj-xyz"}` {
		t.Errorf("unexpected body: %q", result.Body)
	}
}

// ---------------------------------------------------------------------------
// Header pattern matching
// ---------------------------------------------------------------------------

func TestHTTPMatch_HeaderPatternRegex(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method:  "GET",
			Path:    "/api",
			Headers: map[string]string{"Authorization": "re:^Bearer .+"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"Authorization": "Bearer mytoken"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/api", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected match when Authorization header matches regex")
	}

	noToken := map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}
	_, ok = engine.HTTPMatch(mocks, "GET", "/api", nil, noToken, "", nil)
	if ok {
		t.Fatal("expected no match for non-Bearer Authorization")
	}
}

func TestHTTPMatch_HeaderWildcard(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method:  "GET",
			Path:    "/api",
			Headers: map[string]string{"X-Custom": "*"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	_, ok := engine.HTTPMatch(mocks, "GET", "/api", nil, map[string]string{"X-Custom": "anything"}, "", nil)
	if !ok {
		t.Fatal("expected wildcard header to match any value")
	}
}

func TestHTTPMatch_HeaderCaseInsensitiveKey(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method:  "GET",
			Path:    "/api",
			Headers: map[string]string{"content-type": "application/json"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	// Incoming headers use canonical form (as net/http does).
	hdrs := map[string]string{"Content-Type": "application/json"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/api", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected case-insensitive header key match")
	}
}

// ---------------------------------------------------------------------------
// Auth matching — bearer
// ---------------------------------------------------------------------------

func TestHTTPMatch_BearerAuth_ValidToken(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/secure",
			Auth:   &config.HTTPAuth{Type: "bearer", Token: "secret"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"Authorization": "Bearer secret"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/secure", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected match with valid bearer token")
	}
}

func TestHTTPMatch_BearerAuth_WrongToken(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/secure",
			Auth:   &config.HTTPAuth{Type: "bearer", Token: "secret"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"Authorization": "Bearer wrong"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/secure", nil, hdrs, "", nil)
	if ok {
		t.Fatal("expected no match with wrong bearer token")
	}
}

func TestHTTPMatch_BearerAuth_MissingHeader(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/secure",
			Auth:   &config.HTTPAuth{Type: "bearer", Token: "secret"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	_, ok := engine.HTTPMatch(mocks, "GET", "/secure", nil, nil, "", nil)
	if ok {
		t.Fatal("expected no match when Authorization header is absent")
	}
}

func TestHTTPMatch_BearerAuth_WildcardToken(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/secure",
			Auth:   &config.HTTPAuth{Type: "bearer", Token: "*"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"Authorization": "Bearer anyvalue"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/secure", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected wildcard token to match any bearer value")
	}
}

func TestHTTPMatch_BearerAuth_RegexToken(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/secure",
			Auth:   &config.HTTPAuth{Type: "bearer", Token: "re:^ey[A-Za-z0-9]"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"Authorization": "Bearer eyJWT"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/secure", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected regex token match")
	}

	hdrs2 := map[string]string{"Authorization": "Bearer notajwt"}
	_, ok = engine.HTTPMatch(mocks, "GET", "/secure", nil, hdrs2, "", nil)
	if ok {
		t.Fatal("expected no match for non-matching token regex")
	}
}

// ---------------------------------------------------------------------------
// Auth matching — basic
// ---------------------------------------------------------------------------

func TestHTTPMatch_BasicAuth_Valid(t *testing.T) {
	// "user:pass" in base64 = "dXNlcjpwYXNz"
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/admin",
			Auth:   &config.HTTPAuth{Type: "basic", Username: "user", Password: "pass"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/admin", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected match with valid basic credentials")
	}
}

func TestHTTPMatch_BasicAuth_WrongPassword(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/admin",
			Auth:   &config.HTTPAuth{Type: "basic", Username: "user", Password: "pass"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	// "user:wrong" in base64 = "dXNlcjp3cm9uZw=="
	hdrs := map[string]string{"Authorization": "Basic dXNlcjp3cm9uZw=="}
	_, ok := engine.HTTPMatch(mocks, "GET", "/admin", nil, hdrs, "", nil)
	if ok {
		t.Fatal("expected no match with wrong password")
	}
}

// ---------------------------------------------------------------------------
// Auth matching — api_key
// ---------------------------------------------------------------------------

func TestHTTPMatch_APIKey_Header(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/weather",
			Auth:   &config.HTTPAuth{Type: "api_key", Header: "X-API-Key", Value: "key-abc"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"X-API-Key": "key-abc"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/weather", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected match with valid API key header")
	}

	hdrs2 := map[string]string{"X-API-Key": "wrong"}
	_, ok = engine.HTTPMatch(mocks, "GET", "/weather", nil, hdrs2, "", nil)
	if ok {
		t.Fatal("expected no match with wrong API key")
	}
}

func TestHTTPMatch_APIKey_Query(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/data",
			Auth:   &config.HTTPAuth{Type: "api_key", Query: "apikey", Value: "key-xyz"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	query := map[string]string{"apikey": "key-xyz"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/data", query, nil, "", nil)
	if !ok {
		t.Fatal("expected match with valid API key query param")
	}
}

// ---------------------------------------------------------------------------
// Auth matching — digest
// ---------------------------------------------------------------------------

func TestHTTPMatch_DigestAuth_Present(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/digest",
			Auth:   &config.HTTPAuth{Type: "digest"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	hdrs := map[string]string{"Authorization": `Digest username="alice", realm="mockly", nonce="abc"`}
	_, ok := engine.HTTPMatch(mocks, "GET", "/digest", nil, hdrs, "", nil)
	if !ok {
		t.Fatal("expected match when Digest Authorization header is present")
	}
}

func TestHTTPMatch_DigestAuth_Missing(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/digest",
			Auth:   &config.HTTPAuth{Type: "digest"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	_, ok := engine.HTTPMatch(mocks, "GET", "/digest", nil, nil, "", nil)
	if ok {
		t.Fatal("expected no match when Digest Authorization header is absent")
	}
}

// ---------------------------------------------------------------------------
// Auth is optional — mock without auth still matches normally
// ---------------------------------------------------------------------------

func TestHTTPMatch_NoAuth_StillMatches(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m",
		Request:  config.HTTPRequest{Method: "GET", Path: "/open"},
		Response: config.HTTPResponse{Status: 200},
	}}

	_, ok := engine.HTTPMatch(mocks, "GET", "/open", nil, nil, "", nil)
	if !ok {
		t.Fatal("expected match for mock without auth config")
	}
}

// ---------------------------------------------------------------------------
// Fallback pattern: auth mock + open fallback mock
// ---------------------------------------------------------------------------

func TestHTTPMatch_AuthFallback(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID: "authenticated",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/api/users",
				Auth:   &config.HTTPAuth{Type: "bearer", Token: "valid"},
			},
			Response: config.HTTPResponse{Status: 200, Body: `{"users":[]}`},
		},
		{
			ID:       "unauthenticated",
			Request:  config.HTTPRequest{Method: "GET", Path: "/api/users"},
			Response: config.HTTPResponse{Status: 401, Body: `{"error":"unauthorized"}`},
		},
	}

	// With valid token → first mock matches.
	result, ok := engine.HTTPMatch(mocks, "GET", "/api/users", nil,
		map[string]string{"Authorization": "Bearer valid"}, "", nil)
	if !ok || result.Status != 200 {
		t.Fatalf("expected 200 with valid token, got ok=%v status=%d", ok, result.Status)
	}

	// Without token → first mock skipped, fallback matches.
	result, ok = engine.HTTPMatch(mocks, "GET", "/api/users", nil, nil, "", nil)
	if !ok || result.Status != 401 {
		t.Fatalf("expected 401 fallback, got ok=%v status=%d", ok, result.Status)
	}
}

func TestHTTPMatch_BearerAuth_ExactNotSubstring(t *testing.T) {
	// "secret" must NOT match a token that merely contains "secret" as a substring.
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/secure",
			Auth:   &config.HTTPAuth{Type: "bearer", Token: "secret"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	superstring := map[string]string{"Authorization": "Bearer notsecrettoken"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/secure", nil, superstring, "", nil)
	if ok {
		t.Fatal("bearer match must be exact, not substring")
	}

	exact := map[string]string{"Authorization": "Bearer secret"}
	_, ok = engine.HTTPMatch(mocks, "GET", "/secure", nil, exact, "", nil)
	if !ok {
		t.Fatal("exact bearer token should match")
	}
}

func TestHTTPMatch_APIKey_ExactNotSubstring(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID: "m",
		Request: config.HTTPRequest{
			Method: "GET",
			Path:   "/data",
			Auth:   &config.HTTPAuth{Type: "api_key", Header: "X-API-Key", Value: "key-abc"},
		},
		Response: config.HTTPResponse{Status: 200},
	}}

	superstring := map[string]string{"X-API-Key": "bad-key-abc-extra"}
	_, ok := engine.HTTPMatch(mocks, "GET", "/data", nil, superstring, "", nil)
	if ok {
		t.Fatal("api_key match must be exact, not substring")
	}

	exact := map[string]string{"X-API-Key": "key-abc"}
	_, ok = engine.HTTPMatch(mocks, "GET", "/data", nil, exact, "", nil)
	if !ok {
		t.Fatal("exact api key should match")
	}
}
