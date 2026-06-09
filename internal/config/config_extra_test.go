package config_test

// Additional tests to cover applyDefaults branches, error paths, and UnmarshalYAML.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

// ---------------------------------------------------------------------------
// Load error paths
// ---------------------------------------------------------------------------

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte("protocols: [invalid: yaml: {"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_ReadError(t *testing.T) {
	// Pass a path to a directory (not a file) — ReadFile on a dir returns an error
	// that is not os.IsNotExist, triggering the read-error branch.
	dir := t.TempDir()
	_, err := config.Load(dir) // dir is readable but ReadFile returns "is a directory"
	if err == nil {
		t.Fatal("expected error when reading a directory, got nil")
	}
}

func TestSave_WriteError(t *testing.T) {
	// Pass a path inside a nonexistent directory to force a write error.
	err := config.Save("/nonexistent-dir/mockly.yaml", &config.Config{})
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// applyDefaults — new protocol branches not covered by existing tests
// ---------------------------------------------------------------------------

func TestApplyDefaults_AllProtocols(t *testing.T) {
	yaml := `
protocols:
  http:
    enabled: true
  websocket:
    enabled: true
  grpc:
    enabled: true
  snmp:
    enabled: true
  dns:
    enabled: true
  amqp:
    enabled: true
  kafka:
    enabled: true
  ldap:
    enabled: true
  imap:
    enabled: true
  ftp:
    enabled: true
  memcached:
    enabled: true
  stomp:
    enabled: true
  coap:
    enabled: true
  sip:
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

	cases := []struct {
		name string
		got  int
		want int
	}{
		{"HTTP", cfg.Protocols.HTTP.Port, 8080},
		{"WebSocket", cfg.Protocols.WebSocket.Port, 8081},
		{"GRPC", cfg.Protocols.GRPC.Port, 50051},
		{"DNS", cfg.Protocols.DNS.Port, 5353},
		{"AMQP", cfg.Protocols.AMQP.Port, 5672},
		{"Kafka", cfg.Protocols.Kafka.Port, 9092},
		{"LDAP", cfg.Protocols.LDAP.Port, 3893},
		{"IMAP", cfg.Protocols.IMAP.Port, 1143},
		{"FTP", cfg.Protocols.FTP.Port, 2121},
		{"FTP PassivePortStart", cfg.Protocols.FTP.PassivePortStart, 50000},
		{"Memcached", cfg.Protocols.Memcached.Port, 11211},
		{"STOMP", cfg.Protocols.STOMP.Port, 61613},
		{"CoAP", cfg.Protocols.CoAP.Port, 5683},
		{"SIP", cfg.Protocols.SIP.Port, 5060},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s default port: want %d, got %d", c.name, c.want, c.got)
		}
	}

	// SNMP sub-field defaults
	if cfg.Protocols.SNMP == nil {
		t.Fatal("expected SNMP config")
	}
	if cfg.Protocols.SNMP.Port != 1161 {
		t.Errorf("SNMP port: want 1161, got %d", cfg.Protocols.SNMP.Port)
	}
	if cfg.Protocols.SNMP.Community != "public" {
		t.Errorf("SNMP community: want public, got %q", cfg.Protocols.SNMP.Community)
	}
}

func TestApplyDefaults_SMTPSubFields(t *testing.T) {
	yaml := `
protocols:
  smtp:
    enabled: true
    max_emails: 500
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
	if cfg.Protocols.SMTP.Domain != "mockly.local" {
		t.Errorf("SMTP domain default: want mockly.local, got %q", cfg.Protocols.SMTP.Domain)
	}
	// max_emails was supplied — should NOT be overridden
	if cfg.Protocols.SMTP.MaxEmails != 500 {
		t.Errorf("SMTP max_emails: want 500, got %d", cfg.Protocols.SMTP.MaxEmails)
	}
}

func TestApplyDefaults_SMTPMaxEmailsDefault(t *testing.T) {
	yaml := `
protocols:
  smtp:
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
	if cfg.Protocols.SMTP.MaxEmails != 1000 {
		t.Errorf("SMTP max_emails default: want 1000, got %d", cfg.Protocols.SMTP.MaxEmails)
	}
}

func TestApplyDefaults_FTPPassivePortStartPreserved(t *testing.T) {
	yaml := `
protocols:
  ftp:
    enabled: true
    port: 2121
    passive_port_start: 60000
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
	if cfg.Protocols.FTP.PassivePortStart != 60000 {
		t.Errorf("FTP passive_port_start: want 60000, got %d", cfg.Protocols.FTP.PassivePortStart)
	}
}

// ---------------------------------------------------------------------------
// Duration — UnmarshalYAML
// ---------------------------------------------------------------------------

func TestDuration_UnmarshalYAML_ViaLoad(t *testing.T) {
	// Exercise UnmarshalYAML valid path via a config that has a duration field.
	yaml := `
protocols:
  http:
    enabled: true
    mocks:
      - id: m1
        request:
          method: GET
          path: /
        response:
          status: 200
          delay: 100ms
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
	if cfg.Protocols.HTTP.Mocks[0].Response.Delay.Milliseconds() != 100 {
		t.Errorf("delay: want 100ms, got %v", cfg.Protocols.HTTP.Mocks[0].Response.Delay)
	}
}

func TestDuration_UnmarshalYAML_InvalidViaLoad(t *testing.T) {
	yaml := `
protocols:
  http:
    enabled: true
    mocks:
      - id: m1
        response:
          delay: not-a-duration
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid duration in YAML, got nil")
	}
}
