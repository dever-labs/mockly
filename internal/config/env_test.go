package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

func TestExpandEnvVarsSubstitutesSetVariable(t *testing.T) {
	t.Setenv("MOCKLY_TEST_SECRET", "s3cr3t")
	out, err := expandEnvVars([]byte(`headers: { X-Signature: "${MOCKLY_TEST_SECRET}" }`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "s3cr3t") {
		t.Fatalf("expected substituted value in output, got: %s", out)
	}
}

func TestExpandEnvVarsUsesDefaultWhenUnset(t *testing.T) {
	os.Unsetenv("MOCKLY_TEST_UNSET_VAR") //nolint:errcheck
	out, err := expandEnvVars([]byte(`value: "${MOCKLY_TEST_UNSET_VAR:-fallback}"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "fallback") {
		t.Fatalf("expected fallback value in output, got: %s", out)
	}
}

func TestExpandEnvVarsAllowsEmptyDefault(t *testing.T) {
	os.Unsetenv("MOCKLY_TEST_UNSET_VAR2") //nolint:errcheck
	out, err := expandEnvVars([]byte(`value: "${MOCKLY_TEST_UNSET_VAR2:-}"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "${") {
		t.Fatalf("expected placeholder to be fully expanded, got: %s", out)
	}
}

func TestExpandEnvVarsFailsLoudlyWhenUnsetAndNoDefault(t *testing.T) {
	os.Unsetenv("MOCKLY_TEST_MISSING_VAR") //nolint:errcheck
	_, err := expandEnvVars([]byte(`value: "${MOCKLY_TEST_MISSING_VAR}"`))
	if err == nil {
		t.Fatal("expected an error for an undefined variable with no default")
	}
	if !strings.Contains(err.Error(), "MOCKLY_TEST_MISSING_VAR") {
		t.Fatalf("expected error to mention the variable name, got: %v", err)
	}
}

func TestExpandEnvVarsSetVariableWinsOverDefault(t *testing.T) {
	t.Setenv("MOCKLY_TEST_WITH_DEFAULT", "real-value")
	out, err := expandEnvVars([]byte(`value: "${MOCKLY_TEST_WITH_DEFAULT:-fallback}"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "real-value") || strings.Contains(string(out), "fallback") {
		t.Fatalf("expected the set env var to win over the default, got: %s", out)
	}
}

func TestExpandEnvVarsLeavesPlainStringsUntouched(t *testing.T) {
	out, err := expandEnvVars([]byte(`body: "Price is $5.00, no braces here"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != `body: "Price is $5.00, no braces here"` {
		t.Fatalf("expected input unchanged, got: %s", out)
	}
}

func TestLoadExpandsEnvVarsInConfigFile(t *testing.T) {
	t.Setenv("MOCKLY_TEST_PORT_SUFFIX", "080")
	path := writeTempConfig(t, `
protocols:
  http:
    enabled: true
    port: 8${MOCKLY_TEST_PORT_SUFFIX}
    mocks:
      - id: test
        request: { method: GET, path: /x }
        response: { status: 200, body: "${MOCKLY_TEST_PORT_SUFFIX}" }
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Protocols.HTTP.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", cfg.Protocols.HTTP.Port)
	}
	if cfg.Protocols.HTTP.Mocks[0].Response.Body != "080" {
		t.Fatalf("expected body %q, got %q", "080", cfg.Protocols.HTTP.Mocks[0].Response.Body)
	}
}

func TestLoadFailsWhenConfigReferencesUndefinedEnvVar(t *testing.T) {
	os.Unsetenv("MOCKLY_TEST_REALLY_UNDEFINED") //nolint:errcheck
	path := writeTempConfig(t, `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: test
        request: { method: GET, path: /x }
        response: { status: 200, body: "${MOCKLY_TEST_REALLY_UNDEFINED}" }
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to fail for an undefined env var with no default")
	}
}
