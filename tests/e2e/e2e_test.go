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

	"github.com/gosnmp/gosnmp"
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
	if err := ln.Close(); err != nil {
		t.Fatalf("freePort close: %v", err)
	}
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
		if err := cmd.Process.Kill(); err != nil {
			t.Logf("cleanup: failed to kill mockly process: %v", err)
		}
		if err := cmd.Wait(); err != nil {
			t.Logf("cleanup: wait on mockly process returned error: %v", err)
		}
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
	defer func() { _ = resp.Body.Close() }()
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
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload for POST %s: %v", url, err)
	}
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
	defer resp.Body.Close() //nolint:errcheck
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
	defer resp2.Body.Close() //nolint:errcheck
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
	defer r2.Body.Close() //nolint:errcheck
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

// ---------------------------------------------------------------------------
// Query parameter tests
// ---------------------------------------------------------------------------

// TestE2E_HTTP_QueryParam_ExactMatch verifies that a mock with required query
// params only fires when those params are present with the right value, and a
// fallback mock answers all other requests.
func TestE2E_HTTP_QueryParam_ExactMatch(t *testing.T) {
	_, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
    mocks:
      - id: admin-users
        request:
          method: GET
          path: /api/users
          query:
            role: admin
        response:
          status: 200
          body: '[{"role":"admin"}]'
      - id: all-users
        request:
          method: GET
          path: /api/users
        response:
          status: 200
          body: '[{"role":"any"}]'
`)

	// Exact match → admin mock.
	r1, err := http.Get(httpBase + "/api/users?role=admin")
	if err != nil {
		t.Fatalf("GET ?role=admin: %v", err)
	}
	defer r1.Body.Close() //nolint:errcheck
	if r1.StatusCode != 200 {
		t.Fatalf("want 200, got %d", r1.StatusCode)
	}
	b1, _ := io.ReadAll(r1.Body)
	if !strings.Contains(string(b1), `"admin"`) {
		t.Errorf("expected admin body, got: %s", b1)
	}

	// Different value → fallback mock.
	r2, err := http.Get(httpBase + "/api/users?role=viewer")
	if err != nil {
		t.Fatalf("GET ?role=viewer: %v", err)
	}
	defer r2.Body.Close() //nolint:errcheck
	b2, _ := io.ReadAll(r2.Body)
	if !strings.Contains(string(b2), `"any"`) {
		t.Errorf("expected fallback body, got: %s", b2)
	}

	// No param → fallback mock.
	r3, err := http.Get(httpBase + "/api/users")
	if err != nil {
		t.Fatalf("GET /api/users (no params): %v", err)
	}
	defer r3.Body.Close() //nolint:errcheck
	b3, _ := io.ReadAll(r3.Body)
	if !strings.Contains(string(b3), `"any"`) {
		t.Errorf("expected fallback body, got: %s", b3)
	}
}

// TestE2E_HTTP_QueryParam_Wildcard verifies that a `*` value matches any
// value for that param (useful when the value is dynamic but presence is required).
func TestE2E_HTTP_QueryParam_Wildcard(t *testing.T) {
	_, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
    mocks:
      - id: paginated
        request:
          method: GET
          path: /api/items
          query:
            page: "*"
        response:
          status: 200
          body: '{"paginated":true}'
      - id: unpaginated
        request:
          method: GET
          path: /api/items
        response:
          status: 200
          body: '{"paginated":false}'
`)

	for _, page := range []string{"1", "42", "999"} {
		r, err := http.Get(httpBase + "/api/items?page=" + page)
		if err != nil {
			t.Fatalf("GET ?page=%s: %v", page, err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close() //nolint:errcheck
		if !strings.Contains(string(b), `"paginated":true`) {
			t.Errorf("page=%s: expected wildcard mock, got: %s", page, b)
		}
	}

	// No page param → unpaginated mock.
	r, err := http.Get(httpBase + "/api/items")
	if err != nil {
		t.Fatalf("GET /api/items: %v", err)
	}
	defer r.Body.Close() //nolint:errcheck
	b, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(b), `"paginated":false`) {
		t.Errorf("no page param: expected unpaginated mock, got: %s", b)
	}
}

// TestE2E_HTTP_QueryParam_TemplateInBody verifies that query param values are
// accessible via {{.query.key}} in response bodies.
func TestE2E_HTTP_QueryParam_TemplateInBody(t *testing.T) {
	_, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
    mocks:
      - id: echo
        request:
          method: GET
          path: /echo
        response:
          status: 200
          headers:
            Content-Type: application/json
          body: '{"name":"{{.query.name}}","page":{{.query.page}}}'
`)

	r, err := http.Get(httpBase + "/echo?name=alice&page=3")
	if err != nil {
		t.Fatalf("GET /echo: %v", err)
	}
	defer r.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(body), `"name":"alice"`) {
		t.Errorf("missing name in body: %s", body)
	}
	if !strings.Contains(string(body), `"page":3`) {
		t.Errorf("missing page in body: %s", body)
	}
}

// TestE2E_HTTP_OAuthAuthorize is the primary motivating test: mock an OAuth2
// authorization endpoint that requires specific query params, reflects
// redirect_uri and state in the Location header, and falls back to a 400 for
// unrecognised clients — all without any real auth server.
func TestE2E_HTTP_OAuthAuthorize(t *testing.T) {
	_, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
    mocks:
      - id: oauth-authorize
        request:
          method: GET
          path: /oauth/authorize
          query:
            response_type: code
            client_id: my-app
            redirect_uri: "*"
            state: "*"
        response:
          status: 302
          headers:
            Location: "{{.query.redirect_uri}}?code=mock-code&state={{.query.state}}"
      - id: oauth-bad-client
        request:
          method: GET
          path: /oauth/authorize
        response:
          status: 400
          body: '{"error":"unauthorized_client"}'
`)

	noRedirect := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Valid authorization request → 302 with correct Location.
	url := httpBase + "/oauth/authorize?response_type=code&client_id=my-app" +
		"&redirect_uri=https://app.example.com/callback&state=csrf-token-42"
	r1, err := noRedirect.Get(url)
	if err != nil {
		t.Fatalf("GET authorize: %v", err)
	}
	defer r1.Body.Close() //nolint:errcheck
	if r1.StatusCode != 302 {
		t.Fatalf("want 302, got %d", r1.StatusCode)
	}
	loc := r1.Header.Get("Location")
	if !strings.HasPrefix(loc, "https://app.example.com/callback") {
		t.Errorf("Location should redirect to app callback, got: %s", loc)
	}
	if !strings.Contains(loc, "code=mock-code") {
		t.Errorf("Location missing code, got: %s", loc)
	}
	if !strings.Contains(loc, "state=csrf-token-42") {
		t.Errorf("Location missing reflected state, got: %s", loc)
	}

	// Wrong client_id → 400 error.
	r2, err := noRedirect.Get(httpBase + "/oauth/authorize?response_type=code&client_id=evil" +
		"&redirect_uri=https://evil.example.com&state=x")
	if err != nil {
		t.Fatalf("GET authorize (bad client): %v", err)
	}
	defer r2.Body.Close() //nolint:errcheck
	if r2.StatusCode != 400 {
		t.Errorf("bad client: want 400, got %d", r2.StatusCode)
	}
	b2, _ := io.ReadAll(r2.Body)
	if !strings.Contains(string(b2), "unauthorized_client") {
		t.Errorf("expected unauthorized_client error, got: %s", b2)
	}
}

// TestE2E_HTTP_QueryParam_ViaAPI verifies that a mock with query param
// constraints can be created through the management REST API (not just YAML).
func TestE2E_HTTP_QueryParam_ViaAPI(t *testing.T) {
	apiBase, httpBase := startMockly(t, `
protocols:
  http:
    enabled: true
    port: %d
`)

	// Register mock with query constraints via the API.
	mock := map[string]interface{}{
		"id": "search",
		"request": map[string]interface{}{
			"method": "GET",
			"path":   "/search",
			"query":  map[string]interface{}{"q": "*", "lang": "en"},
		},
		"response": map[string]interface{}{
			"status": 200,
			"body":   `{"results":[]}`,
		},
	}
	resp := postJSON(t, apiBase+"/api/mocks/http", mock)
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 201 {
		t.Fatalf("create mock: want 201, got %d", resp.StatusCode)
	}

	// Matching request → 200.
	r1, err := http.Get(httpBase + "/search?q=golang&lang=en")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer r1.Body.Close() //nolint:errcheck
	if r1.StatusCode != 200 {
		t.Errorf("matching request: want 200, got %d", r1.StatusCode)
	}

	// Wrong lang → no match → 404.
	r2, err := http.Get(httpBase + "/search?q=golang&lang=fr")
	if err != nil {
		t.Fatalf("GET /search (wrong lang): %v", err)
	}
	defer r2.Body.Close() //nolint:errcheck
	if r2.StatusCode != 404 {
		t.Errorf("wrong lang: want 404, got %d", r2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// SNMP helpers
// ---------------------------------------------------------------------------

// startMocklyWithSNMP starts Mockly with SNMP enabled on a free port and
// returns (apiBase, snmpPort).
func startMocklyWithSNMP(t *testing.T, extraMocks string) (apiBase string, snmpPort int) {
t.Helper()

apiPort := freePort(t)
snmpPort = freePort(t)
httpPort := freePort(t)

cfg := fmt.Sprintf(`mockly:
  api:
    port: %d
protocols:
  http:
    enabled: true
    port: %d
  snmp:
    enabled: true
    port: %d
    community: public
    mocks:
      - id: sys-descr
        oid: 1.3.6.1.2.1.1.1.0
        type: string
        value: "Mockly Virtual Device"
      - id: sys-uptime
        oid: 1.3.6.1.2.1.1.3.0
        type: timeticks
        value: 987654
%s`, apiPort, httpPort, snmpPort, extraMocks)

dir := t.TempDir()
cfgPath := filepath.Join(dir, "mockly.yaml")
if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
t.Fatalf("write config: %v", err)
}

cmd := exec.Command(binaryPath, "start", "--config", cfgPath)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Start(); err != nil {
t.Fatalf("start mockly: %v", err)
}
t.Cleanup(func() {
if err := cmd.Process.Kill(); err != nil {
t.Logf("cleanup: failed to kill mockly process: %v", err)
}
_ = cmd.Wait()
})

apiBase = fmt.Sprintf("http://127.0.0.1:%d", apiPort)
waitForHTTP(t, apiBase+"/api/protocols", 10*time.Second)

// Wait for SNMP UDP port to become reachable.
deadline := time.Now().Add(10 * time.Second)
for time.Now().Before(deadline) {
g := &gosnmp.GoSNMP{
Target:    "127.0.0.1",
Port:      uint16(snmpPort),
Community: "public",
Version:   gosnmp.Version2c,
Timeout:   200 * time.Millisecond,
Retries:   0,
}
if err := g.Connect(); err == nil {
_, err = g.Get([]string{"1.3.6.1.2.1.1.1.0"})
g.Conn.Close()
if err == nil {
break
}
}
time.Sleep(50 * time.Millisecond)
}
return apiBase, snmpPort
}

// snmpGet connects a v2c client to the given port and returns the value of the
// first variable in the GET response.
func snmpGet(t *testing.T, port int, oid string) gosnmp.SnmpPDU {
t.Helper()
g := &gosnmp.GoSNMP{
Target:    "127.0.0.1",
Port:      uint16(port),
Community: "public",
Version:   gosnmp.Version2c,
Timeout:   3 * time.Second,
Retries:   1,
}
if err := g.Connect(); err != nil {
t.Fatalf("snmpGet Connect: %v", err)
}
defer g.Conn.Close()
result, err := g.Get([]string{oid})
if err != nil {
t.Fatalf("snmpGet %s: %v", oid, err)
}
if len(result.Variables) == 0 {
t.Fatalf("snmpGet %s: empty response", oid)
}
return result.Variables[0]
}

// ---------------------------------------------------------------------------
// SNMP E2E tests
// ---------------------------------------------------------------------------

func TestE2E_SNMP_GET_ReturnsConfiguredValue(t *testing.T) {
_, snmpPort := startMocklyWithSNMP(t, "")

pdu := snmpGet(t, snmpPort, "1.3.6.1.2.1.1.1.0")
got := string(pdu.Value.([]byte))
if got != "Mockly Virtual Device" {
t.Errorf("sys-descr: want %q, got %q", "Mockly Virtual Device", got)
}
}

func TestE2E_SNMP_GETNEXT_WalksOIDs(t *testing.T) {
_, snmpPort := startMocklyWithSNMP(t, "")

g := &gosnmp.GoSNMP{
Target:    "127.0.0.1",
Port:      uint16(snmpPort),
Community: "public",
Version:   gosnmp.Version2c,
Timeout:   3 * time.Second,
Retries:   1,
}
if err := g.Connect(); err != nil {
t.Fatalf("Connect: %v", err)
}
defer g.Conn.Close()

// GETNEXT on sys-descr should return sys-uptime (next OID).
result, err := g.GetNext([]string{"1.3.6.1.2.1.1.1.0"})
if err != nil {
t.Fatalf("GetNext: %v", err)
}
if len(result.Variables) == 0 {
t.Fatal("GetNext: empty response")
}
nextOID := result.Variables[0].Name
if !strings.Contains(nextOID, "1.3.6.1.2.1.1.3.0") {
t.Errorf("GETNEXT from sys-descr: expected next OID to contain sys-uptime, got %s", nextOID)
}
}

func TestE2E_SNMP_SET_UpdatesValue(t *testing.T) {
_, snmpPort := startMocklyWithSNMP(t, "")

g := &gosnmp.GoSNMP{
Target:    "127.0.0.1",
Port:      uint16(snmpPort),
Community: "public",
Version:   gosnmp.Version2c,
Timeout:   3 * time.Second,
Retries:   1,
}
if err := g.Connect(); err != nil {
t.Fatalf("Connect: %v", err)
}
defer g.Conn.Close()

pdus := []gosnmp.SnmpPDU{{
Name:  "1.3.6.1.2.1.1.1.0",
Type:  gosnmp.OctetString,
Value: "Updated Device",
}}
if _, err := g.Set(pdus); err != nil {
t.Fatalf("SET: %v", err)
}

// Verify with GET.
got := snmpGet(t, snmpPort, "1.3.6.1.2.1.1.1.0")
gotStr := string(got.Value.([]byte))
if gotStr != "Updated Device" {
t.Errorf("after SET: want %q, got %q", "Updated Device", gotStr)
}
}

func TestE2E_SNMP_APICRUDReflectsInGET(t *testing.T) {
apiBase, snmpPort := startMocklyWithSNMP(t, "")

// Add a new OID mock via API.
newMock := map[string]interface{}{
"id":    "custom-oid",
"oid":   "1.3.6.1.4.1.9999.99.0",
"type":  "integer",
"value": 42,
}
resp := postJSON(t, apiBase+"/api/mocks/snmp", newMock)
defer resp.Body.Close()
if resp.StatusCode != 201 {
body, _ := io.ReadAll(resp.Body)
t.Fatalf("POST /api/mocks/snmp: want 201, got %d — %s", resp.StatusCode, body)
}

// Give the server a moment to apply the new mock.
time.Sleep(300 * time.Millisecond)

pdu := snmpGet(t, snmpPort, "1.3.6.1.4.1.9999.99.0")
got, ok := pdu.Value.(int)
if !ok {
t.Fatalf("expected integer value, got %T", pdu.Value)
}
if got != 42 {
t.Errorf("custom OID: want 42, got %d", got)
}
}

func TestE2E_SNMP_TrapSend_Returns200(t *testing.T) {
apiBase, _ := startMocklyWithSNMP(t, "")

// Add a trap config.
trap := map[string]interface{}{
"id":        "test-trap",
"target":    "127.0.0.1:1162",
"version":   "2c",
"community": "public",
"oid":       "1.3.6.1.6.3.1.1.5.1",
"bindings": []map[string]interface{}{
{"oid": "1.3.6.1.2.1.1.1.0", "type": "string", "value": "test"},
},
}
r1 := postJSON(t, apiBase+"/api/snmp/traps", trap)
defer r1.Body.Close()
if r1.StatusCode != 201 {
body, _ := io.ReadAll(r1.Body)
t.Fatalf("POST /api/snmp/traps: want 201, got %d — %s", r1.StatusCode, body)
}

// Send the trap — 127.0.0.1:1162 likely has no listener, but the API
// should still return 200 if the UDP packet was dispatched (best-effort).
r2 := postJSON(t, apiBase+"/api/snmp/traps/test-trap/send", nil)
defer r2.Body.Close()
if r2.StatusCode != 200 {
body, _ := io.ReadAll(r2.Body)
t.Fatalf("POST /api/snmp/traps/test-trap/send: want 200, got %d — %s", r2.StatusCode, body)
}
}

func TestE2E_SNMP_UnknownOID_ReturnsNoSuchObject(t *testing.T) {
_, snmpPort := startMocklyWithSNMP(t, "")

g := &gosnmp.GoSNMP{
Target:    "127.0.0.1",
Port:      uint16(snmpPort),
Community: "public",
Version:   gosnmp.Version2c,
Timeout:   3 * time.Second,
Retries:   1,
}
if err := g.Connect(); err != nil {
t.Fatalf("Connect: %v", err)
}
defer g.Conn.Close()

result, err := g.Get([]string{"1.3.6.1.4.1.99999.0.0.0"})
if err != nil {
t.Fatalf("GET unknown OID: %v", err)
}
if len(result.Variables) == 0 {
t.Fatal("GET unknown OID: no variables in response")
}
pduType := result.Variables[0].Type
// GoSNMPServer returns NoSuchObject (0x80) or NoSuchInstance (0x81) for unknown OIDs.
if pduType != gosnmp.NoSuchObject && pduType != gosnmp.NoSuchInstance {
t.Errorf("unknown OID: expected NoSuchObject or NoSuchInstance, got type %v", pduType)
}
}
