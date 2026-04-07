// Integration tests for the GraphQL HTTP handler.
// Uses httptest to drive the handler without spinning up a real TCP listener.
package graphqlserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// newTestServer returns a minimal Server suitable for handler tests.
func newTestServer(mocks []config.GraphQLMock) *Server {
	cfg := &config.GraphQLConfig{
		Enabled: true,
		Port:    0,
		Path:    "/graphql",
	}
	cfg.Mocks = append([]config.GraphQLMock(nil), mocks...)
	return New(cfg, state.New(), scenarios.New(nil), logger.New(10))
}

// doRequest sends a JSON POST to the handleGraphQL handler.
func doRequest(t *testing.T, srv *Server, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handleGraphQL(rr, req)
	return rr
}

func TestHandleGraphQL_MatchedQuery(t *testing.T) {
	mocks := []config.GraphQLMock{
		{
			ID:            "m1",
			OperationType: "query",
			OperationName: "GetUser",
			Response:      map[string]interface{}{"user": map[string]interface{}{"id": "42"}},
		},
	}
	srv := newTestServer(mocks)

	rr := doRequest(t, srv, map[string]interface{}{
		"query":         "query GetUser { user { id } }",
		"operationName": "GetUser",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	if resp["data"] == nil {
		t.Error("expected non-nil data field")
	}
}

func TestHandleGraphQL_NoMatch(t *testing.T) {
	srv := newTestServer(nil)

	rr := doRequest(t, srv, map[string]interface{}{
		"query":         "query UnknownOp { foo }",
		"operationName": "UnknownOp",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("want 200 (GraphQL always returns 200), got %d", rr.Code)
	}
	body := rr.Body.String()
	if !bytes.Contains([]byte(body), []byte("errors")) {
		t.Errorf("expected errors field in body: %s", body)
	}
}

func TestHandleGraphQL_Introspection(t *testing.T) {
	srv := newTestServer(nil)

	rr := doRequest(t, srv, map[string]interface{}{
		"query":         "query IntrospectionQuery { __schema { queryType { name } } }",
		"operationName": "IntrospectionQuery",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("__schema")) {
		t.Error("introspection response should contain __schema")
	}
}

func TestHandleGraphQL_InvalidJSON(t *testing.T) {
	srv := newTestServer(nil)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString("{bad json}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handleGraphQL(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid JSON, got %d", rr.Code)
	}
}

func TestHandleGraphQL_GETRequest(t *testing.T) {
	mocks := []config.GraphQLMock{
		{ID: "m1", OperationName: "GetUser", Response: map[string]interface{}{"user": "alice"}},
	}
	srv := newTestServer(mocks)

	req := httptest.NewRequest(http.MethodGet, "/graphql?query=query+GetUser+%7B+user+%7D&operationName=GetUser", nil)
	rr := httptest.NewRecorder()
	srv.handleGraphQL(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
}

func TestHandleGraphQL_Mutation(t *testing.T) {
	mocks := []config.GraphQLMock{
		{ID: "m1", OperationType: "mutation", OperationName: "CreateUser",
			Response: map[string]interface{}{"id": "new-user"}},
	}
	srv := newTestServer(mocks)

	rr := doRequest(t, srv, map[string]interface{}{
		"query":         "mutation CreateUser { createUser { id } }",
		"operationName": "CreateUser",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	var resp map[string]interface{}
	_ = json.Unmarshal(body, &resp)
	if resp["data"] == nil {
		t.Error("expected data in mutation response")
	}
}

func TestHandleGraphQL_FaultStatusOverride(t *testing.T) {
	mocks := []config.GraphQLMock{
		{ID: "m1", OperationName: "GetUser", Response: map[string]interface{}{"user": "alice"}},
	}
	srv := newTestServer(mocks)

	// Enable a fault that overrides status to 503 with 100% error rate.
	srv.scenarios.SetFault(&config.GlobalFault{
		Enabled:        true,
		StatusOverride: http.StatusServiceUnavailable,
		ErrorRate:      1.0,
	})

	rr := doRequest(t, srv, map[string]interface{}{
		"query":         "query GetUser { user { id } }",
		"operationName": "GetUser",
	})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 from fault, got %d", rr.Code)
	}
}

func TestHandleGraphQL_MockWithErrors(t *testing.T) {
	mocks := []config.GraphQLMock{
		{
			ID:            "m1",
			OperationName: "BrokenQuery",
			Response: nil,
			Errors: []config.GraphQLError{
				{Message: "something went wrong"},
			},
		},
	}
	srv := newTestServer(mocks)

	rr := doRequest(t, srv, map[string]interface{}{
		"query":         "query BrokenQuery { broken }",
		"operationName": "BrokenQuery",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["errors"] == nil {
		t.Error("expected errors field in response")
	}
}
