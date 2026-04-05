package httpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
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
	ln.Close()

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
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
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
	resp.Body.Close()

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
		resp.Body.Close()
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
	resp.Body.Close()

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
	r1, _ := http.Post(base+"/token", "application/json", nil)
	r1.Body.Close()
	if r1.StatusCode != 200 {
		t.Errorf("before activation: want 200, got %d", r1.StatusCode)
	}

	// Activate scenario
	sc.Activate("auth-down")

	// With scenario: 503
	r2, _ := http.Post(base+"/token", "application/json", nil)
	defer r2.Body.Close()
	if r2.StatusCode != 503 {
		t.Errorf("after activation: want 503, got %d", r2.StatusCode)
	}
	body, _ := io.ReadAll(r2.Body)
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
	resp.Body.Close()
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
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("global fault should override to 503, got %d", resp.StatusCode)
	}

	sc.ClearFault()
	resp2, _ := http.Get(base + "/ok")
	resp2.Body.Close()
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

	resp, _ := http.Get(base + "/time")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

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
			resp.Body.Close()
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
	ln.Close()

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

	resp, _ := http.Get(base + "/users?role=admin")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"admin"`) {
		t.Errorf("expected admin response, got %q", body)
	}

	resp2, _ := http.Get(base + "/users?role=user")
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
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

	resp, _ := http.Post(base+"/pay", "application/json", strings.NewReader(`{"currency":"GBP","amount":100}`))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for GBP, got %d", resp.StatusCode)
	}

	resp2, _ := http.Post(base+"/pay", "application/json", strings.NewReader(`{"currency":"USD","amount":100}`))
	defer resp2.Body.Close()
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
		resp.Body.Close()
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
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
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
	resp.Body.Close()
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
		resp.Body.Close()
	}

	if n := srv.CallCount("counted"); n != 3 {
		t.Fatalf("expected 3 calls, got %d", n)
	}

	srv.ResetCallCounts()
	if n := srv.CallCount("counted"); n != 0 {
		t.Fatalf("expected 0 after reset, got %d", n)
	}
}
