package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// /api/health
// ---------------------------------------------------------------------------

func TestAPI_Health(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, err := http.Get(base + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/fault GET
// ---------------------------------------------------------------------------

func TestAPI_Fault_Get(t *testing.T) {
	base, _, _, sc, _ := startAPI(t)

	// No fault set yet — response should still be 200.
	resp, err := http.Get(base + "/api/fault")
	if err != nil {
		t.Fatalf("GET /api/fault: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	// Set a fault and retrieve it.
	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})
	resp2, err := http.Get(base + "/api/fault")
	if err != nil {
		t.Fatalf("GET /api/fault after set: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("want 200 after fault set, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/state DELETE key
// ---------------------------------------------------------------------------

func TestAPI_State_DeleteKey(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	// Set a key.
	body := `{"del-key":"to-be-deleted"}`
	resp, _ := http.Post(base+"/api/state", "application/json", bytes.NewBufferString(body))
	_ = resp.Body.Close()

	// Delete it.
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/state/del-key", nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/state/del-key: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("delete state key: want 200, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/scenarios – list, get, update, delete, active, deactivate alias
// ---------------------------------------------------------------------------

func TestAPI_Scenarios_List(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Get(base + "/api/scenarios")
	if err != nil {
		t.Fatalf("GET /api/scenarios: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Scenarios_GetSingle(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	// Create one first.
	payload := map[string]interface{}{"id": "sc-get", "name": "GetTest"}
	body, _ := json.Marshal(payload)
	resp, _ := http.Post(base+"/api/scenarios", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	resp2, err := http.Get(base + "/api/scenarios/sc-get")
	if err != nil {
		t.Fatalf("GET /api/scenarios/sc-get: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_Scenarios_GetSingle_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Get(base + "/api/scenarios/does-not-exist")
	if err != nil {
		t.Fatalf("GET /api/scenarios/does-not-exist: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_Scenarios_Update(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	// Create.
	payload := map[string]interface{}{"id": "sc-upd", "name": "Before"}
	body, _ := json.Marshal(payload)
	resp, _ := http.Post(base+"/api/scenarios", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	// Update.
	updated := map[string]interface{}{"id": "sc-upd", "name": "After"}
	body2, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/scenarios/sc-upd", bytes.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/scenarios/sc-upd: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("update scenario: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_Scenarios_Delete(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	// Create.
	payload := map[string]interface{}{"id": "sc-del", "name": "ToDelete"}
	body, _ := json.Marshal(payload)
	resp, _ := http.Post(base+"/api/scenarios", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	// Delete.
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/scenarios/sc-del", nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/scenarios/sc-del: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("delete scenario: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_Scenarios_Active_List(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Get(base + "/api/scenarios/active")
	if err != nil {
		t.Fatalf("GET /api/scenarios/active: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Scenarios_DeactivateAlias(t *testing.T) {
	base, _, _, sc, _ := startAPI(t)

	// Create + activate.
	payload := map[string]interface{}{"id": "sc-deact", "name": "DeactivateAlias"}
	body, _ := json.Marshal(payload)
	resp, _ := http.Post(base+"/api/scenarios", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()
	resp2, _ := http.Post(base+"/api/scenarios/sc-deact/activate", "application/json", nil)
	_ = resp2.Body.Close()

	if !isActive(sc, "sc-deact") {
		t.Fatal("scenario should be active before deactivate test")
	}

	// Deactivate via POST alias.
	resp3, err := http.Post(base+"/api/scenarios/sc-deact/deactivate", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /deactivate: %v", err)
	}
	defer resp3.Body.Close() //nolint:errcheck
	if resp3.StatusCode != 200 {
		t.Errorf("deactivate alias: want 200, got %d", resp3.StatusCode)
	}
	if isActive(sc, "sc-deact") {
		t.Error("scenario should not be active after POST /deactivate")
	}
}

// ---------------------------------------------------------------------------
// /api/logs DELETE + clear
// ---------------------------------------------------------------------------

func TestAPI_Logs_Clear(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/logs: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("DELETE /api/logs: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Logs_PostThenClearFlow(t *testing.T) {
	base, _, _, _, _, log := startAPIWithLogger(t)

	// Seed a log entry to simulate an HTTP request being processed.
	log.Log(logger.Entry{Protocol: "http", Method: "POST", Path: "/users", Status: 201})

	// GET /api/logs — must be non-empty.
	resp, err := http.Get(base + "/api/logs")
	if err != nil {
		t.Fatalf("GET /api/logs: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Fatalf("GET /api/logs: want 200, got %d", resp.StatusCode)
	}
	var entries []map[string]interface{}
	mustDecodeJSON(t, resp.Body, &entries)
	if len(entries) == 0 {
		t.Fatal("GET /api/logs: expected at least one entry after logging, got none")
	}

	// DELETE /api/logs — clear all logs.
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/logs", nil)
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/logs: %v", err)
	}
	defer delResp.Body.Close() //nolint:errcheck
	if delResp.StatusCode != 200 {
		t.Fatalf("DELETE /api/logs: want 200, got %d", delResp.StatusCode)
	}

	// GET /api/logs — must now be empty.
	resp2, err := http.Get(base + "/api/logs")
	if err != nil {
		t.Fatalf("GET /api/logs after clear: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	var afterClear []map[string]interface{}
	mustDecodeJSON(t, resp2.Body, &afterClear)
	if len(afterClear) != 0 {
		t.Errorf("GET /api/logs after clear: expected empty, got %d entries", len(afterClear))
	}
}

// ---------------------------------------------------------------------------
// /api/logs?matched_id filtering
// ---------------------------------------------------------------------------

func TestAPI_Logs_FilterByMatchedID(t *testing.T) {
	base, _, _, _, _, log := startAPIWithLogger(t)

	log.Log(logger.Entry{Protocol: "http", Path: "/a", MatchedID: "mock-a"})
	log.Log(logger.Entry{Protocol: "http", Path: "/b", MatchedID: "mock-b"})
	log.Log(logger.Entry{Protocol: "http", Path: "/a2", MatchedID: "mock-a"})

	resp, err := http.Get(base + "/api/logs?matched_id=mock-a")
	if err != nil {
		t.Fatalf("GET /api/logs?matched_id=mock-a: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	var entries []map[string]interface{}
	mustDecodeJSON(t, resp.Body, &entries)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for mock-a, got %d", len(entries))
	}
	for _, e := range entries {
		if e["matched_id"] != "mock-a" {
			t.Errorf("unexpected matched_id %q", e["matched_id"])
		}
	}
}

func TestAPI_Logs_FilterByMatchedID_NoMatch(t *testing.T) {
	base, _, _, _, _, log := startAPIWithLogger(t)
	log.Log(logger.Entry{Protocol: "http", Path: "/x", MatchedID: "other"})

	resp, err := http.Get(base + "/api/logs?matched_id=nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	var entries []map[string]interface{}
	mustDecodeJSON(t, resp.Body, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d entries", len(entries))
	}
}

// ---------------------------------------------------------------------------
// /api/logs/count
// ---------------------------------------------------------------------------

func TestAPI_Logs_Count(t *testing.T) {
	base, _, _, _, _, log := startAPIWithLogger(t)

	log.Log(logger.Entry{Protocol: "http", Path: "/a", MatchedID: "mock-x"})
	log.Log(logger.Entry{Protocol: "http", Path: "/b", MatchedID: "mock-y"})
	log.Log(logger.Entry{Protocol: "http", Path: "/c", MatchedID: "mock-x"})

	// Total count (no filter).
	resp, err := http.Get(base + "/api/logs/count")
	if err != nil {
		t.Fatalf("GET /api/logs/count: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	var total map[string]int
	mustDecodeJSON(t, resp.Body, &total)
	if total["count"] != 3 {
		t.Errorf("expected total count=3, got %d", total["count"])
	}

	// Filtered count.
	resp2, err := http.Get(base + "/api/logs/count?matched_id=mock-x")
	if err != nil {
		t.Fatalf("GET /api/logs/count?matched_id=mock-x: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	var filtered map[string]int
	mustDecodeJSON(t, resp2.Body, &filtered)
	if filtered["count"] != 2 {
		t.Errorf("expected filtered count=2, got %d", filtered["count"])
	}
}

func TestAPI_Logs_Count_WithMatchedID_NoMatch(t *testing.T) {
	base, _, _, _, _, log := startAPIWithLogger(t)

	log.Log(logger.Entry{Protocol: "http", Path: "/a", MatchedID: "mock-a"})

	resp, err := http.Get(base + "/api/logs/count?matched_id=missing")
	if err != nil {
		t.Fatalf("GET /api/logs/count?matched_id=missing: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var got map[string]int
	mustDecodeJSON(t, resp.Body, &got)
	if got["count"] != 0 {
		t.Errorf("expected count=0, got %d", got["count"])
	}
}

func TestAPI_Logs_PathParams_InEntry(t *testing.T) {
	base, _, _, _, _, log := startAPIWithLogger(t)
	httpBase := startLoggedHTTPServer(t, []config.HTTPMock{{
		ID:       "thing-by-id",
		Request:  config.HTTPRequest{Method: "GET", Path: "/things/{id}"},
		Response: config.HTTPResponse{Status: 200, Body: `{"ok":true}`},
	}}, log)

	resp, err := http.Get(httpBase + "/things/abc123")
	if err != nil {
		t.Fatalf("GET /things/abc123: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	logsResp, err := http.Get(base + "/api/logs")
	if err != nil {
		t.Fatalf("GET /api/logs: %v", err)
	}
	defer logsResp.Body.Close() //nolint:errcheck

	var entries []map[string]interface{}
	mustDecodeJSON(t, logsResp.Body, &entries)
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	for _, entry := range entries {
		if entry["matched_id"] != "thing-by-id" {
			continue
		}
		params, ok := entry["path_params"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected path_params object, got %#v", entry["path_params"])
		}
		if params["id"] != "abc123" {
			t.Errorf("want path_params.id=abc123, got %#v", params["id"])
		}
		return
	}

	t.Fatal("expected log entry for matched mock thing-by-id")
}

func startLoggedHTTPServer(t *testing.T, mocks []config.HTTPMock, log *logger.Logger) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.HTTPConfig{
		Enabled: true,
		Port:    port,
		Mocks:   mocks,
	}
	srv := httpserver.New(cfg, state.New(), scenarios.New(nil), log)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base+"/", 2*time.Second)
	return base
}

// ---------------------------------------------------------------------------
// /api/calls/http – call counts
// ---------------------------------------------------------------------------

func TestAPI_Calls_HTTP_Get(t *testing.T) {
	base, httpStub, _, _, _ := startAPI(t)

	// Seed a mock.
	httpStub.SetMocks([]config.HTTPMock{{ID: "call-mock"}})

	resp, err := http.Get(base + "/api/calls/http/call-mock")
	if err != nil {
		t.Fatalf("GET /api/calls/http/call-mock: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Calls_HTTP_DeleteSingle(t *testing.T) {
	base, httpStub, _, _, _ := startAPI(t)
	httpStub.SetMocks([]config.HTTPMock{{ID: "call-del"}})

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/calls/http/call-del", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/calls/http/call-del: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Calls_HTTP_DeleteAll(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/calls/http", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/calls/http: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/mocks/graphql CRUD
// ---------------------------------------------------------------------------

func TestAPI_GraphQL_CRUD(t *testing.T) {
	base, _, graphqlStub, _, _ := startAPI(t)

	// Create.
	mock := map[string]interface{}{
		"id":             "gql-mock",
		"operation_type": "query",
		"operation_name": "GetUser",
		"response":       map[string]interface{}{"user": map[string]interface{}{"id": "1"}},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/graphql", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/graphql: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create graphql mock: want 201, got %d", resp.StatusCode)
	}
	if len(graphqlStub.GetMocks()) != 1 {
		t.Fatalf("expected 1 graphql mock, got %d", len(graphqlStub.GetMocks()))
	}

	// List.
	resp2, err := http.Get(base + "/api/mocks/graphql")
	if err != nil {
		t.Fatalf("GET /api/mocks/graphql: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("list graphql mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	updated := map[string]interface{}{
		"id":             "gql-mock",
		"operation_type": "query",
		"operation_name": "GetUserV2",
	}
	body2, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/graphql/gql-mock", bytes.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update graphql mock: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/graphql/gql-mock", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete graphql mock: want 200, got %d", resp4.StatusCode)
	}
	if len(graphqlStub.GetMocks()) != 0 {
		t.Errorf("expected 0 graphql mocks after delete, got %d", len(graphqlStub.GetMocks()))
	}
}

// ---------------------------------------------------------------------------
// /api/mocks/websocket CRUD
// ---------------------------------------------------------------------------

func TestAPI_WebSocket_CRUD(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{
		"id":       "ws-mock",
		"trigger":  map[string]interface{}{"type": "message", "value": "ping"},
		"response": map[string]interface{}{"body": "pong"},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/websocket", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/websocket: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create ws mock: want 201, got %d", resp.StatusCode)
	}

	resp2, _ := http.Get(base + "/api/mocks/websocket")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list ws mocks: want 200, got %d", resp2.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/websocket/ws-mock", nil)
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("delete ws mock: want 200, got %d", resp3.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/mocks/grpc CRUD
// ---------------------------------------------------------------------------

func TestAPI_GRPC_CRUD(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{
		"id":       "grpc-mock",
		"service":  "UserService",
		"method":   "GetUser",
		"response": map[string]interface{}{"id": "1"},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/grpc", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/grpc: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create grpc mock: want 201, got %d", resp.StatusCode)
	}

	resp2, _ := http.Get(base + "/api/mocks/grpc")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list grpc mocks: want 200, got %d", resp2.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/grpc/grpc-mock", nil)
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("delete grpc mock: want 200, got %d", resp3.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/mocks/tcp CRUD
// ---------------------------------------------------------------------------

func TestAPI_TCP_CRUD(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{
		"id":       "tcp-mock",
		"match":    "PING",
		"response": "PONG",
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/tcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/tcp: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create tcp mock: want 201, got %d", resp.StatusCode)
	}

	resp2, _ := http.Get(base + "/api/mocks/tcp")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list tcp mocks: want 200, got %d", resp2.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/tcp/tcp-mock", nil)
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("delete tcp mock: want 200, got %d", resp3.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/mocks/redis CRUD
// ---------------------------------------------------------------------------

func TestAPI_Redis_CRUD(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{
		"id":       "redis-mock",
		"command":  "GET",
		"key":      "mykey",
		"response": map[string]interface{}{"type": "string", "value": "myvalue"},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/redis", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/redis: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create redis mock: want 201, got %d", resp.StatusCode)
	}

	resp2, _ := http.Get(base + "/api/mocks/redis")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list redis mocks: want 200, got %d", resp2.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/redis/redis-mock", nil)
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("delete redis mock: want 200, got %d", resp3.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/mocks/smtp CRUD + /api/emails
// ---------------------------------------------------------------------------

func TestAPI_SMTP_Rules_CRUD(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	rule := map[string]interface{}{
		"id":     "smtp-rule",
		"from":   "spam@*",
		"action": "reject",
	}
	body, _ := json.Marshal(rule)
	resp, err := http.Post(base+"/api/mocks/smtp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/smtp: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create smtp rule: want 201, got %d", resp.StatusCode)
	}

	resp2, _ := http.Get(base + "/api/mocks/smtp")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list smtp rules: want 200, got %d", resp2.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/smtp/smtp-rule", nil)
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("delete smtp rule: want 200, got %d", resp3.StatusCode)
	}
}

func TestAPI_Emails_ListAndClear(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Get(base + "/api/emails")
	if err != nil {
		t.Fatalf("GET /api/emails: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/emails: want 200, got %d", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/emails", nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/emails: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("DELETE /api/emails: want 200, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/mocks/mqtt CRUD + /api/mqtt/messages
// ---------------------------------------------------------------------------

func TestAPI_MQTT_Mocks_CRUD(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{
		"id":               "mqtt-mock",
		"topic":            "sensors/#",
		"response_topic":   "sensors/response",
		"response_payload": "OK",
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/mqtt", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/mqtt: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create mqtt mock: want 201, got %d", resp.StatusCode)
	}

	resp2, _ := http.Get(base + "/api/mocks/mqtt")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list mqtt mocks: want 200, got %d", resp2.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/mqtt/mqtt-mock", nil)
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("delete mqtt mock: want 200, got %d", resp3.StatusCode)
	}
}

func TestAPI_MQTT_Messages_ListAndClear(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Get(base + "/api/mqtt/messages")
	if err != nil {
		t.Fatalf("GET /api/mqtt/messages: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/mqtt/messages: want 200, got %d", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/mqtt/messages", nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/mqtt/messages: %v", err)
	}
	defer resp2.Body.Close() //nolint:errcheck
	if resp2.StatusCode != 200 {
		t.Errorf("DELETE /api/mqtt/messages: want 200, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Error paths – invalid JSON bodies
// ---------------------------------------------------------------------------

func TestAPI_HTTP_Mock_InvalidJSON(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Post(base+"/api/mocks/http", "application/json", bytes.NewBufferString("{invalid}"))
	if err != nil {
		t.Fatalf("POST invalid JSON: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 400 {
		t.Errorf("invalid JSON: want 400, got %d", resp.StatusCode)
	}
}

func TestAPI_Scenario_InvalidJSON(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Post(base+"/api/scenarios", "application/json", bytes.NewBufferString("{bad}"))
	if err != nil {
		t.Fatalf("POST invalid scenario JSON: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 400 {
		t.Errorf("invalid JSON: want 400, got %d", resp.StatusCode)
	}
}

func TestAPI_Fault_InvalidJSON(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	resp, err := http.Post(base+"/api/fault/http", "application/json", bytes.NewBufferString("{bad}"))
	if err != nil {
		t.Fatalf("POST invalid fault JSON: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 400 {
		t.Errorf("invalid JSON fault: want 400, got %d", resp.StatusCode)
	}
}
