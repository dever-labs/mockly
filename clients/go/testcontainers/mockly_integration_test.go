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
