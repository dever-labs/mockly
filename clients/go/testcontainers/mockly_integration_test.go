//go:build integration

package testcontainersmockly

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	mocklydriver "github.com/dever-labs/mockly/clients/go"
)

func TestIntegration_ContainerLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	httpBase, err := c.HTTPBase(ctx)
	if err != nil {
		t.Fatalf("HTTPBase: %v", err)
	}
	apiBase, err := c.APIBase(ctx)
	if err != nil {
		t.Fatalf("APIBase: %v", err)
	}
	t.Logf("http base: %s", httpBase)
	t.Logf("api base: %s", apiBase)

	assertStatus(t, http.MethodGet, apiBase+"/api/protocols", http.StatusOK)

	err = c.AddMock(ctx, mocklydriver.Mock{
		ID: "hello-mock",
		Request: mocklydriver.MockRequest{
			Method: http.MethodGet,
			Path:   "/hello",
		},
		Response: mocklydriver.MockResponse{
			Status: http.StatusOK,
			Body:   "world",
		},
	})
	if err != nil {
		t.Fatalf("AddMock: %v", err)
	}

	resp := mustDoRequest(t, http.MethodGet, httpBase+"/hello")
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /hello response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /hello status = %d, body = %s", resp.StatusCode, string(body))
	}
	if string(body) != "world" {
		t.Fatalf("GET /hello body = %q, want %q", string(body), "world")
	}

	if err := c.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	resp = mustDoRequest(t, http.MethodGet, httpBase+"/hello")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /hello after reset returned 200, body = %s", string(body))
	}
}

func TestIntegration_GetLogs(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	httpBase, err := c.HTTPBase(ctx)
	if err != nil {
		t.Fatalf("HTTPBase: %v", err)
	}

	resp := mustDoRequest(t, http.MethodGet, httpBase+"/some-path")
	defer resp.Body.Close() //nolint:errcheck
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("draining /some-path response: %v", err)
	}

	logs, err := c.GetLogs(ctx, "")
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("GetLogs returned no entries")
	}

	if _, err := json.Marshal(logs); err != nil {
		t.Fatalf("GetLogs returned invalid entries: %v", err)
	}
}

func TestIntegration_WithInlineConfig(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx, WithInlineConfig(defaultConfig))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	apiBase, err := c.APIBase(ctx)
	if err != nil {
		t.Fatalf("APIBase: %v", err)
	}

	assertStatus(t, http.MethodGet, apiBase+"/api/protocols", http.StatusOK)
}

func TestIntegration_ListMocks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	mock := mocklydriver.Mock{
		ID: "list-mock",
		Request: mocklydriver.MockRequest{
			Method: http.MethodGet,
			Path:   "/list-mock",
		},
		Response: mocklydriver.MockResponse{
			Status: http.StatusOK,
			Body:   "listed",
		},
	}

	if err := c.AddMock(ctx, mock); err != nil {
		t.Fatalf("AddMock: %v", err)
	}

	mocks, err := c.ListMocks(ctx)
	if err != nil {
		t.Fatalf("ListMocks: %v", err)
	}
	if len(mocks) == 0 {
		t.Fatal("ListMocks returned no mocks")
	}

	found := false
	for _, gotMock := range mocks {
		if gotMock.ID == mock.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ListMocks did not return mock ID %q: %#v", mock.ID, mocks)
	}
}

func TestIntegration_UpdateAndPatchMock(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	httpBase, err := c.HTTPBase(ctx)
	if err != nil {
		t.Fatalf("HTTPBase: %v", err)
	}

	mock := mocklydriver.Mock{
		ID: "update-patch-mock",
		Request: mocklydriver.MockRequest{
			Method: http.MethodGet,
			Path:   "/update-patch",
		},
		Response: mocklydriver.MockResponse{
			Status: http.StatusOK,
			Body:   "initial",
		},
	}

	if err := c.AddMock(ctx, mock); err != nil {
		t.Fatalf("AddMock: %v", err)
	}

	updatedMock := mocklydriver.Mock{
		ID: mock.ID,
		Request: mock.Request,
		Response: mocklydriver.MockResponse{
			Status: http.StatusOK,
			Body:   "updated",
		},
	}

	if _, err := c.UpdateMock(ctx, mock.ID, updatedMock); err != nil {
		t.Fatalf("UpdateMock: %v", err)
	}

	resp := mustDoRequest(t, http.MethodGet, httpBase+mock.Request.Path)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck
	if err != nil {
		t.Fatalf("reading updated response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d, body = %s", mock.Request.Path, resp.StatusCode, http.StatusOK, string(body))
	}
	if string(body) != "updated" {
		t.Fatalf("GET %s body = %q, want %q", mock.Request.Path, string(body), "updated")
	}

	status := http.StatusTeapot
	patch := mocklydriver.MockResponsePatch{Status: &status}
	if _, err := c.PatchMock(ctx, mock.ID, patch); err != nil {
		t.Fatalf("PatchMock: %v", err)
	}

	resp = mustDoRequest(t, http.MethodGet, httpBase+mock.Request.Path)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck
	if err != nil {
		t.Fatalf("reading patched response: %v", err)
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("GET %s status = %d, want %d, body = %s", mock.Request.Path, resp.StatusCode, http.StatusTeapot, string(body))
	}
}

func TestIntegration_GetState_SetState_DeleteState(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	state, err := c.SetState(ctx, map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("SetState: %v", err)
	}
	if state["key"] != "val" {
		t.Fatalf("SetState returned key = %q, want %q", state["key"], "val")
	}

	state, err = c.GetState(ctx)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state["key"] != "val" {
		t.Fatalf("GetState returned key = %q, want %q", state["key"], "val")
	}

	if err := c.DeleteState(ctx, "key"); err != nil {
		t.Fatalf("DeleteState: %v", err)
	}

	state, err = c.GetState(ctx)
	if err != nil {
		t.Fatalf("GetState after DeleteState: %v", err)
	}
	if _, ok := state["key"]; ok {
		t.Fatalf("GetState after DeleteState still contains key: %#v", state)
	}
}

func TestIntegration_GetLogsCount(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	httpBase, err := c.HTTPBase(ctx)
	if err != nil {
		t.Fatalf("HTTPBase: %v", err)
	}

	resp := mustDoRequest(t, http.MethodGet, httpBase+"/logs-count")
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		resp.Body.Close() //nolint:errcheck
		t.Fatalf("draining /logs-count response: %v", err)
	}
	resp.Body.Close() //nolint:errcheck

	count, err := c.GetLogsCount(ctx, "")
	if err != nil {
		t.Fatalf("GetLogsCount(all): %v", err)
	}
	if count <= 0 {
		t.Fatalf("GetLogsCount(all) = %d, want > 0", count)
	}

	count, err = c.GetLogsCount(ctx, "unknown-mock")
	if err != nil {
		t.Fatalf("GetLogsCount(unknown-mock): %v", err)
	}
	if count < 0 {
		t.Fatalf("GetLogsCount(unknown-mock) = %d, want >= 0", count)
	}
}

func TestIntegration_Scenarios(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	scenarios, err := c.ListScenarios(ctx)
	if err != nil {
		t.Fatalf("ListScenarios initial: %v", err)
	}
	if len(scenarios) != 0 {
		t.Fatalf("ListScenarios initial len = %d, want 0", len(scenarios))
	}

	scenario := mocklydriver.Scenario{
		ID:      "tc-scenario",
		Name:    "TC Scenario",
		Patches: []mocklydriver.ScenarioPatch{},
	}

	if _, err := c.CreateScenario(ctx, scenario); err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}

	gotScenario, err := c.GetScenario(ctx, scenario.ID)
	if err != nil {
		t.Fatalf("GetScenario: %v", err)
	}
	if gotScenario.ID != scenario.ID {
		t.Fatalf("GetScenario ID = %q, want %q", gotScenario.ID, scenario.ID)
	}

	scenarios, err = c.ListScenarios(ctx)
	if err != nil {
		t.Fatalf("ListScenarios after create: %v", err)
	}
	if len(scenarios) == 0 {
		t.Fatal("ListScenarios after create returned no scenarios")
	}

	if err := c.DeleteScenario(ctx, scenario.ID); err != nil {
		t.Fatalf("DeleteScenario: %v", err)
	}

	scenarios, err = c.ListScenarios(ctx)
	if err != nil {
		t.Fatalf("ListScenarios after delete: %v", err)
	}
	if len(scenarios) != 0 {
		t.Fatalf("ListScenarios after delete len = %d, want 0", len(scenarios))
	}
}

func TestIntegration_GetCalls(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	httpBase, err := c.HTTPBase(ctx)
	if err != nil {
		t.Fatalf("HTTPBase: %v", err)
	}

	mock := mocklydriver.Mock{
		ID: "calls-mock",
		Request: mocklydriver.MockRequest{
			Method: http.MethodGet,
			Path:   "/calls",
		},
		Response: mocklydriver.MockResponse{
			Status: http.StatusOK,
			Body:   "calls",
		},
	}

	if err := c.AddMock(ctx, mock); err != nil {
		t.Fatalf("AddMock: %v", err)
	}

	resp := mustDoRequest(t, http.MethodGet, httpBase+mock.Request.Path)
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		resp.Body.Close() //nolint:errcheck
		t.Fatalf("draining %s response: %v", mock.Request.Path, err)
	}
	resp.Body.Close() //nolint:errcheck

	summary, err := c.GetCalls(ctx, mock.ID)
	if err != nil {
		t.Fatalf("GetCalls: %v", err)
	}
	if summary.Count <= 0 {
		t.Fatalf("GetCalls count = %d, want > 0", summary.Count)
	}

	if err := c.ClearCalls(ctx, mock.ID); err != nil {
		t.Fatalf("ClearCalls: %v", err)
	}

	summary, err = c.GetCalls(ctx, mock.ID)
	if err != nil {
		t.Fatalf("GetCalls after ClearCalls: %v", err)
	}
	if summary.Count != 0 {
		t.Fatalf("GetCalls after ClearCalls count = %d, want 0", summary.Count)
	}

	if err := c.ClearAllCalls(ctx); err != nil {
		t.Fatalf("ClearAllCalls: %v", err)
	}
}

func TestIntegration_WaitForCalls(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Logf("Terminate: %v", err)
		}
	})

	httpBase, err := c.HTTPBase(ctx)
	if err != nil {
		t.Fatalf("HTTPBase: %v", err)
	}

	mock := mocklydriver.Mock{
		ID: "wait-mock",
		Request: mocklydriver.MockRequest{
			Method: http.MethodGet,
			Path:   "/wait",
		},
		Response: mocklydriver.MockResponse{
			Status: http.StatusOK,
			Body:   "waited",
		},
	}

	if err := c.AddMock(ctx, mock); err != nil {
		t.Fatalf("AddMock: %v", err)
	}

	reqErrCh := make(chan error, 1)
	go func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, httpBase+mock.Request.Path, nil)
		if err != nil {
			reqErrCh <- err
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			reqErrCh <- err
			return
		}
		defer resp.Body.Close() //nolint:errcheck

		_, err = io.Copy(io.Discard, resp.Body)
		reqErrCh <- err
	}()

	summary, err := c.WaitForCalls(ctx, mock.ID, 1, 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForCalls: %v", err)
	}
	if summary.Count < 1 {
		t.Fatalf("WaitForCalls count = %d, want >= 1", summary.Count)
	}

	if err := <-reqErrCh; err != nil {
		t.Fatalf("draining %s response: %v", mock.Request.Path, err)
	}
}

func assertStatus(t *testing.T, method, url string, want int) {
	t.Helper()

	resp := mustDoRequest(t, method, url)
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s %s response: %v", method, url, err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s status = %d, want %d, body = %s", method, url, resp.StatusCode, want, string(body))
	}
}

func mustDoRequest(t *testing.T, method, url string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		t.Fatalf("creating %s request for %s: %v", method, url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("performing %s request for %s: %v", method, url, err)
	}

	return resp
}
