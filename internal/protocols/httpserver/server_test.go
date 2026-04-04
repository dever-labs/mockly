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
