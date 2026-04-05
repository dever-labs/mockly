package mocklydriver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetFreePort(t *testing.T) {
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("getFreePort() error: %v", err)
	}
	if port < 1 || port > 65535 {
		t.Fatalf("getFreePort() returned out-of-range port: %d", port)
	}
}

func TestGetBinaryPathReturnsEmptyWhenMissing(t *testing.T) {
	t.Setenv("MOCKLY_BINARY_PATH", "")
	result := GetBinaryPath("/nonexistent/path/that/does/not/exist")
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestGetBinaryPathRespectsEnvVar(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "mockly-fake-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	f.Close()

	t.Setenv("MOCKLY_BINARY_PATH", f.Name())
	result := GetBinaryPath("")
	if result != f.Name() {
		t.Fatalf("expected %q, got %q", f.Name(), result)
	}
}

func TestGetBinaryPathIgnoresMissingEnvVar(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("MOCKLY_BINARY_PATH", nonExistent)

	result := GetBinaryPath("")
	if result != "" {
		t.Fatalf("expected empty string when MOCKLY_BINARY_PATH points to missing file, got %q", result)
	}
}

func TestInstallReturnsErrorWithNoInstall(t *testing.T) {
	t.Setenv("MOCKLY_BINARY_PATH", "")
	t.Setenv("MOCKLY_NO_INSTALL", "1")

	_, err := Install(InstallOptions{BinDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error when MOCKLY_NO_INSTALL is set, got nil")
	}
	if !strings.Contains(err.Error(), "MOCKLY_NO_INSTALL") {
		t.Fatalf("expected error message to mention MOCKLY_NO_INSTALL, got: %v", err)
	}
}

func TestInstallReturnsStagedBinaryPath(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "mockly-staged-*")
	if err != nil {
		t.Fatalf("creating staged binary: %v", err)
	}
	f.Close()

	t.Setenv("MOCKLY_BINARY_PATH", f.Name())

	got, err := Install(InstallOptions{})
	if err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}
	if got != f.Name() {
		t.Fatalf("expected %q, got %q", f.Name(), got)
	}
}

func TestIsPortConflict(t *testing.T) {
	cases := []struct {
		msg      string
		expected bool
	}{
		{"listen tcp 0.0.0.0:9000: bind: address already in use", true},
		{"EADDRINUSE :::9000", true},
		{"bind: address already in use", true},
		{"connection refused", false},
		{"timeout waiting for server", false},
		{"", false},
	}

	for _, c := range cases {
		got := isPortConflict(c.msg)
		if got != c.expected {
			t.Errorf("isPortConflict(%q) = %v, want %v", c.msg, got, c.expected)
		}
	}
}

func TestIsPortConflictAdditional(t *testing.T) {
	cases := []struct {
		msg      string
		expected bool
	}{
		{"EADDRINUSE :::8080", true},
		{"address already in use", true},
		{"listen tcp4 0.0.0.0:80: bind: permission denied", false},
		{"network unreachable", false},
	}
	for _, c := range cases {
		got := isPortConflict(c.msg)
		if got != c.expected {
			t.Errorf("isPortConflict(%q) = %v, want %v", c.msg, got, c.expected)
		}
	}
}

func TestYamlStr(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"hello", "'hello'"},
		{"it's here", "'it''s here'"},
		{`back\slash`, `'back\slash'`},
		{"", "''"},
		{"line\nnewline", "'line\nnewline'"},
	}
	for _, c := range cases {
		got := yamlStr(c.input)
		if got != c.expected {
			t.Errorf("yamlStr(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestWriteConfigNilScenarios(t *testing.T) {
	s := &Server{HTTPPort: 8080, APIPort: 8081}
	path, err := s.writeConfig(nil)
	if err != nil {
		t.Fatalf("writeConfig(nil) error: %v", err)
	}
	defer os.Remove(path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("config file does not exist at %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	cs := string(content)
	for _, want := range []string{"port: 8080", "port: 8081"} {
		if !strings.Contains(cs, want) {
			t.Errorf("config missing %q:\n%s", want, cs)
		}
	}
}

func TestWriteConfigWithScenario(t *testing.T) {
	status := 404
	s := &Server{HTTPPort: 9090, APIPort: 9091}
	sc := Scenario{
		ID:   "sc-1",
		Name: "Not Found",
		Patches: []ScenarioPatch{
			{MockID: "mock-1", Status: &status},
		},
	}
	path, err := s.writeConfig([]Scenario{sc})
	if err != nil {
		t.Fatalf("writeConfig error: %v", err)
	}
	defer os.Remove(path)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	cs := string(content)
	for _, want := range []string{"'sc-1'", "'Not Found'", "'mock-1'"} {
		if !strings.Contains(cs, want) {
			t.Errorf("config missing %q:\n%s", want, cs)
		}
	}
}

func newTestServer(t *testing.T, apiURL string) *Server {
	t.Helper()
	return &Server{
		HTTPPort: 9999,
		APIPort:  9998,
		HTTPBase: "http://127.0.0.1:9999",
		APIBase:  apiURL,
	}
}

func TestAddMockSuccess(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	mock := Mock{
		ID:       "m1",
		Request:  MockRequest{Method: "GET", Path: "/ping"},
		Response: MockResponse{Status: 200},
	}
	if err := s.AddMock(mock); err != nil {
		t.Fatalf("AddMock error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/mocks/http" {
		t.Errorf("expected path /api/mocks/http, got %s", gotPath)
	}

	var parsed map[string]any
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("parsing body: %v", err)
	}
	if id, _ := parsed["id"].(string); id != "m1" {
		t.Errorf("body.id = %q, want m1", id)
	}
	req, _ := parsed["request"].(map[string]any)
	if method, _ := req["method"].(string); method != "GET" {
		t.Errorf("body.request.method = %q, want GET", method)
	}
	if p, _ := req["path"].(string); p != "/ping" {
		t.Errorf("body.request.path = %q, want /ping", p)
	}
	resp, _ := parsed["response"].(map[string]any)
	if status, _ := resp["status"].(float64); int(status) != 200 {
		t.Errorf("body.response.status = %v, want 200", status)
	}
}

func TestAddMockErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	err := s.AddMock(Mock{ID: "m2", Request: MockRequest{Method: "GET", Path: "/x"}, Response: MockResponse{Status: 200}})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code, got: %v", err)
	}
}

func TestDeleteMock(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	if err := s.DeleteMock("my-id"); err != nil {
		t.Fatalf("DeleteMock error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/api/mocks/http/my-id" {
		t.Errorf("expected /api/mocks/http/my-id, got %s", gotPath)
	}
}

func TestReset(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	if err := s.Reset(); err != nil {
		t.Fatalf("Reset error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/reset" {
		t.Errorf("expected /api/reset, got %s", gotPath)
	}
}

func TestActivateScenario(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	if err := s.ActivateScenario("sc1"); err != nil {
		t.Fatalf("ActivateScenario error: %v", err)
	}
	if gotPath != "/api/scenarios/sc1/activate" {
		t.Errorf("expected /api/scenarios/sc1/activate, got %s", gotPath)
	}
}

func TestDeactivateScenario(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	if err := s.DeactivateScenario("sc1"); err != nil {
		t.Fatalf("DeactivateScenario error: %v", err)
	}
	if gotPath != "/api/scenarios/sc1/deactivate" {
		t.Errorf("expected /api/scenarios/sc1/deactivate, got %s", gotPath)
	}
}

func TestSetFault(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	if err := s.SetFault(FaultConfig{Enabled: true, Delay: "100ms"}); err != nil {
		t.Fatalf("SetFault error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/fault" {
		t.Errorf("expected /api/fault, got %s", gotPath)
	}
}

func TestClearFault(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	if err := s.ClearFault(); err != nil {
		t.Fatalf("ClearFault error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/api/fault" {
		t.Errorf("expected /api/fault, got %s", gotPath)
	}
}

func TestSleep(t *testing.T) {
	start := time.Now()
	sleep(10 * time.Millisecond)
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Errorf("sleep(10ms) returned too quickly: elapsed %v", elapsed)
	}
}
