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
		w.WriteHeader(http.StatusOK)
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
	if gotPath != "/api/fault/http" {
		t.Errorf("expected /api/fault/http, got %s", gotPath)
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

func testMockFixture(id string) Mock {
	return Mock{
		ID:       id,
		Request:  MockRequest{Method: "GET", Path: "/ping"},
		Response: MockResponse{Status: 200, Body: "ok"},
	}
}

func testScenarioFixture(id, name string) Scenario {
	return Scenario{ID: id, Name: name, Patches: []ScenarioPatch{}}
}

func TestListMocks(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"m1","request":{"method":"GET","path":"/ping"},"response":{"status":200,"body":"ok"}}]`))
	}))
	defer ts.Close()

	s := newTestServer(t, ts.URL)
	mocks, err := s.ListMocks()
	if err != nil {
		t.Fatalf("ListMocks error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/api/mocks/http" {
		t.Errorf("expected /api/mocks/http, got %s", gotPath)
	}
	if len(mocks) != 1 || mocks[0].ID != "m1" {
		t.Fatalf("unexpected mocks: %#v", mocks)
	}
}

func TestListMocksError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer ts.Close()

	_, err := newTestServer(t, ts.URL).ListMocks()
	if err == nil {
		t.Fatal("expected error for ListMocks")
	}
}

func TestUpdateMock(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"m1","request":{"method":"GET","path":"/ping"},"response":{"status":201,"body":"updated"}}`))
	}))
	defer ts.Close()

	updated, err := newTestServer(t, ts.URL).UpdateMock("m1", testMockFixture("m1"))
	if err != nil {
		t.Fatalf("UpdateMock error: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/api/mocks/http/m1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal update body: %v", err)
	}
	if payload["id"] != "m1" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if updated.ID != "m1" || updated.Response.Status != 201 {
		t.Fatalf("unexpected updated mock: %#v", updated)
	}
}

func TestUpdateMockError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := newTestServer(t, ts.URL).UpdateMock("m1", testMockFixture("m1"))
	if err == nil {
		t.Fatal("expected error for UpdateMock")
	}
}

func TestPatchMock(t *testing.T) {
	status := 201
	body := "patched"
	var gotMethod, gotPath string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"m1","request":{"method":"GET","path":"/ping"},"response":{"status":201,"body":"patched"}}`))
	}))
	defer ts.Close()

	patched, err := newTestServer(t, ts.URL).PatchMock("m1", MockResponsePatch{Status: &status, Body: &body})
	if err != nil {
		t.Fatalf("PatchMock error: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/api/mocks/http/m1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal patch body: %v", err)
	}
	if int(payload["status"].(float64)) != 201 {
		t.Fatalf("unexpected patch payload: %#v", payload)
	}
	if patched.Response.Body != "patched" {
		t.Fatalf("unexpected patched mock: %#v", patched)
	}
}

func TestPatchMockError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := newTestServer(t, ts.URL).PatchMock("m1", MockResponsePatch{})
	if err == nil {
		t.Fatal("expected error for PatchMock")
	}
}

func TestGetState(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key1":"val1"}`))
	}))
	defer ts.Close()

	state, err := newTestServer(t, ts.URL).GetState()
	if err != nil {
		t.Fatalf("GetState error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/state" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if state["key1"] != "val1" {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestGetStateError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	_, err := newTestServer(t, ts.URL).GetState()
	if err == nil {
		t.Fatal("expected error for GetState")
	}
}

func TestSetState(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key1":"val1"}`))
	}))
	defer ts.Close()

	state, err := newTestServer(t, ts.URL).SetState(map[string]string{"key1": "val1"})
	if err != nil {
		t.Fatalf("SetState error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/state" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal set state body: %v", err)
	}
	if payload["key1"] != "val1" || state["key1"] != "val1" {
		t.Fatalf("unexpected state payload/result: %#v %#v", payload, state)
	}
}

func TestSetStateError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	_, err := newTestServer(t, ts.URL).SetState(map[string]string{"key1": "val1"})
	if err == nil {
		t.Fatal("expected error for SetState")
	}
}

func TestDeleteState(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if err := newTestServer(t, ts.URL).DeleteState("key1"); err != nil {
		t.Fatalf("DeleteState error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/state/key1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
}

func TestDeleteStateError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).DeleteState("key1"); err == nil {
		t.Fatal("expected error for DeleteState")
	}
}

func TestGetLogs(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"c1","timestamp":"2026-01-01T00:00:00Z","protocol":"http","method":"GET","path":"/ping","status":200,"duration_ms":5,"matched_id":"m1"}]`))
	}))
	defer ts.Close()

	logs, err := newTestServer(t, ts.URL).GetLogs("m1")
	if err != nil {
		t.Fatalf("GetLogs error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/logs" || gotQuery != "matched_id=m1" {
		t.Fatalf("unexpected request: %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if len(logs) != 1 || logs[0].MatchedID != "m1" {
		t.Fatalf("unexpected logs: %#v", logs)
	}
}

func TestGetLogsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	_, err := newTestServer(t, ts.URL).GetLogs("")
	if err == nil {
		t.Fatal("expected error for GetLogs")
	}
}

func TestClearLogs(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).ClearLogs(); err != nil {
		t.Fatalf("ClearLogs error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/logs" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
}

func TestClearLogsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).ClearLogs(); err == nil {
		t.Fatal("expected error for ClearLogs")
	}
}

func TestGetLogsCount(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":5}`))
	}))
	defer ts.Close()
	count, err := newTestServer(t, ts.URL).GetLogsCount("m1")
	if err != nil {
		t.Fatalf("GetLogsCount error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/logs/count" || gotQuery != "matched_id=m1" {
		t.Fatalf("unexpected request: %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if count != 5 {
		t.Fatalf("expected count 5, got %d", count)
	}
}

func TestGetLogsCountError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).GetLogsCount(""); err == nil {
		t.Fatal("expected error for GetLogsCount")
	}
}

func TestListScenarios(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"s1","name":"Test","patches":[]}]`))
	}))
	defer ts.Close()
	scenarios, err := newTestServer(t, ts.URL).ListScenarios()
	if err != nil {
		t.Fatalf("ListScenarios error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/scenarios" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if len(scenarios) != 1 || scenarios[0].ID != "s1" {
		t.Fatalf("unexpected scenarios: %#v", scenarios)
	}
}

func TestListScenariosError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).ListScenarios(); err == nil {
		t.Fatal("expected error for ListScenarios")
	}
}

func TestCreateScenario(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"s1","name":"Test","patches":[]}`))
	}))
	defer ts.Close()
	created, err := newTestServer(t, ts.URL).CreateScenario(testScenarioFixture("s1", "Test"))
	if err != nil {
		t.Fatalf("CreateScenario error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/scenarios" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal create scenario body: %v", err)
	}
	if payload["id"] != "s1" || created.Name != "Test" {
		t.Fatalf("unexpected create payload/result: %#v %#v", payload, created)
	}
}

func TestCreateScenarioError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).CreateScenario(testScenarioFixture("s1", "Test")); err == nil {
		t.Fatal("expected error for CreateScenario")
	}
}

func TestGetScenario(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"s1","name":"Test","patches":[]}`))
	}))
	defer ts.Close()
	scenario, err := newTestServer(t, ts.URL).GetScenario("s1")
	if err != nil {
		t.Fatalf("GetScenario error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/scenarios/s1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if scenario.ID != "s1" {
		t.Fatalf("unexpected scenario: %#v", scenario)
	}
}

func TestGetScenarioError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).GetScenario("s1"); err == nil {
		t.Fatal("expected error for GetScenario")
	}
}

func TestUpdateScenario(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"s1","name":"Updated","patches":[]}`))
	}))
	defer ts.Close()
	updated, err := newTestServer(t, ts.URL).UpdateScenario("s1", testScenarioFixture("s1", "Updated"))
	if err != nil {
		t.Fatalf("UpdateScenario error: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/api/scenarios/s1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if updated.Name != "Updated" {
		t.Fatalf("unexpected scenario: %#v", updated)
	}
}

func TestUpdateScenarioError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).UpdateScenario("s1", testScenarioFixture("s1", "Updated")); err == nil {
		t.Fatal("expected error for UpdateScenario")
	}
}

func TestDeleteScenario(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).DeleteScenario("s1"); err != nil {
		t.Fatalf("DeleteScenario error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/scenarios/s1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
}

func TestDeleteScenarioError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).DeleteScenario("s1"); err == nil {
		t.Fatal("expected error for DeleteScenario")
	}
}

func TestListActiveScenarios(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"active":["s1"],"scenarios":[{"id":"s1","name":"Test","patches":[]}]}`))
	}))
	defer ts.Close()
	active, err := newTestServer(t, ts.URL).ListActiveScenarios()
	if err != nil {
		t.Fatalf("ListActiveScenarios error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/scenarios/active" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if len(active.Active) != 1 || active.Active[0] != "s1" || len(active.Scenarios) != 1 {
		t.Fatalf("unexpected active scenarios: %#v", active)
	}
}

func TestListActiveScenariosError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).ListActiveScenarios(); err == nil {
		t.Fatal("expected error for ListActiveScenarios")
	}
}

func TestGetCalls(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mock_id":"m1","count":2,"calls":[{"id":"c1","timestamp":"2026-01-01T00:00:00Z","protocol":"http","method":"GET","path":"/ping","status":200,"duration_ms":5}]}`))
	}))
	defer ts.Close()
	summary, err := newTestServer(t, ts.URL).GetCalls("m1")
	if err != nil {
		t.Fatalf("GetCalls error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/calls/http/m1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if summary.MockID != "m1" || summary.Count != 2 || len(summary.Calls) != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestGetCallsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).GetCalls("m1"); err == nil {
		t.Fatal("expected error for GetCalls")
	}
}

func TestClearCalls(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).ClearCalls("m1"); err != nil {
		t.Fatalf("ClearCalls error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/calls/http/m1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
}

func TestClearCallsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).ClearCalls("m1"); err == nil {
		t.Fatal("expected error for ClearCalls")
	}
}

func TestClearAllCalls(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).ClearAllCalls(); err != nil {
		t.Fatalf("ClearAllCalls error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/calls/http" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
}

func TestClearAllCallsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer ts.Close()
	if err := newTestServer(t, ts.URL).ClearAllCalls(); err == nil {
		t.Fatal("expected error for ClearAllCalls")
	}
}

func TestWaitForCalls(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mock_id":"m1","count":2,"calls":[{"id":"c1","timestamp":"2026-01-01T00:00:00Z","protocol":"http","method":"GET","path":"/ping","status":200,"duration_ms":5}]}`))
	}))
	defer ts.Close()
	summary, err := newTestServer(t, ts.URL).WaitForCalls("m1", 2, 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForCalls error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/calls/http/m1/wait" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal wait body: %v", err)
	}
	if int(payload["count"].(float64)) != 2 || payload["timeout"] != "5s" {
		t.Fatalf("unexpected wait payload: %#v", payload)
	}
	if summary.Count != 2 || summary.MockID != "m1" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestWaitForCallsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestTimeout)
		_, _ = w.Write([]byte("timed out"))
	}))
	defer ts.Close()
	if _, err := newTestServer(t, ts.URL).WaitForCalls("m1", 2, 5*time.Second); err == nil {
		t.Fatal("expected error for WaitForCalls")
	}
}

func TestSleep(t *testing.T) {
	start := time.Now()
	sleep(10 * time.Millisecond)
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Errorf("sleep(10ms) returned too quickly: elapsed %v", elapsed)
	}
}
