package testcontainersmockly

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	mocklydriver "github.com/dever-labs/mockly/clients/go"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
)

// fakeContainer is a minimal stand-in for testcontainers.Container that
// only implements Host/MappedPort, pointing both HTTPPort and APIPort at a
// local httptest.Server so MocklyContainer's HTTP methods can be unit
// tested without starting real Docker containers. Embedding the (nil)
// interface satisfies every other method of the (large) Container
// interface; calling any of those would panic, but none of the methods
// under test here call anything beyond Host/MappedPort.
type fakeContainer struct {
	testcontainers.Container
	host string
	port string
}

func (f *fakeContainer) Host(context.Context) (string, error) { return f.host, nil }

func (f *fakeContainer) MappedPort(_ context.Context, _ string) (network.Port, error) {
	return network.ParsePort(f.port + "/tcp")
}

// newTestContainer starts an httptest.Server running handler and returns a
// MocklyContainer whose HTTPBase/APIBase both resolve to it.
func newTestContainer(t *testing.T, handler http.HandlerFunc) (*MocklyContainer, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}

	return &MocklyContainer{Container: &fakeContainer{host: u.Hostname(), port: u.Port()}}, ts
}

func TestHTTPBaseAndAPIBase(t *testing.T) {
	c, ts := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {})

	httpBase, err := c.HTTPBase(context.Background())
	if err != nil {
		t.Fatalf("HTTPBase: %v", err)
	}
	if httpBase != ts.URL {
		t.Errorf("HTTPBase = %q, want %q", httpBase, ts.URL)
	}

	apiBase, err := c.APIBase(context.Background())
	if err != nil {
		t.Fatalf("APIBase: %v", err)
	}
	if apiBase != ts.URL {
		t.Errorf("APIBase = %q, want %q", apiBase, ts.URL)
	}
}

func TestAddMock(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody []byte
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	})

	mock := mocklydriver.Mock{ID: "m1", Request: mocklydriver.MockRequest{Method: "GET", Path: "/ping"}, Response: mocklydriver.MockResponse{Status: 200}}
	if err := c.AddMock(context.Background(), mock); err != nil {
		t.Fatalf("AddMock: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/mocks/http" {
		t.Errorf("got %s %s, want POST /api/mocks/http", gotMethod, gotPath)
	}
	var parsed map[string]any
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("unmarshalling body: %v", err)
	}
	if parsed["id"] != "m1" {
		t.Errorf("body.id = %v, want m1", parsed["id"])
	}
}

func TestAddMockErrorResponse(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	err := c.AddMock(context.Background(), mocklydriver.Mock{ID: "m1"})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error mentioning 500, got: %v", err)
	}
}

func TestListMocks(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/mocks/http" {
			t.Errorf("got %s %s, want GET /api/mocks/http", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]mocklydriver.Mock{{ID: "a"}, {ID: "b"}})
	})

	mocks, err := c.ListMocks(context.Background())
	if err != nil {
		t.Fatalf("ListMocks: %v", err)
	}
	if len(mocks) != 2 || mocks[0].ID != "a" || mocks[1].ID != "b" {
		t.Errorf("ListMocks = %+v", mocks)
	}
}

func TestUpdateMock(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/mocks/http/m1" {
			t.Errorf("got %s %s, want PUT /api/mocks/http/m1", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(mocklydriver.Mock{ID: "m1"})
	})

	updated, err := c.UpdateMock(context.Background(), "m1", mocklydriver.Mock{ID: "m1"})
	if err != nil {
		t.Fatalf("UpdateMock: %v", err)
	}
	if updated.ID != "m1" {
		t.Errorf("UpdateMock = %+v", updated)
	}
}

func TestPatchMock(t *testing.T) {
	status := 201
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/mocks/http/m1" {
			t.Errorf("got %s %s, want PATCH /api/mocks/http/m1", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(mocklydriver.Mock{ID: "m1", Response: mocklydriver.MockResponse{Status: 201}})
	})

	updated, err := c.PatchMock(context.Background(), "m1", mocklydriver.MockResponsePatch{Status: &status})
	if err != nil {
		t.Fatalf("PatchMock: %v", err)
	}
	if updated.Response.Status != 201 {
		t.Errorf("PatchMock = %+v", updated)
	}
}

func TestDeleteMock(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/mocks/http/m1" {
			t.Errorf("got %s %s, want DELETE /api/mocks/http/m1", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := c.DeleteMock(context.Background(), "m1"); err != nil {
		t.Fatalf("DeleteMock: %v", err)
	}
}

func TestGetState(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/state" {
			t.Errorf("got %s %s, want GET /api/state", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"env": "prod"})
	})

	state, err := c.GetState(context.Background())
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state["env"] != "prod" {
		t.Errorf("GetState = %+v", state)
	}
}

func TestSetState(t *testing.T) {
	var gotBody []byte
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/state" {
			t.Errorf("got %s %s, want POST /api/state", r.Method, r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]string{"env": "prod"})
	})

	state, err := c.SetState(context.Background(), map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("SetState: %v", err)
	}
	if state["env"] != "prod" {
		t.Errorf("SetState = %+v", state)
	}
	if !strings.Contains(string(gotBody), "prod") {
		t.Errorf("request body = %s, want it to contain 'prod'", gotBody)
	}
}

func TestDeleteState(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/state/env" {
			t.Errorf("got %s %s, want DELETE /api/state/env", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := c.DeleteState(context.Background(), "env"); err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
}

func TestReset(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/reset" {
			t.Errorf("got %s %s, want POST /api/reset", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := c.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}
}

func TestActivateAndDeactivateScenario(t *testing.T) {
	var paths []string
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})

	if err := c.ActivateScenario(context.Background(), "sc1"); err != nil {
		t.Fatalf("ActivateScenario: %v", err)
	}
	if err := c.DeactivateScenario(context.Background(), "sc1"); err != nil {
		t.Fatalf("DeactivateScenario: %v", err)
	}

	want := []string{"POST /api/scenarios/sc1/activate", "POST /api/scenarios/sc1/deactivate"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Errorf("paths = %v, want %v", paths, want)
	}
}

func TestListScenarios(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/scenarios" {
			t.Errorf("got %s %s, want GET /api/scenarios", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]mocklydriver.Scenario{{ID: "sc1"}})
	})

	scenarios, err := c.ListScenarios(context.Background())
	if err != nil {
		t.Fatalf("ListScenarios: %v", err)
	}
	if len(scenarios) != 1 || scenarios[0].ID != "sc1" {
		t.Errorf("ListScenarios = %+v", scenarios)
	}
}

func TestCreateScenario(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/scenarios" {
			t.Errorf("got %s %s, want POST /api/scenarios", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(mocklydriver.Scenario{ID: "sc1"})
	})

	created, err := c.CreateScenario(context.Background(), mocklydriver.Scenario{ID: "sc1"})
	if err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}
	if created.ID != "sc1" {
		t.Errorf("CreateScenario = %+v", created)
	}
}

func TestGetScenario(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/scenarios/sc1" {
			t.Errorf("got %s %s, want GET /api/scenarios/sc1", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(mocklydriver.Scenario{ID: "sc1"})
	})

	scenario, err := c.GetScenario(context.Background(), "sc1")
	if err != nil {
		t.Fatalf("GetScenario: %v", err)
	}
	if scenario.ID != "sc1" {
		t.Errorf("GetScenario = %+v", scenario)
	}
}

func TestUpdateScenario(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/scenarios/sc1" {
			t.Errorf("got %s %s, want PUT /api/scenarios/sc1", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(mocklydriver.Scenario{ID: "sc1", Name: "updated"})
	})

	updated, err := c.UpdateScenario(context.Background(), "sc1", mocklydriver.Scenario{ID: "sc1", Name: "updated"})
	if err != nil {
		t.Fatalf("UpdateScenario: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("UpdateScenario = %+v", updated)
	}
}

func TestDeleteScenario(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/scenarios/sc1" {
			t.Errorf("got %s %s, want DELETE /api/scenarios/sc1", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := c.DeleteScenario(context.Background(), "sc1"); err != nil {
		t.Fatalf("DeleteScenario: %v", err)
	}
}

func TestListActiveScenarios(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/scenarios/active" {
			t.Errorf("got %s %s, want GET /api/scenarios/active", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(mocklydriver.ActiveScenariosResponse{Active: []string{"sc1"}})
	})

	active, err := c.ListActiveScenarios(context.Background())
	if err != nil {
		t.Fatalf("ListActiveScenarios: %v", err)
	}
	if len(active.Active) != 1 || active.Active[0] != "sc1" {
		t.Errorf("ListActiveScenarios = %+v", active)
	}
}

func TestSetFault(t *testing.T) {
	status := 503
	var gotBody []byte
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/fault/http" {
			t.Errorf("got %s %s, want POST /api/fault/http", r.Method, r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	err := c.SetFault(context.Background(), mocklydriver.FaultConfig{Delay: "10ms", Status: &status, ErrorRate: 0.5})
	if err != nil {
		t.Fatalf("SetFault: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("unmarshalling body: %v", err)
	}
	if parsed["delay"] != "10ms" || parsed["status"] != float64(503) || parsed["error_rate"] != 0.5 {
		t.Errorf("SetFault body = %+v", parsed)
	}
}

func TestClearFault(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/fault" {
			t.Errorf("got %s %s, want DELETE /api/fault", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.ClearFault(context.Background()); err != nil {
		t.Fatalf("ClearFault: %v", err)
	}
}

func TestGetLogs(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/logs" || r.URL.Query().Get("matched_id") != "m1" {
			t.Errorf("got %s %s?%s, want GET /api/logs?matched_id=m1", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]mocklydriver.CallEntry{{ID: "c1"}})
	})

	logs, err := c.GetLogs(context.Background(), "m1")
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != "c1" {
		t.Errorf("GetLogs = %+v", logs)
	}
}

func TestClearLogs(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/logs" {
			t.Errorf("got %s %s, want DELETE /api/logs", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := c.ClearLogs(context.Background()); err != nil {
		t.Fatalf("ClearLogs: %v", err)
	}
}

func TestGetLogsCount(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/logs/count" {
			t.Errorf("got %s %s, want GET /api/logs/count", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"count":7}`))
	})

	count, err := c.GetLogsCount(context.Background(), "")
	if err != nil {
		t.Fatalf("GetLogsCount: %v", err)
	}
	if count != 7 {
		t.Errorf("GetLogsCount = %d, want 7", count)
	}
}

func TestGetCalls(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/calls/http/m1" {
			t.Errorf("got %s %s, want GET /api/calls/http/m1", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(mocklydriver.CallSummary{MockID: "m1", Count: 3})
	})

	summary, err := c.GetCalls(context.Background(), "m1")
	if err != nil {
		t.Fatalf("GetCalls: %v", err)
	}
	if summary.Count != 3 {
		t.Errorf("GetCalls = %+v", summary)
	}
}

func TestClearCalls(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/calls/http/m1" {
			t.Errorf("got %s %s, want DELETE /api/calls/http/m1", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := c.ClearCalls(context.Background(), "m1"); err != nil {
		t.Fatalf("ClearCalls: %v", err)
	}
}

func TestClearAllCalls(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/calls/http" {
			t.Errorf("got %s %s, want DELETE /api/calls/http", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := c.ClearAllCalls(context.Background()); err != nil {
		t.Fatalf("ClearAllCalls: %v", err)
	}
}

func TestWaitForCallsSuccess(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/calls/http/m1/wait" {
			t.Errorf("got %s %s, want POST /api/calls/http/m1/wait", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(mocklydriver.CallSummary{MockID: "m1", Count: 2})
	})

	summary, err := c.WaitForCalls(context.Background(), "m1", 2, time.Second)
	if err != nil {
		t.Fatalf("WaitForCalls: %v", err)
	}
	if summary.Count != 2 {
		t.Errorf("WaitForCalls = %+v", summary)
	}
}

func TestWaitForCallsTimeout(t *testing.T) {
	c, _ := newTestContainer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestTimeout)
		_, _ = w.Write([]byte("timed out"))
	})

	_, err := c.WaitForCalls(context.Background(), "m1", 5, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timeout waiting for") {
		t.Fatalf("expected friendly timeout error, got: %v", err)
	}
}

func TestDoRequestPropagatesAPIBaseError(t *testing.T) {
	c := &MocklyContainer{Container: &fakeContainer{host: "", port: ""}}
	_, err := c.doRequest(context.Background(), http.MethodGet, "/api/state", nil)
	if err == nil {
		t.Fatal("expected an error when the mapped port can't be parsed")
	}
}
