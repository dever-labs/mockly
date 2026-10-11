package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
)

func TestReloadConfig_ValidYAML_ReturnsConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "mockly.yaml", `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: m1
        request: { method: GET, path: /m1 }
        response: { status: 200 }
`)
	cfg, err := reloadConfig(path)
	if err != nil {
		t.Fatalf("reloadConfig: %v", err)
	}
	if cfg.Protocols.HTTP == nil || len(cfg.Protocols.HTTP.Mocks) != 1 {
		t.Fatalf("HTTP mocks = %+v, want 1", cfg.Protocols.HTTP)
	}
}

func TestReloadConfig_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "mockly.yaml", "this: [is not: valid yaml")
	if _, err := reloadConfig(path); err == nil {
		t.Fatal("expected a parse error for invalid YAML, got nil")
	}
}

func TestReloadConfig_FailsValidation_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	// Two HTTP mocks sharing the same ID is a validation error.
	path := writeFile(t, dir, "mockly.yaml", `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: dup
        request: { method: GET, path: /a }
        response: { status: 200 }
      - id: dup
        request: { method: GET, path: /b }
        response: { status: 200 }
`)
	if _, err := reloadConfig(path); err == nil {
		t.Fatal("expected a validation error for duplicate mock IDs, got nil")
	}
}

// TestWatchConfig_FileEdit_TriggersApplyWithNewMocks is an end-to-end test
// of the real fsnotify-based watch loop: it edits the config file on disk
// and asserts the apply callback is eventually invoked with the new mock
// set, proving the whole watch -> debounce -> reload -> validate -> apply
// pipeline works together, not just its individual pieces.
func TestWatchConfig_FileEdit_TriggersApplyWithNewMocks(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "mockly.yaml", `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: v1
        request: { method: GET, path: /v1 }
        response: { status: 200 }
`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var applied int
	var lastMockIDs []string
	done := make(chan struct{}, 10)

	apply := func(cfg *config.Config) {
		mu.Lock()
		applied++
		lastMockIDs = nil
		if cfg.Protocols.HTTP != nil {
			for _, m := range cfg.Protocols.HTTP.Mocks {
				lastMockIDs = append(lastMockIDs, m.ID)
			}
		}
		mu.Unlock()
		done <- struct{}{}
	}

	go func() { _ = watchConfig(ctx, path, apply) }()

	// Give the watcher a moment to register its fsnotify.Add before
	// editing the file, avoiding a race where the write happens first.
	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(path, []byte(`
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: v2
        request: { method: GET, path: /v2 }
        response: { status: 200 }
`), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watchConfig to apply the reloaded config")
	}

	mu.Lock()
	defer mu.Unlock()
	if applied == 0 {
		t.Fatal("apply was never called")
	}
	if len(lastMockIDs) != 1 || lastMockIDs[0] != "v2" {
		t.Fatalf("lastMockIDs = %v, want [v2]", lastMockIDs)
	}
}

func TestWatchConfig_InvalidEdit_DoesNotApply(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "mockly.yaml", `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: v1
        request: { method: GET, path: /v1 }
        response: { status: 200 }
`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	applied := 0
	apply := func(cfg *config.Config) {
		mu.Lock()
		applied++
		mu.Unlock()
	}

	go func() { _ = watchConfig(ctx, path, apply) }()
	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(path, []byte("this: [is not: valid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Give the watcher time to notice, debounce, and attempt (and fail)
	// the reload; there's no success signal to wait on since apply must
	// never fire, so a fixed wait is used instead.
	time.Sleep(1 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if applied != 0 {
		t.Fatalf("apply was called %d time(s) for invalid YAML, want 0", applied)
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
