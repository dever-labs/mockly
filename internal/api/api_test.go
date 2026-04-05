package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/api"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// Stub protocol implementations
// ---------------------------------------------------------------------------

type stubHTTP struct {
	mocks      []config.HTTPMock
	callCounts map[string]int64
}

func (s *stubHTTP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "http", "enabled": true}
}
func (s *stubHTTP) GetMocks() []config.HTTPMock  { return s.mocks }
func (s *stubHTTP) SetMocks(m []config.HTTPMock) { s.mocks = m; s.callCounts = nil }
func (s *stubHTTP) CallCount(mockID string) int64 {
	if s.callCounts == nil {
		return 0
	}
	return s.callCounts[mockID]
}
func (s *stubHTTP) ResetCallCounts() { s.callCounts = nil }

type stubWS struct{}

func (s *stubWS) StatusInfo() map[string]interface{}       { return map[string]interface{}{"protocol": "ws"} }
func (s *stubWS) GetMocks() []config.WebSocketMock         { return nil }
func (s *stubWS) SetMocks(m []config.WebSocketMock)        {}

type stubGRPC struct{}

func (s *stubGRPC) StatusInfo() map[string]interface{} { return map[string]interface{}{"protocol": "grpc"} }
func (s *stubGRPC) GetMocks() []config.GRPCMock        { return nil }
func (s *stubGRPC) SetMocks(m []config.GRPCMock)       {}

type stubGraphQL struct {
	mocks []config.GraphQLMock
}

func (s *stubGraphQL) StatusInfo() map[string]interface{}     { return map[string]interface{}{"protocol": "graphql"} }
func (s *stubGraphQL) GetMocks() []config.GraphQLMock         { return s.mocks }
func (s *stubGraphQL) SetMocks(m []config.GraphQLMock)        { s.mocks = m }

type stubTCP struct{}

func (s *stubTCP) StatusInfo() map[string]interface{} { return map[string]interface{}{"protocol": "tcp"} }
func (s *stubTCP) GetMocks() []config.TCPMock         { return nil }
func (s *stubTCP) SetMocks(m []config.TCPMock)        {}

type stubRedis struct{}

func (s *stubRedis) StatusInfo() map[string]interface{} { return map[string]interface{}{"protocol": "redis"} }
func (s *stubRedis) GetMocks() []config.RedisMock       { return nil }
func (s *stubRedis) SetMocks(m []config.RedisMock)      {}

type stubSMTP struct {
	inbox *smtpserver.Inbox
	rules []config.SMTPRule
}

func (s *stubSMTP) StatusInfo() map[string]interface{} { return map[string]interface{}{"protocol": "smtp"} }
func (s *stubSMTP) GetRules() []config.SMTPRule        { return s.rules }
func (s *stubSMTP) SetRules(r []config.SMTPRule)       { s.rules = r }
func (s *stubSMTP) GetInbox() *smtpserver.Inbox        { return s.inbox }

type stubMQTT struct {
	ms *mqttserver.MessageStore
}

func (s *stubMQTT) StatusInfo() map[string]interface{}    { return map[string]interface{}{"protocol": "mqtt"} }
func (s *stubMQTT) GetMocks() []config.MQTTMock           { return nil }
func (s *stubMQTT) SetMocks(m []config.MQTTMock)          {}
func (s *stubMQTT) GetMessageStore() *mqttserver.MessageStore { return s.ms }

// ---------------------------------------------------------------------------
// Helper: start an API server on a free port
// ---------------------------------------------------------------------------

func startAPI(t *testing.T) (string, *stubHTTP, *stubGraphQL, *scenarios.Store) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg := &config.Config{}
	cfg.Mockly.API.Port = port

	sc := scenarios.New(nil)
	store := state.New()
	log := logger.New(100)

	httpStub := &stubHTTP{}
	graphqlStub := &stubGraphQL{}
	smtpStub := &stubSMTP{inbox: smtpserver.NewInbox(50)}
	mqttStub := &stubMQTT{ms: mqttserver.NewMessageStore(50)}

	srv := api.New(
		cfg, store, sc, log,
		httpStub,
		&stubWS{},
		&stubGRPC{},
		graphqlStub,
		&stubTCP{},
		&stubRedis{},
		smtpStub,
		mqttStub,
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base+"/api/protocols", 2*time.Second)
	return base, httpStub, graphqlStub, sc
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAPI_GetProtocols(t *testing.T) {
	base, _, _, _ := startAPI(t)

	resp, err := http.Get(base + "/api/protocols")
	if err != nil {
		t.Fatalf("GET /api/protocols: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var body []map[string]interface{}
	mustDecodeJSON(t, resp.Body, &body)
	if len(body) == 0 {
		t.Error("expected at least one protocol in the list")
	}
}

func TestAPI_HTTP_CRUD(t *testing.T) {
	base, httpStub, _, _ := startAPI(t)

	// Create a mock.
	mock := map[string]interface{}{
		"id":       "test-mock",
		"request":  map[string]interface{}{"method": "GET", "path": "/test"},
		"response": map[string]interface{}{"status": 200, "body": `{"ok":true}`},
	}
	postBody, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/http", "application/json", bytes.NewReader(postBody))
	if err != nil {
		t.Fatalf("POST /api/mocks/http: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create: want 201, got %d", resp.StatusCode)
	}
	if len(httpStub.GetMocks()) != 1 {
		t.Fatalf("expected 1 mock after create, got %d", len(httpStub.GetMocks()))
	}

	// List mocks.
	resp2, _ := http.Get(base + "/api/mocks/http")
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list: want 200, got %d", resp2.StatusCode)
	}
	var mocks []config.HTTPMock
	mustDecodeJSON(t, resp2.Body, &mocks)
	if len(mocks) != 1 || mocks[0].ID != "test-mock" {
		t.Errorf("unexpected mocks list: %+v", mocks)
	}

	// Update mock.
	updated := mock
	updated["response"] = map[string]interface{}{"status": 404, "body": "not found"}
	putBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/http/test-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update: want 200, got %d", resp3.StatusCode)
	}

	// Delete mock.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/http/test-mock", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete: want 200, got %d", resp4.StatusCode)
	}
	if len(httpStub.GetMocks()) != 0 {
		t.Errorf("expected 0 mocks after delete, got %d", len(httpStub.GetMocks()))
	}
}

func TestAPI_Scenarios_CRUD(t *testing.T) {
	base, _, _, sc := startAPI(t)

	// Create scenario.
	payload := map[string]interface{}{
		"id":   "s1",
		"name": "Test Scenario",
		"patches": []map[string]interface{}{
			{"mock_id": "m1", "status": 503},
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(base+"/api/scenarios", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/scenarios: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create scenario: want 201, got %d", resp.StatusCode)
	}

	// Activate scenario.
	resp2, _ := http.Post(base+"/api/scenarios/s1/activate", "application/json", nil)
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("activate: want 200, got %d", resp2.StatusCode)
	}
	if !isActive(sc, "s1") {
		t.Error("scenario s1 should be active")
	}

	// Deactivate scenario — uses DELETE /api/scenarios/{id}/activate
	req3, _ := http.NewRequest(http.MethodDelete, base+"/api/scenarios/s1/activate", nil)
	resp3, _ := http.DefaultClient.Do(req3)
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("deactivate: want 200, got %d", resp3.StatusCode)
	}
	if isActive(sc, "s1") {
		t.Error("scenario s1 should be inactive after deactivate")
	}
}

func TestAPI_Fault_SetClear(t *testing.T) {
	base, _, _, sc := startAPI(t)

	// Set fault.
	fault := map[string]interface{}{
		"enabled":         true,
		"status_override": 503,
		"error_rate":      1.0,
	}
	body, _ := json.Marshal(fault)
	resp, _ := http.Post(base+"/api/fault", "application/json", bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("set fault: want 200, got %d", resp.StatusCode)
	}
	if f := sc.GetFault(); f == nil || !f.Enabled || f.StatusOverride != 503 {
		t.Errorf("fault not set correctly: %+v", f)
	}

	// Clear fault.
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/fault", nil)
	resp2, _ := http.DefaultClient.Do(req)
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("clear fault: want 200, got %d", resp2.StatusCode)
	}
	if sc.GetFault() != nil {
		t.Error("fault should be nil after clear")
	}
}

func TestAPI_Reset(t *testing.T) {
	base, _, _, sc := startAPI(t)

	// Set some state.
	sc.SetFault(&config.GlobalFault{Enabled: true, StatusOverride: 500})

	// Reset.
	resp, _ := http.Post(base+"/api/reset", "application/json", nil)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("reset: want 200, got %d", resp.StatusCode)
	}
	if sc.GetFault() != nil {
		t.Error("fault should be cleared after reset")
	}
}

func TestAPI_State_SetGet(t *testing.T) {
	base, _, _, _ := startAPI(t)

	// Set state key.
	body := `{"my-key":"test-val"}`
	resp, _ := http.Post(base+"/api/state", "application/json", strings.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("set state: want 200, got %d", resp.StatusCode)
	}

	// Get all state keys.
	resp2, _ := http.Get(base + "/api/state")
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("get state: want 200, got %d", resp2.StatusCode)
	}
	var result map[string]string
	mustDecodeJSON(t, resp2.Body, &result)
	if result["my-key"] != "test-val" {
		t.Errorf("want test-val, got %q", result["my-key"])
	}
}

func TestAPI_Logs(t *testing.T) {
	base, _, _, _ := startAPI(t)

	resp, _ := http.Get(base + "/api/logs")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/logs: want 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isActive checks if a scenario ID is in the active set.
func isActive(sc *scenarios.Store, id string) bool {
	for _, a := range sc.ActiveIDs() {
		if a == id {
			return true
		}
	}
	return false
}

func mustDecodeJSON(t *testing.T, r io.Reader, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(v); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}
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
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s within %v", url, timeout)
}
