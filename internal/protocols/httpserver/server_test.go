package httpserver_test

import (
	"context"
	"crypto/tls"
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

	sc.SetFault(&config.GlobalFault{Enabled: true, StatusOverride: 503, ErrorRate: 0})

	resp, _ := http.Get(base + "/ok")
	_ = resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("global fault should override to 503, got %d", resp.StatusCode)
	}

	sc.ClearFault()
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
	sc.SetFault(&config.GlobalFault{Enabled: true, Delay: config.Duration{Duration: 100 * time.Millisecond}})
	t1 := time.Now()
	http.Get(base + "/fast") //nolint:errcheck
	withFault := time.Since(t1)
	sc.ClearFault()

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
		ID:      "flaky",
		Request: config.HTTPRequest{Method: "GET", Path: "/data"},
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
		ID:      "cycler",
		Request: config.HTTPRequest{Method: "GET", Path: "/cycle"},
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
