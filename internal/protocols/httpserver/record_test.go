package httpserver_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// startRecordingServer starts an HTTP mock server with record mode enabled
// against upstream, optionally persisting captured mocks to saveTo. It
// returns the server's base URL and the Server itself (for inspecting
// GetMocks() directly, since these tests don't wire up the Management API).
func startRecordingServer(t *testing.T, upstream, saveTo string, seed []config.HTTPMock) (string, *httpserver.Server) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.HTTPConfig{
		Enabled: true,
		Port:    port,
		Mocks:   seed,
		Record: &config.HTTPRecordConfig{
			Enabled: true,
			Target:  upstream,
			SaveTo:  saveTo,
		},
	}
	store := state.New()
	log := logger.New(100)
	srv := httpserver.New(cfg, store, scenarios.New(nil), log, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base, 2*time.Second)
	return base, srv
}

func TestHTTPServer_RecordMode_ProxiesAndCaptures(t *testing.T) {
	var upstreamHits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/widgets/42" {
			// Ignore incidental requests (e.g. the test helper's readiness
			// probe against "/") — only the path under test is counted.
			w.WriteHeader(http.StatusOK)
			return
		}
		upstreamHits++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":42,"name":"widget"}`))
	}))
	defer upstream.Close()

	saveTo := filepath.Join(t.TempDir(), "recorded.yaml")
	base, _ := startRecordingServer(t, upstream.URL, saveTo, nil)

	// First call: no mock matches, so it's proxied+captured.
	resp, err := http.Get(base + "/widgets/42")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != `{"id":42,"name":"widget"}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := resp.Header.Get("X-Upstream"); got != "yes" {
		t.Fatalf("expected upstream header to be relayed, got %q", got)
	}

	// Second call: should now be served from the captured mock, not the
	// upstream again.
	resp2, err := http.Get(base + "/widgets/42")
	if err != nil {
		t.Fatalf("second GET failed: %v", err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close() //nolint:errcheck
	if string(body2) != `{"id":42,"name":"widget"}` {
		t.Fatalf("unexpected replayed body: %s", body2)
	}
	if upstreamHits != 1 {
		t.Fatalf("expected exactly 1 upstream hit (second request should replay), got %d", upstreamHits)
	}

	// Verify it was persisted to disk.
	data, err := os.ReadFile(saveTo)
	if err != nil {
		t.Fatalf("expected saveTo file to exist: %v", err)
	}
	if !contains(string(data), "widgets/42") || !contains(string(data), "widget") {
		t.Fatalf("saved recording missing expected content:\n%s", data)
	}
}

func TestHTTPServer_RecordMode_FallsBackTo404WhenUpstreamUnreachable(t *testing.T) {
	// Deliberately unreachable upstream (closed listener).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	unreachable := fmt.Sprintf("http://%s", ln.Addr().String())
	_ = ln.Close()

	base, _ := startRecordingServer(t, unreachable, "", nil)

	resp, err := http.Get(base + "/anything")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 fallback when upstream is unreachable, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_RecordMode_ExistingMockTakesPrecedence(t *testing.T) {
	var upstreamHits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/widgets/42" {
			upstreamHits++
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"should not be used"`))
	}))
	defer upstream.Close()

	seed := []config.HTTPMock{{
		ID:       "hand-written",
		Request:  config.HTTPRequest{Method: "GET", Path: "/widgets/42"},
		Response: config.HTTPResponse{Status: 201, Body: `{"source":"hand-written"}`},
	}}
	base, _ := startRecordingServer(t, upstream.URL, "", seed)

	resp, err := http.Get(base + "/widgets/42")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected hand-written mock (201) to take precedence, got %d", resp.StatusCode)
	}
	if string(body) != `{"source":"hand-written"}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if upstreamHits != 0 {
		t.Fatalf("expected upstream never to be hit when a mock already matches, got %d hits", upstreamHits)
	}
}

func TestHTTPServer_RecordMode_ConcurrentRequestsDoNotDuplicate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/widgets/racey" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Give concurrent requests time to race each other before any of
		// them finishes recording.
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"racey"}`))
	}))
	defer upstream.Close()

	saveTo := filepath.Join(t.TempDir(), "recorded.yaml")
	base, srv := startRecordingServer(t, upstream.URL, saveTo, nil)

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(base + "/widgets/racey")
			if err != nil {
				t.Errorf("GET failed: %v", err)
				return
			}
			resp.Body.Close() //nolint:errcheck
		}()
	}
	wg.Wait()

	var raceyMocks int
	for _, m := range srv.GetMocks() {
		if m.Request.Path == "/widgets/racey" {
			raceyMocks++
		}
	}
	if raceyMocks != 1 {
		t.Fatalf("expected exactly 1 recorded mock for /widgets/racey despite %d concurrent requests, got %d", concurrency, raceyMocks)
	}
}

func TestHTTPServer_RecordMode_StripsAcceptEncodingFromUpstreamRequest(t *testing.T) {
	var gotAcceptEncoding string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/compressible" {
			gotAcceptEncoding = r.Header.Get("Accept-Encoding")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	base, _ := startRecordingServer(t, upstream.URL, "", nil)

	req, _ := http.NewRequest(http.MethodGet, base+"/compressible", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close() //nolint:errcheck

	// The caller's own "gzip, br" must not be forwarded verbatim: if it
	// were, Go's http.Transport would NOT auto-decompress the upstream's
	// response (it only does so when *it* sets Accept-Encoding), so any
	// gzip/br-compressed body would get captured as-is into the recorded
	// mock. Transport is still free to add its own "gzip" since we didn't
	// set one ourselves — that's what keeps auto-decompression working.
	if gotAcceptEncoding == "gzip, br" {
		t.Fatalf("expected the client's original Accept-Encoding not to be forwarded verbatim, got %q", gotAcceptEncoding)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
