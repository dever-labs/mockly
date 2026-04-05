//go:build e2e

// Package e2e runs end-to-end tests against a real Mockly binary.
// The tests are gated with the "e2e" build tag so they only run when
// explicitly requested:
//
//	go test -tags e2e ./tests/e2e/...
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestMain: build binary once, run all tests, clean up.
// ---------------------------------------------------------------------------

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary into a temp dir.
	dir, err := os.MkdirTemp("", "mockly-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "MkdirTemp: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binName := "mockly"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binaryPath = filepath.Join(dir, binName)

	// Resolve the module root (two levels up from tests/e2e).
	_, callerFile, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Join(filepath.Dir(callerFile), "..", "..")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/mockly")
	cmd.Dir = moduleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// freePort returns a free TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// startMockly writes a config, starts the mockly binary, and returns the API
// base URL and HTTP mock base URL. The process is killed when the test ends.
//
// cfgFmt is a fmt format string. The first %d is replaced with the allocated
// HTTP port so callers can embed `port: %d` under their http: block without
// producing a duplicate top-level `protocols:` key.
func startMockly(t *testing.T, cfgFmt string) (apiBase, httpBase string) {
	t.Helper()

	apiPort := freePort(t)
	httpPort := freePort(t)

	// Inject the HTTP port into the caller's YAML, then prepend the api port.
	cfgYAML := fmt.Sprintf(cfgFmt, httpPort)
	fullCfg := fmt.Sprintf("mockly:\n  api:\n    port: %d\n", apiPort) + cfgYAML

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(cfgPath, []byte(fullCfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(binaryPath, "start",
		"--config", cfgPath,
		"--api-port", fmt.Sprintf("%d", apiPort),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mockly: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill() //nolint:errcheck
		cmd.Wait()         //nolint:errcheck
	})

	apiBase = fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	httpBase = fmt.Sprintf("http://127.0.0.1:%d", httpPort)

	waitForHTTP(t, apiBase+"/api/protocols", 10*time.Second)
	return apiBase, httpBase
}

func waitForHTTP(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s within %v", url, timeout)
}

func mustGetJSON(t *testing.T, url string, out interface{}) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: status %d — %s", url, resp.StatusCode, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func postJSON(t *testing.T, url string, payload interface{}) *http.Response {
	t.Helper()
	b, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// ---------------------------------------------------------------------------
// E2E tests
// ---------------------------------------------------------------------------

func TestE2E_StartAndProtocolList(t *testing.T) {
	apiBase, _ := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
`)
	var protocols []map[string]interface{}
	mustGetJSON(t, apiBase+"/api/protocols", &protocols)
	if len(protocols) == 0 {
		t.Fatal("expected at least one protocol")
	}
	found := false
	for _, p := range protocols {
		if p["protocol"] == "http" {
			found = true
		}
	}
	if !found {
		t.Error("http protocol not listed")
	}
}

func TestE2E_HTTP_MockAndRequest(t *testing.T) {
	apiBase, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
    mocks:
      - id: hello
        request:
          method: GET
          path: /hello
        response:
          status: 200
          body: '{"msg":"world"}'
          headers:
            Content-Type: application/json
`)

	// Hit the mock directly.
	resp, err := http.Get(httpBase + "/hello")
	if err != nil {
		t.Fatalf("GET /hello: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "world") {
		t.Errorf("unexpected body: %s", body)
	}

	// Verify mock appears in API.
	var mocks []map[string]interface{}
	mustGetJSON(t, apiBase+"/api/mocks/http", &mocks)
	if len(mocks) != 1 || mocks[0]["id"] != "hello" {
		t.Errorf("unexpected mocks from API: %+v", mocks)
	}
}

func TestE2E_HTTP_CreateMockViaAPI(t *testing.T) {
	apiBase, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
`)

	// Create a mock via the management API.
	mock := map[string]interface{}{
		"id":       "dynamic",
		"request":  map[string]interface{}{"method": "GET", "path": "/dynamic"},
		"response": map[string]interface{}{"status": 200, "body": `{"dynamic":true}`},
	}
	resp := postJSON(t, apiBase+"/api/mocks/http", mock)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("create mock via API: want 201, got %d", resp.StatusCode)
	}

	// Hit the newly created mock.
	resp2, err := http.Get(httpBase + "/dynamic")
	if err != nil {
		t.Fatalf("GET /dynamic: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp2.StatusCode)
	}
}

func TestE2E_ScenarioActivation(t *testing.T) {
	apiBase, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
    mocks:
      - id: token
        request:
          method: POST
          path: /token
        response:
          status: 200
          body: '{"access_token":"abc"}'
scenarios:
  - id: auth-down
    name: Auth Down
    patches:
      - mock_id: token
        status: 503
        body: '{"error":"down"}'
`)

	// Without scenario: 200.
	r1, _ := http.Post(httpBase+"/token", "application/json", nil)
	r1.Body.Close()
	if r1.StatusCode != 200 {
		t.Fatalf("before scenario: want 200, got %d", r1.StatusCode)
	}

	// Activate the scenario via API.
	resp := postJSON(t, apiBase+"/api/scenarios/auth-down/activate", nil)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("activate scenario: want 200, got %d", resp.StatusCode)
	}

	// With scenario active: 503.
	r2, _ := http.Post(httpBase+"/token", "application/json", nil)
	defer r2.Body.Close()
	if r2.StatusCode != 503 {
		t.Errorf("after activation: want 503, got %d", r2.StatusCode)
	}
	b, _ := io.ReadAll(r2.Body)
	if !strings.Contains(string(b), "down") {
		t.Errorf("want 'down' in body, got: %s", b)
	}
}

func TestE2E_FaultInjection(t *testing.T) {
	apiBase, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
    mocks:
      - id: ok
        request:
          method: GET
          path: /ok
        response:
          status: 200
`)

	// Set global fault.
	resp := postJSON(t, apiBase+"/api/fault", map[string]interface{}{
		"enabled":         true,
		"status_override": 503,
		"error_rate":      0,
	})
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("set fault: want 200, got %d", resp.StatusCode)
	}

	r1, _ := http.Get(httpBase + "/ok")
	r1.Body.Close()
	if r1.StatusCode != 503 {
		t.Errorf("with fault: want 503, got %d", r1.StatusCode)
	}

	// Clear fault.
	req, _ := http.NewRequest(http.MethodDelete, apiBase+"/api/fault", nil)
	resp2, _ := http.DefaultClient.Do(req)
	resp2.Body.Close()

	r2, _ := http.Get(httpBase + "/ok")
	r2.Body.Close()
	if r2.StatusCode != 200 {
		t.Errorf("after fault cleared: want 200, got %d", r2.StatusCode)
	}
}

func TestE2E_Reset(t *testing.T) {
	apiBase, _ := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
`)

	// Create a mock, then reset.
	mock := map[string]interface{}{
		"id":       "temp",
		"request":  map[string]interface{}{"method": "GET", "path": "/temp"},
		"response": map[string]interface{}{"status": 200},
	}
	postJSON(t, apiBase+"/api/mocks/http", mock).Body.Close()

	var before []map[string]interface{}
	mustGetJSON(t, apiBase+"/api/mocks/http", &before)
	if len(before) == 0 {
		t.Fatal("expected mock before reset")
	}

	resp := postJSON(t, apiBase+"/api/reset", nil)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("reset: want 200, got %d", resp.StatusCode)
	}

	var after []map[string]interface{}
	mustGetJSON(t, apiBase+"/api/mocks/http", &after)
	if len(after) != 0 {
		t.Errorf("after reset: want 0 mocks, got %d", len(after))
	}
}
