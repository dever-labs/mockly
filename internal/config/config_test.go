package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestLoad_DefaultsWhenMissing(t *testing.T) {
	cfg, err := config.Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mockly.UI.Port != 9090 {
		t.Errorf("want UI port 9090, got %d", cfg.Mockly.UI.Port)
	}
	if cfg.Mockly.API.Port != 9091 {
		t.Errorf("want API port 9091, got %d", cfg.Mockly.API.Port)
	}
}

func TestLoad_ParseYAML(t *testing.T) {
	yaml := `
mockly:
  ui:
    enabled: true
    port: 3000
  api:
    port: 3001
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: test-mock
        request:
          method: GET
          path: /hello
        response:
          status: 200
          body: '{"hello":"world"}'
          delay: 50ms
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Mockly.UI.Port != 3000 {
		t.Errorf("want 3000, got %d", cfg.Mockly.UI.Port)
	}
	if cfg.Protocols.HTTP == nil {
		t.Fatal("expected HTTP config")
	}
	if len(cfg.Protocols.HTTP.Mocks) != 1 {
		t.Fatalf("want 1 mock, got %d", len(cfg.Protocols.HTTP.Mocks))
	}
	m := cfg.Protocols.HTTP.Mocks[0]
	if m.ID != "test-mock" {
		t.Errorf("unexpected mock ID %q", m.ID)
	}
	if m.Response.Delay.Milliseconds() != 50 {
		t.Errorf("unexpected delay %v", m.Response.Delay)
	}
}
