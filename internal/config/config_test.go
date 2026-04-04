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

func TestLoad_NewProtocolDefaults(t *testing.T) {
	yaml := `
protocols:
  graphql:
    enabled: true
  tcp:
    enabled: true
  redis:
    enabled: true
  smtp:
    enabled: true
  mqtt:
    enabled: true
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

	if cfg.Protocols.GraphQL == nil || cfg.Protocols.GraphQL.Port != 8082 {
		t.Errorf("want GraphQL port 8082, got %v", cfg.Protocols.GraphQL)
	}
	if cfg.Protocols.GraphQL.Path != "/graphql" {
		t.Errorf("want GraphQL path /graphql, got %q", cfg.Protocols.GraphQL.Path)
	}
	if cfg.Protocols.TCP == nil || cfg.Protocols.TCP.Port != 8083 {
		t.Errorf("want TCP port 8083, got %v", cfg.Protocols.TCP)
	}
	if cfg.Protocols.Redis == nil || cfg.Protocols.Redis.Port != 6379 {
		t.Errorf("want Redis port 6379, got %v", cfg.Protocols.Redis)
	}
	if cfg.Protocols.SMTP == nil || cfg.Protocols.SMTP.Port != 2525 {
		t.Errorf("want SMTP port 2525, got %v", cfg.Protocols.SMTP)
	}
	if cfg.Protocols.SMTP.Domain != "mockly.local" {
		t.Errorf("want SMTP domain mockly.local, got %q", cfg.Protocols.SMTP.Domain)
	}
	if cfg.Protocols.MQTT == nil || cfg.Protocols.MQTT.Port != 1883 {
		t.Errorf("want MQTT port 1883, got %v", cfg.Protocols.MQTT)
	}
}

func TestLoad_Scenarios(t *testing.T) {
	yaml := `
scenarios:
  - id: auth-down
    name: Auth Down
    description: Simulate auth service outage
    patches:
      - mock_id: token
        status: 503
        body: '{"error":"down"}'
      - mock_id: userinfo
        disabled: true
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

	if len(cfg.Scenarios) != 1 {
		t.Fatalf("want 1 scenario, got %d", len(cfg.Scenarios))
	}
	sc := cfg.Scenarios[0]
	if sc.ID != "auth-down" {
		t.Errorf("unexpected scenario ID %q", sc.ID)
	}
	if len(sc.Patches) != 2 {
		t.Fatalf("want 2 patches, got %d", len(sc.Patches))
	}
	if sc.Patches[0].MockID != "token" || sc.Patches[0].Status != 503 {
		t.Errorf("unexpected first patch: %+v", sc.Patches[0])
	}
	if !sc.Patches[1].Disabled {
		t.Error("second patch should be disabled")
	}
}

func TestLoad_GraphQLMocks(t *testing.T) {
	yaml := `
protocols:
  graphql:
    enabled: true
    port: 8082
    mocks:
      - id: get-user
        operation_type: query
        operation_name: GetUser
        response:
          user:
            id: "123"
            name: Alice
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

	gql := cfg.Protocols.GraphQL
	if gql == nil {
		t.Fatal("expected GraphQL config")
	}
	if len(gql.Mocks) != 1 {
		t.Fatalf("want 1 mock, got %d", len(gql.Mocks))
	}
	m := gql.Mocks[0]
	if m.ID != "get-user" || m.OperationType != "query" || m.OperationName != "GetUser" {
		t.Errorf("unexpected mock: %+v", m)
	}
}

func TestLoad_RedisMocks(t *testing.T) {
	yaml := `
protocols:
  redis:
    enabled: true
    mocks:
      - id: get-session
        command: GET
        key: "session:*"
        response:
          type: bulk
          value: '{"userId":"abc"}'
          delay: 10ms
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

	r := cfg.Protocols.Redis
	if r == nil || len(r.Mocks) != 1 {
		t.Fatal("expected 1 redis mock")
	}
	m := r.Mocks[0]
	if m.Command != "GET" || m.Key != "session:*" {
		t.Errorf("unexpected mock: %+v", m)
	}
	if m.Response.Type != "bulk" {
		t.Errorf("want type bulk, got %q", m.Response.Type)
	}
}
