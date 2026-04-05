package mocklydriver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetFreePort(t *testing.T) {
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("getFreePort() error: %v", err)
	}
	if port < 1 || port > 65535 {
		t.Fatalf("getFreePort() returned out-of-range port: %d", port)
	}
}

func TestGetBinaryPathReturnsEmptyWhenMissing(t *testing.T) {
	t.Setenv("MOCKLY_BINARY_PATH", "")
	result := GetBinaryPath("/nonexistent/path/that/does/not/exist")
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestGetBinaryPathRespectsEnvVar(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "mockly-fake-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	f.Close()

	t.Setenv("MOCKLY_BINARY_PATH", f.Name())
	result := GetBinaryPath("")
	if result != f.Name() {
		t.Fatalf("expected %q, got %q", f.Name(), result)
	}
}

func TestGetBinaryPathIgnoresMissingEnvVar(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("MOCKLY_BINARY_PATH", nonExistent)

	result := GetBinaryPath("")
	if result != "" {
		t.Fatalf("expected empty string when MOCKLY_BINARY_PATH points to missing file, got %q", result)
	}
}

func TestInstallReturnsErrorWithNoInstall(t *testing.T) {
	t.Setenv("MOCKLY_BINARY_PATH", "")
	t.Setenv("MOCKLY_NO_INSTALL", "1")

	_, err := Install(InstallOptions{BinDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error when MOCKLY_NO_INSTALL is set, got nil")
	}
	if !strings.Contains(err.Error(), "MOCKLY_NO_INSTALL") {
		t.Fatalf("expected error message to mention MOCKLY_NO_INSTALL, got: %v", err)
	}
}

func TestInstallReturnsStagedBinaryPath(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "mockly-staged-*")
	if err != nil {
		t.Fatalf("creating staged binary: %v", err)
	}
	f.Close()

	t.Setenv("MOCKLY_BINARY_PATH", f.Name())

	got, err := Install(InstallOptions{})
	if err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}
	if got != f.Name() {
		t.Fatalf("expected %q, got %q", f.Name(), got)
	}
}

func TestIsPortConflict(t *testing.T) {
	cases := []struct {
		msg      string
		expected bool
	}{
		{"listen tcp 0.0.0.0:9000: bind: address already in use", true},
		{"EADDRINUSE :::9000", true},
		{"bind: address already in use", true},
		{"connection refused", false},
		{"timeout waiting for server", false},
		{"", false},
	}

	for _, c := range cases {
		got := isPortConflict(c.msg)
		if got != c.expected {
			t.Errorf("isPortConflict(%q) = %v, want %v", c.msg, got, c.expected)
		}
	}
}
