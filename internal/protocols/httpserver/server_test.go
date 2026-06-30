package httpserver_test

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/testutil"
)

// startTestServer starts an HTTP mock server on a free port and returns its base URL.
// The server is stopped automatically when the test ends.
func startTestServer(t *testing.T, mocks []config.HTTPMock, sc *scenarios.Store) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.HTTPConfig{
		Enabled: true,
		Port:    port,
		Mocks:   mocks,
	}
	if sc == nil {
		sc = scenarios.New(nil)
	}
	store := state.New()
	log := logger.New(100)
	srv := httpserver.New(cfg, store, sc, log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base, 2*time.Second)
	return base
}

func TestHTTPServer_BasicGET(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "get-users",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/users"},
		Response: config.HTTPResponse{Status: 200, Body: `[{"id":1}]`, Headers: map[string]string{"Content-Type": "application/json"}},
	}}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Get(base + "/api/users")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(body) != `[{"id":1}]` {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestHTTPServer_NoMatchReturns404(t *testing.T) {
	base := startTestServer(t, nil, nil)

	resp, err := http.Get(base + "/nonexistent")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_WildcardPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "m",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/*"},
		Response: config.HTTPResponse{Status: 200, Body: "ok"},
	}}
	base := startTestServer(t, mocks, nil)

	for _, path := range []string{"/api/users", "/api/users/123", "/api/orders"} {
		resp, _ := http.Get(base + path)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("path %s: want 200, got %d", path, resp.StatusCode)
		}
	}
}

func TestHTTPServer_MidSegmentWildcardPath(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "regions",
		Request:  config.HTTPRequest{Method: "GET", Path: "/transactional-email/v1alpha1/regions/*/emails"},
		Response: config.HTTPResponse{Status: 200, Body: `{"emails":[]}`},
	}}
	base := startTestServer(t, mocks, nil)

	// Wildcard segment matches any single region name.
	for _, region := range []string{"fr-par", "nl-ams", "pl-waw"} {
		path := "/transactional-email/v1alpha1/regions/" + region + "/emails"
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("path %s: want 200, got %d", path, resp.StatusCode)
		}
	}

	// Extra segment should not match.
	resp, _ := http.Get(base + "/transactional-email/v1alpha1/regions/fr-par/extra/emails")
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("extra segment: want 404, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_NamedWildcard_LogsPathParams(t *testing.T) {
	log := logger.New(100)
	mocks := []config.HTTPMock{{
		ID:       "regions",
		Request:  config.HTTPRequest{Method: "GET", Path: "/regions/{region}/emails"},
		Response: config.HTTPResponse{Status: 200, Body: `{"emails":[]}`},
	}}

	cfg := &config.HTTPConfig{Enabled: true}
	store := state.New()
	sc := scenarios.New(nil)
	srv := httpserver.New(cfg, store, sc, log)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	cfg.Port = port
	srv.SetMocks(mocks)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base, 2*time.Second)

	resp, err := http.Get(base + "/regions/fr-par/emails")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	entries := log.Entries()
	if len(entries) == 0 {
		t.Fatal("expected log entry")
	}
	params := entries[len(entries)-1].PathParams
	if params["region"] != "fr-par" {
		t.Errorf("expected PathParams[region]=fr-par, got %q", params["region"])
	}
}

func TestHTTPServer_POST_WithBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "create",
		Request:  config.HTTPRequest{Method: "POST", Path: "/users"},
		Response: config.HTTPResponse{Status: 201, Body: `{"id":99}`, Headers: map[string]string{"Content-Type": "application/json"}},
	}}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Post(base+"/users", "application/json", strings.NewReader(`{"name":"Bob"}`))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("want 201, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_ScenarioPatch_OverridesStatus(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "token",
		Request:  config.HTTPRequest{Method: "POST", Path: "/token"},
		Response: config.HTTPResponse{Status: 200, Body: `{"access_token":"abc"}`},
	}}
	sc := scenarios.New([]config.Scenario{{
		ID: "auth-down",
		Patches: []config.MockPatch{
			{MockID: "token", Status: 503, Body: `{"error":"down"}`},
		},
	}})
	base := startTestServer(t, mocks, sc)

	// Without scenario: 200
	r1, err := http.Post(base+"/token", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /token: %v", err)
	}
	_ = r1.Body.Close()
	if r1.StatusCode != 200 {
		t.Errorf("before activation: want 200, got %d", r1.StatusCode)
	}

	// Activate scenario
	sc.Activate("auth-down")

	// With scenario: 503
	r2, err := http.Post(base+"/token", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /token (after activate): %v", err)
	}
	defer r2.Body.Close() //nolint:errcheck
	if r2.StatusCode != 503 {
		t.Errorf("after activation: want 503, got %d", r2.StatusCode)
	}
	body, err := io.ReadAll(r2.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if !strings.Contains(string(body), "down") {
		t.Errorf("expected 'down' in body, got %q", body)
	}
}

func TestHTTPServer_ScenarioPatch_Disabled(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "resource",
		Request:  config.HTTPRequest{Method: "GET", Path: "/resource"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New([]config.Scenario{{
		ID:      "hide",
		Patches: []config.MockPatch{{MockID: "resource", Disabled: true}},
	}})
	base := startTestServer(t, mocks, sc)

	sc.Activate("hide")
	resp, _ := http.Get(base + "/resource")
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("disabled mock should return 404, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_GlobalFault_StatusOverride(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503, ErrorRate: 0}})

	resp, _ := http.Get(base + "/ok")
	_ = resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("global fault should override to 503, got %d", resp.StatusCode)
	}

	sc.ClearDirectFaults()
	resp2, _ := http.Get(base + "/ok")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("after clear fault: want 200, got %d", resp2.StatusCode)
	}
}

func TestHTTPServer_GlobalFault_Delay(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "fast",
		Request:  config.HTTPRequest{Method: "GET", Path: "/fast"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	// No fault: should be fast.
	t0 := time.Now()
	http.Get(base + "/fast") //nolint:errcheck
	baseline := time.Since(t0)

	// Add 100ms fault delay.
	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Delay: config.Duration{Duration: 100 * time.Millisecond}}})
	t1 := time.Now()
	http.Get(base + "/fast") //nolint:errcheck
	withFault := time.Since(t1)
	sc.ClearDirectFaults()

	if withFault < 80*time.Millisecond {
		t.Errorf("expected ≥80ms with fault, got %v (baseline %v)", withFault, baseline)
	}
}

func TestHTTPServer_TemplateResponse(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "time",
		Request:  config.HTTPRequest{Method: "GET", Path: "/time"},
		Response: config.HTTPResponse{Status: 200, Body: `{"time":"{{now}}"}`},
	}}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Get(base + "/time")
	if err != nil {
		t.Fatalf("GET /time: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}

	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("response not valid JSON: %v — body: %s", err, body)
	}
	if m["time"] == "{{now}}" {
		t.Error("template was not rendered")
	}
	if m["time"] == "" {
		t.Error("time value should not be empty")
	}
}

func TestHTTPServer_PathRegex_Field(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "user-by-id",
		Request:  config.HTTPRequest{Method: "GET", PathRegex: `^/users/\d+$`},
		Response: config.HTTPResponse{Status: 200, Body: "ok"},
	}}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Get(base + "/users/42")
	if err != nil {
		t.Fatalf("GET /users/42: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200 for regex match, got %d", resp.StatusCode)
	}

	resp2, err := http.Get(base + "/users/alice")
	if err != nil {
		t.Fatalf("GET /users/alice: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Fatalf("want 404 for regex miss, got %d", resp2.StatusCode)
	}
}

func TestHTTPServer_NamedParam_TemplateInBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:      "user-by-id",
		Request: config.HTTPRequest{Method: "GET", Path: "/users/{id}"},
		Response: config.HTTPResponse{
			Status: 200,
			Body:   `{"user":"{{.request.params.id}}"}`,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Get(base + "/users/42")
	if err != nil {
		t.Fatalf("GET /users/42: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["user"] != "42" {
		t.Errorf("want user=42, got %q", got["user"])
	}
}

func TestHTTPServer_RequestQuery_TemplateInBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "search",
		Request:  config.HTTPRequest{Method: "GET", Path: "/search"},
		Response: config.HTTPResponse{Status: 200, Body: `{{.request.query.filter}}`},
	}}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Get(base + "/search?filter=active")
	if err != nil {
		t.Fatalf("GET /search?filter=active: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(body) != "active" {
		t.Errorf("want active, got %q", body)
	}
}

func TestHTTPServer_RequestHeader_TemplateInBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "trace",
		Request:  config.HTTPRequest{Method: "GET", Path: "/trace"},
		Response: config.HTTPResponse{Status: 200, Body: `{{index .request.headers "X-Trace"}}`},
	}}
	base := startTestServer(t, mocks, nil)

	req, err := http.NewRequest(http.MethodGet, base+"/trace", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Trace", "trace-123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /trace: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(body) != "trace-123" {
		t.Errorf("want trace-123, got %q", body)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func waitForHTTP(t *testing.T, base string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/")
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s did not start within %v", base, timeout)
}

// startTestServerWithServer is like startTestServer but also returns the server
// so tests can inspect call counts etc.
func startTestServerWithServer(t *testing.T, mocks []config.HTTPMock, sc *scenarios.Store) (string, *httpserver.Server) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.HTTPConfig{Enabled: true, Port: port, Mocks: mocks}
	if sc == nil {
		sc = scenarios.New(nil)
	}
	store := state.New()
	log := logger.New(100)
	srv := httpserver.New(cfg, store, sc, log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base, 2*time.Second)
	return base, srv
}

func TestHTTPServer_QueryParams(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID:       "admin",
			Request:  config.HTTPRequest{Method: "GET", Path: "/users", Query: map[string]string{"role": "admin"}},
			Response: config.HTTPResponse{Status: 200, Body: `{"role":"admin"}`},
		},
		{
			ID:       "fallback",
			Request:  config.HTTPRequest{Method: "GET", Path: "/users"},
			Response: config.HTTPResponse{Status: 200, Body: `{"role":"other"}`},
		},
	}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Get(base + "/users?role=admin")
	if err != nil {
		t.Fatalf("GET /users?role=admin: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if !strings.Contains(string(body), `"admin"`) {
		t.Errorf("expected admin response, got %q", body)
	}

	resp2, err := http.Get(base + "/users?role=user")
	if err != nil {
		t.Fatalf("GET /users?role=user: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if !strings.Contains(string(body2), `"other"`) {
		t.Errorf("expected fallback response, got %q", body2)
	}
}

func TestHTTPServer_BodyJSON(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID:       "gbp",
			Request:  config.HTTPRequest{Method: "POST", Path: "/pay", BodyJSON: map[string]string{"currency": "GBP"}},
			Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
		},
		{
			ID:       "other",
			Request:  config.HTTPRequest{Method: "POST", Path: "/pay"},
			Response: config.HTTPResponse{Status: 422, Body: `{"ok":false}`},
		},
	}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Post(base+"/pay", "application/json", strings.NewReader(`{"currency":"GBP","amount":100}`))
	if err != nil {
		t.Fatalf("POST /pay GBP: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for GBP, got %d", resp.StatusCode)
	}

	resp2, err := http.Post(base+"/pay", "application/json", strings.NewReader(`{"currency":"USD","amount":100}`))
	if err != nil {
		t.Fatalf("POST /pay USD: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 422 {
		t.Errorf("expected 422 for USD, got %d", resp2.StatusCode)
	}
}

func TestHTTPServer_Sequence_HoldLast(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "flaky",
		Request:  config.HTTPRequest{Method: "GET", Path: "/data"},
		Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
		Sequence: []config.HTTPResponse{
			{Status: 503, Body: `{"error":"down"}`},
			{Status: 503, Body: `{"error":"down"}`},
			{Status: 200, Body: `{"ok":true}`},
		},
		SequenceExhausted: "hold_last",
	}}
	base := startTestServer(t, mocks, nil)

	codes := []int{503, 503, 200, 200, 200}
	for i, want := range codes {
		resp, _ := http.Get(base + "/data")
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("call %d: want %d, got %d", i+1, want, resp.StatusCode)
		}
	}
}

func TestHTTPServer_Sequence_Loop(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "cycler",
		Request:  config.HTTPRequest{Method: "GET", Path: "/cycle"},
		Response: config.HTTPResponse{Status: 200},
		Sequence: []config.HTTPResponse{
			{Status: 200, Body: `"a"`},
			{Status: 200, Body: `"b"`},
		},
		SequenceExhausted: "loop",
	}}
	base := startTestServer(t, mocks, nil)

	expected := []string{`"a"`, `"b"`, `"a"`, `"b"`}
	for i, want := range expected {
		resp, _ := http.Get(base + "/cycle")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading response body: %v", err)
		}
		_ = resp.Body.Close()
		if strings.TrimSpace(string(body)) != want {
			t.Errorf("call %d: want %q, got %q", i+1, want, body)
		}
	}
}

func TestHTTPServer_PerMockFault(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "fragile",
		Request:  config.HTTPRequest{Method: "GET", Path: "/fragile"},
		Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
		Fault:    &config.MockFault{StatusOverride: 503, ErrorRate: 0},
	}}
	base := startTestServer(t, mocks, nil)

	resp, _ := http.Get(base + "/fragile")
	_ = resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("expected per-mock fault to override to 503, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_CallCount(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "counted",
		Request:  config.HTTPRequest{Method: "GET", Path: "/counted"},
		Response: config.HTTPResponse{Status: 200},
	}}
	base, srv := startTestServerWithServer(t, mocks, nil)

	if n := srv.CallCount("counted"); n != 0 {
		t.Fatalf("expected 0 calls initially, got %d", n)
	}

	for i := 0; i < 3; i++ {
		resp, _ := http.Get(base + "/counted")
		_ = resp.Body.Close()
	}

	if n := srv.CallCount("counted"); n != 3 {
		t.Fatalf("expected 3 calls, got %d", n)
	}

	srv.ResetCallCounts()
	if n := srv.CallCount("counted"); n != 0 {
		t.Fatalf("expected 0 after reset, got %d", n)
	}
}

func TestHTTPServer_QueryParamTemplate_InBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "echo-param",
		Request:  config.HTTPRequest{Method: "GET", Path: "/echo"},
		Response: config.HTTPResponse{Status: 200, Body: `{"param":"{{.query.foo}}"}`},
	}}
	base := startTestServer(t, mocks, nil)

	resp, err := http.Get(base + "/echo?foo=bar")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(body) != `{"param":"bar"}` {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestHTTPServer_QueryParamTemplate_InHeader(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:      "oauth-authorize",
		Request: config.HTTPRequest{Method: "GET", Path: "/oauth2/authorize"},
		Response: config.HTTPResponse{
			Status: 302,
			Headers: map[string]string{
				"Location": "{{.query.redirect_uri}}?code=abc&state={{.query.state}}",
			},
		},
	}}
	base := startTestServer(t, mocks, nil)

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(base + "/oauth2/authorize?redirect_uri=http://app.example.com/cb&state=xyzabc")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != 302 {
		t.Errorf("want 302, got %d", resp.StatusCode)
	}
	want := "http://app.example.com/cb?code=abc&state=xyzabc"
	if got := resp.Header.Get("Location"); got != want {
		t.Errorf("want Location %q, got %q", want, got)
	}
}

func TestHTTPServer_OAuthAuthorize_QueryParamMatching(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID: "oauth-code",
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
		{
			ID:       "oauth-bad-client",
			Request:  config.HTTPRequest{Method: "GET", Path: "/oauth/authorize"},
			Response: config.HTTPResponse{Status: 400, Body: `{"error":"unauthorized_client"}`},
		},
	}
	base := startTestServer(t, mocks, nil)

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// Valid request — should redirect with code and reflected state
	resp, err := client.Get(base + "/oauth/authorize?response_type=code&client_id=my-client&redirect_uri=https://app.example.com/cb&state=s42")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 302 {
		t.Errorf("want 302, got %d", resp.StatusCode)
	}
	wantLoc := "https://app.example.com/cb?code=testcode&state=s42"
	if got := resp.Header.Get("Location"); got != wantLoc {
		t.Errorf("want Location %q, got %q", wantLoc, got)
	}

	// Wrong client_id — should fall through to the 400 mock
	resp2, err := client.Get(base + "/oauth/authorize?response_type=code&client_id=evil&redirect_uri=https://evil.example.com&state=x")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 400 {
		t.Errorf("want 400 for bad client, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// TLS tests
// ---------------------------------------------------------------------------

func startTLSTestServer(t *testing.T, mocks []config.HTTPMock) (string, *http.Client) {
	t.Helper()

	dir := t.TempDir()
	certFile := dir + "/cert.pem"
	keyFile := dir + "/key.pem"
	if err := testutil.WriteSelfSignedCert(certFile, keyFile); err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.HTTPConfig{
		Enabled: true,
		Port:    port,
		TLS:     &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile},
		Mocks:   mocks,
	}
	store := state.New()
	sc := scenarios.New(nil)
	log := logger.New(100)
	srv := httpserver.New(cfg, store, sc, log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	base := fmt.Sprintf("https://127.0.0.1:%d", port)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return base, client
}

func TestHTTPServer_TLS(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ping",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ping"},
		Response: config.HTTPResponse{Status: 200, Body: "pong"},
	}}
	base, client := startTLSTestServer(t, mocks)

	resp, err := client.Get(base + "/ping")
	if err != nil {
		t.Fatalf("GET /ping over TLS: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "pong" {
		t.Errorf("want 'pong', got %q", body)
	}
}

// ---------------------------------------------------------------------------
// Concurrency / race-detector tests
// ---------------------------------------------------------------------------

func TestHTTPServer_SetMocks_ConcurrentAccess(t *testing.T) {
	base, srv := startTestServerWithServer(t, []config.HTTPMock{{
		ID:       "m1",
		Request:  config.HTTPRequest{Method: "GET", Path: "/"},
		Response: config.HTTPResponse{Status: 200, Body: "ok"},
	}}, nil)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				srv.SetMocks([]config.HTTPMock{{
					ID:       "m",
					Request:  config.HTTPRequest{Method: "GET", Path: "/"},
					Response: config.HTTPResponse{Status: 200, Body: "updated"},
				}})
			}
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				resp, err := http.Get(base + "/")
				if err == nil {
					_ = resp.Body.Close()
				}
			}
		}()
	}
	wg.Wait()
}

func TestHTTPServer_HTTPFault_CustomStatus(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)
	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: http.StatusTeapot, ErrorRate: 0}})
	resp, _ := http.Get(base + "/ok")
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("HTTP fault should override to 418, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Latency-only fault — should NOT alter the response status/body
// ---------------------------------------------------------------------------

func TestHTTPServer_GlobalFault_DelayOnly_NoStatusOverride(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	// Set a delay-only fault (no status, no body).
	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Delay: config.Duration{Duration: 50 * time.Millisecond},
	}})
	t0 := time.Now()
	resp, _ := http.Get(base + "/ok")
	elapsed := time.Since(t0)
	defer resp.Body.Close() //nolint:errcheck

	// Delay should be applied.
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected latency ≥40ms, got %v", elapsed)
	}
	// Status must stay 200 — NOT 503.
	if resp.StatusCode != 200 {
		t.Errorf("delay-only fault must not override status: want 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// HTTPFault with custom response headers
// ---------------------------------------------------------------------------

func TestHTTPServer_GlobalFault_Headers(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Status:  http.StatusTooManyRequests,
		Headers: map[string]string{"Retry-After": "60"},
	}})
	resp, _ := http.Get(base + "/ok")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("want 429, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "60" {
		t.Errorf("want Retry-After: 60, got %q", got)
	}
}

func TestHTTPServer_GlobalFault_ServiceUnavailable_WithRetryAfter(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Status:  http.StatusServiceUnavailable,
		Body:    `{"error":"maintenance"}`,
		Headers: map[string]string{"Retry-After": "120", "X-Reason": "maintenance"},
	}})
	resp, _ := http.Get(base + "/ok")
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != 503 {
		t.Errorf("want 503, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(b), "maintenance") {
		t.Errorf("want maintenance body, got %q", string(b))
	}
	if resp.Header.Get("Retry-After") != "120" {
		t.Errorf("want Retry-After: 120, got %q", resp.Header.Get("Retry-After"))
	}
	if resp.Header.Get("X-Reason") != "maintenance" {
		t.Errorf("want X-Reason: maintenance, got %q", resp.Header.Get("X-Reason"))
	}
}

// ---------------------------------------------------------------------------
// Per-mock fault with custom headers
// ---------------------------------------------------------------------------

func TestHTTPServer_PerMockFault_Headers(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "rate-limited",
		Request:  config.HTTPRequest{Method: "GET", Path: "/api/resource"},
		Response: config.HTTPResponse{Status: 200, Body: `{"data":"value"}`},
		Fault: &config.MockFault{
			StatusOverride: http.StatusTooManyRequests,
			Body:           `{"error":"rate limited"}`,
			Headers:        map[string]string{"Retry-After": "30", "X-RateLimit-Limit": "100"},
			ErrorRate:      0, // always inject
		},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	resp, _ := http.Get(base + "/api/resource")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("want 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") != "30" {
		t.Errorf("want Retry-After: 30, got %q", resp.Header.Get("Retry-After"))
	}
	if resp.Header.Get("X-RateLimit-Limit") != "100" {
		t.Errorf("want X-RateLimit-Limit: 100, got %q", resp.Header.Get("X-RateLimit-Limit"))
	}
}

// ---------------------------------------------------------------------------
// Delay + error combined — both latency and status override fire together
// ---------------------------------------------------------------------------

func TestHTTPServer_GlobalFault_DelayAndStatus(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Delay:  config.Duration{Duration: 50 * time.Millisecond},
		Status: http.StatusBadGateway,
	}})
	t0 := time.Now()
	resp, _ := http.Get(base + "/ok")
	elapsed := time.Since(t0)
	defer resp.Body.Close() //nolint:errcheck

	if elapsed < 40*time.Millisecond {
		t.Errorf("expected latency ≥40ms, got %v", elapsed)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("want 502, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Non-error status codes (1xx/2xx/3xx) — no default body injection
// ---------------------------------------------------------------------------

func TestHTTPServer_GlobalFault_Redirect(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "old",
		Request:  config.HTTPRequest{Method: "GET", Path: "/old"},
		Response: config.HTTPResponse{Status: 200, Body: "original"},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Status:  http.StatusMovedPermanently,
		Headers: map[string]string{"Location": "/new"},
	}})

	// Use a non-redirecting client to inspect the raw 301.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, _ := client.Get(base + "/old")
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("want 301, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Location") != "/new" {
		t.Errorf("want Location: /new, got %q", resp.Header.Get("Location"))
	}
	// No default error body should be injected for a 3xx status.
	if strings.Contains(string(b), "fault injected") {
		t.Errorf("3xx fault must not inject error body, got: %q", string(b))
	}
}

func TestHTTPServer_GlobalFault_TemporaryRedirect(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "resource",
		Request:  config.HTTPRequest{Method: "GET", Path: "/resource"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Status:  http.StatusTemporaryRedirect,
		Headers: map[string]string{"Location": "/resource/v2"},
	}})

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, _ := client.Get(base + "/resource")
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("want 307, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Location") != "/resource/v2" {
		t.Errorf("want Location: /resource/v2, got %q", resp.Header.Get("Location"))
	}
}

func TestHTTPServer_GlobalFault_ArbitraryStatusCode(t *testing.T) { // Any integer status code can be injected — test a few representative values.
	cases := []struct {
		status int
		name   string
	}{
		{http.StatusUnauthorized, "401"},
		{http.StatusForbidden, "403"},
		{http.StatusNotFound, "404"},
		{http.StatusMethodNotAllowed, "405"},
		{http.StatusConflict, "409"},
		{http.StatusUnprocessableEntity, "422"},
		{http.StatusTooManyRequests, "429"},
		{http.StatusInternalServerError, "500"},
		{http.StatusBadGateway, "502"},
		{http.StatusGatewayTimeout, "504"},
	}

	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := scenarios.New(nil)
			base := startTestServer(t, mocks, sc)
			sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: tc.status}})
			resp, _ := http.Get(base + "/ok")
			_ = resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Errorf("want %d, got %d", tc.status, resp.StatusCode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Connection abort (TCP reset) — client gets no response
// ---------------------------------------------------------------------------

func TestHTTPServer_GlobalFault_Abort(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Abort: true}})

	resp, err := http.Get(base + "/ok")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected connection error (abort), got a valid response")
	}
	// The client should receive an EOF / connection-reset error, not a response.
}

func TestHTTPServer_GlobalFault_Abort_WithDelay(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Delay: config.Duration{Duration: 50 * time.Millisecond},
		Abort: true,
	}})

	t0 := time.Now()
	resp, err := http.Get(base + "/ok")
	elapsed := time.Since(t0)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected connection error after delay, got valid response")
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected delay ≥40ms before abort, got %v", elapsed)
	}
}

func TestHTTPServer_GlobalFault_Abort_Cleared(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "ok",
		Request:  config.HTTPRequest{Method: "GET", Path: "/ok"},
		Response: config.HTTPResponse{Status: 200},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Abort: true}})
	if _, err := http.Get(base + "/ok"); err == nil {
		t.Fatal("expected abort error while fault is active")
	}

	sc.ClearDirectFaults()
	resp, err := http.Get(base + "/ok")
	if err != nil {
		t.Fatalf("after clearing abort fault: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("after clear: want 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Truncated response — client gets partial body then unexpected EOF
// ---------------------------------------------------------------------------

func TestHTTPServer_GlobalFault_TruncateBody(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "data",
		Request:  config.HTTPRequest{Method: "GET", Path: "/data"},
		Response: config.HTTPResponse{Status: 200, Body: `{"result":"complete-payload-here"}`},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{TruncateBody: 5}})

	// Use a raw connection to bypass Go's http client which buffers errors.
	conn, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	_, _ = fmt.Fprintf(conn, "GET /data HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")

	var buf strings.Builder
	tmp := make([]byte, 1024)
	for {
		n, readErr := conn.Read(tmp)
		buf.Write(tmp[:n])
		if readErr != nil {
			break // EOF or reset — expected
		}
	}
	got := buf.String()
	// Should have received some bytes but not the full body.
	if !strings.Contains(got, "HTTP/1.1 200") {
		t.Errorf("expected HTTP 200 in truncated response, got: %q", got)
	}
	if strings.Contains(got, "complete-payload-here") {
		t.Error("full body must not be present in truncated response")
	}
}

func TestHTTPServer_GlobalFault_TruncateBody_WithStatus(t *testing.T) {
	mocks := []config.HTTPMock{{
		ID:       "data",
		Request:  config.HTTPRequest{Method: "GET", Path: "/data"},
		Response: config.HTTPResponse{Status: 200, Body: `{"result":"value"}`},
	}}
	sc := scenarios.New(nil)
	base := startTestServer(t, mocks, sc)

	// Truncate a 500 error response after 3 bytes.
	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{
		Status:       http.StatusInternalServerError,
		Body:         `{"error":"server-exploded"}`,
		TruncateBody: 3,
	}})

	conn, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	_, _ = fmt.Fprintf(conn, "GET /data HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")

	var buf strings.Builder
	tmp := make([]byte, 2048)
	for {
		n, readErr := conn.Read(tmp)
		buf.Write(tmp[:n])
		if readErr != nil {
			break
		}
	}
	got := buf.String()
	if !strings.Contains(got, "HTTP/1.1 500") {
		t.Errorf("expected HTTP 500 header in truncated response, got: %q", got)
	}
	if strings.Contains(got, "server-exploded") {
		t.Error("full body must not appear in truncated response")
	}
}

// ---------------------------------------------------------------------------
// Authentication tests
// ---------------------------------------------------------------------------

func TestHTTPServer_BearerAuth(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID: "authenticated",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/secure",
				Auth:   &config.HTTPAuth{Type: "bearer", Token: "mysecret"},
			},
			Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
		},
		{
			ID:       "unauthenticated",
			Request:  config.HTTPRequest{Method: "GET", Path: "/secure"},
			Response: config.HTTPResponse{Status: 401, Body: `{"error":"unauthorized"}`},
		},
	}
	base := startTestServer(t, mocks, nil)
	client := &http.Client{}

	// Valid token → 200.
	req, _ := http.NewRequest("GET", base+"/secure", nil)
	req.Header.Set("Authorization", "Bearer mysecret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with valid token, got %d", resp.StatusCode)
	}

	// Wrong token → fallback 401.
	req2, _ := http.NewRequest("GET", base+"/secure", nil)
	req2.Header.Set("Authorization", "Bearer wrong")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Errorf("expected 401 with wrong token, got %d", resp2.StatusCode)
	}

	// No token → fallback 401.
	resp3, err := client.Get(base + "/secure")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 401 {
		t.Errorf("expected 401 with no token, got %d", resp3.StatusCode)
	}
}

func TestHTTPServer_BasicAuth(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID: "admin",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/admin",
				Auth:   &config.HTTPAuth{Type: "basic", Username: "admin", Password: "s3cret"},
			},
			Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
		},
		{
			ID:       "admin-unauth",
			Request:  config.HTTPRequest{Method: "GET", Path: "/admin"},
			Response: config.HTTPResponse{Status: 401, Body: `{"error":"unauthorized"}`},
		},
	}
	base := startTestServer(t, mocks, nil)
	client := &http.Client{}

	// Valid credentials → 200.
	req, _ := http.NewRequest("GET", base+"/admin", nil)
	req.SetBasicAuth("admin", "s3cret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with valid basic auth, got %d", resp.StatusCode)
	}

	// Wrong password → fallback 401.
	req2, _ := http.NewRequest("GET", base+"/admin", nil)
	req2.SetBasicAuth("admin", "wrong")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Errorf("expected 401 with wrong password, got %d", resp2.StatusCode)
	}
}

func TestHTTPServer_APIKey_Header(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID: "weather",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/weather",
				Auth:   &config.HTTPAuth{Type: "api_key", Header: "X-API-Key", Value: "key-abc"},
			},
			Response: config.HTTPResponse{Status: 200, Body: `{"temp":22}`},
		},
		{
			ID:       "weather-unauth",
			Request:  config.HTTPRequest{Method: "GET", Path: "/weather"},
			Response: config.HTTPResponse{Status: 401, Body: `{"error":"unauthorized"}`},
		},
	}
	base := startTestServer(t, mocks, nil)
	client := &http.Client{}

	// Valid key → 200.
	req, _ := http.NewRequest("GET", base+"/weather", nil)
	req.Header.Set("X-API-Key", "key-abc")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with valid API key, got %d", resp.StatusCode)
	}

	// Missing key → fallback 401.
	resp2, err := client.Get(base + "/weather")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Errorf("expected 401 without API key, got %d", resp2.StatusCode)
	}
}

func TestHTTPServer_NTLMHandshake(t *testing.T) {
	mocks := []config.HTTPMock{
		{
			ID: "ntlm-endpoint",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/ntlm",
				Auth:   &config.HTTPAuth{Type: "ntlm"},
			},
			Response: config.HTTPResponse{Status: 200, Body: `{"authenticated":true}`},
		},
	}
	base := startTestServer(t, mocks, nil)
	client := &http.Client{
		// Do not follow redirects; we need to inspect each response.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// Step 1: no token → 401 + WWW-Authenticate: NTLM.
	resp1, err := client.Get(base + "/ntlm")
	if err != nil {
		t.Fatalf("step1 request failed: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != 401 {
		t.Fatalf("step1: expected 401, got %d", resp1.StatusCode)
	}
	wwwAuth1 := resp1.Header.Get("WWW-Authenticate")
	if wwwAuth1 != "NTLM" {
		t.Errorf("step1: expected WWW-Authenticate: NTLM, got %q", wwwAuth1)
	}

	// Step 2: NTLM type-1 Negotiate token → 401 + WWW-Authenticate: NTLM <challenge>.
	// Minimal type-1 message: NTLMSSP\0 + message-type 1 + flags + workstation/domain (all zeros).
	type1Token := buildNTLMType1Token()
	req2, _ := http.NewRequest("GET", base+"/ntlm", nil)
	req2.Header.Set("Authorization", "NTLM "+type1Token)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("step2 request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Fatalf("step2: expected 401, got %d", resp2.StatusCode)
	}
	wwwAuth2 := resp2.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(wwwAuth2, "NTLM ") {
		t.Errorf("step2: expected WWW-Authenticate: NTLM <token>, got %q", wwwAuth2)
	}

	// Step 3: NTLM type-3 Authenticate token → 200.
	type3Token := buildNTLMType3Token()
	req3, _ := http.NewRequest("GET", base+"/ntlm", nil)
	req3.Header.Set("Authorization", "NTLM "+type3Token)
	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("step3 request failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("step3: expected 200, got %d", resp3.StatusCode)
	}
}

// buildNTLMType1Token returns a minimal base64-encoded NTLM type-1 (Negotiate) message.
func buildNTLMType1Token() string {
	msg := make([]byte, 32)
	copy(msg[0:8], "NTLMSSP\x00")
	msg[8] = 0x01 // MessageType = 1
	return base64.StdEncoding.EncodeToString(msg)
}

// buildNTLMType3Token returns a minimal base64-encoded NTLM type-3 (Authenticate) message.
func buildNTLMType3Token() string {
	msg := make([]byte, 32)
	copy(msg[0:8], "NTLMSSP\x00")
	msg[8] = 0x03 // MessageType = 3
	return base64.StdEncoding.EncodeToString(msg)
}

func TestHTTPServer_NTLMDoesNotHijackBearerRequests(t *testing.T) {
	// An NTLM mock and a Bearer mock on the same path must not interfere.
	// A request with a valid Bearer token must reach the Bearer mock, not get
	// hijacked by the NTLM pre-flight handler.
	mocks := []config.HTTPMock{
		{
			ID: "ntlm-mock",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/api/resource",
				Auth:   &config.HTTPAuth{Type: "ntlm"},
			},
			Response: config.HTTPResponse{Status: 200, Body: `{"auth":"ntlm"}`},
		},
		{
			ID: "bearer-mock",
			Request: config.HTTPRequest{
				Method: "GET",
				Path:   "/api/resource",
				Auth:   &config.HTTPAuth{Type: "bearer", Token: "mytoken"},
			},
			Response: config.HTTPResponse{Status: 200, Body: `{"auth":"bearer"}`},
		},
		{
			ID:       "unauth-mock",
			Request:  config.HTTPRequest{Method: "GET", Path: "/api/resource"},
			Response: config.HTTPResponse{Status: 401, Body: `{"error":"unauthorized"}`},
		},
	}
	base := startTestServer(t, mocks, nil)
	client := &http.Client{}

	// Bearer request must reach the bearer mock, not get an NTLM 401.
	req, _ := http.NewRequest("GET", base+"/api/resource", nil)
	req.Header.Set("Authorization", "Bearer mytoken")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for bearer request, got %d (body: %s)", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"bearer"`) {
		t.Errorf("expected bearer mock response, got: %s", body)
	}
}
