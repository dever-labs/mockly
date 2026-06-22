package testcontainersmockly

import (
	"io"
	"reflect"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

func TestWithImage_OverridesDefault(t *testing.T) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{Image: DefaultImage},
	}

	WithImage("custom:tag")(&req)

	if req.Image != "custom:tag" {
		t.Fatalf("expected image custom:tag, got %q", req.Image)
	}
}

func TestWithInlineConfig_SetsFile(t *testing.T) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Files: []testcontainers.ContainerFile{containerConfigFile(defaultConfig)},
		},
	}

	WithInlineConfig("yaml: val")(&req)

	if len(req.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(req.Files))
	}
	if req.Files[0].ContainerFilePath != ContainerConfigPath {
		t.Fatalf("expected file path %q, got %q", ContainerConfigPath, req.Files[0].ContainerFilePath)
	}

	got, err := io.ReadAll(req.Files[0].Reader)
	if err != nil {
		t.Fatalf("reading config reader: %v", err)
	}
	if string(got) != "yaml: val" {
		t.Fatalf("expected file contents %q, got %q", "yaml: val", string(got))
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultImage != "ghcr.io/dever-labs/mockly:latest" {
		t.Fatalf("DefaultImage = %q", DefaultImage)
	}
	if HTTPPort != "8090/tcp" {
		t.Fatalf("HTTPPort = %q", HTTPPort)
	}
	if APIPort != "9091/tcp" {
		t.Fatalf("APIPort = %q", APIPort)
	}
	if ContainerConfigPath != "/config/mockly.yaml" {
		t.Fatalf("ContainerConfigPath = %q", ContainerConfigPath)
	}
}

func TestDefaultContainerRequest_UsesConfigFile(t *testing.T) {
	req := defaultContainerRequest()

	wantCmd := []string{"start", "-c", ContainerConfigPath}
	if !reflect.DeepEqual(req.Cmd, wantCmd) {
		t.Fatalf("expected command %v, got %v", wantCmd, req.Cmd)
	}
	if len(req.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(req.Files))
	}
	if req.Files[0].ContainerFilePath != ContainerConfigPath {
		t.Fatalf("expected file path %q, got %q", ContainerConfigPath, req.Files[0].ContainerFilePath)
	}

	got, err := io.ReadAll(req.Files[0].Reader)
	if err != nil {
		t.Fatalf("reading default config reader: %v", err)
	}
	if string(got) != defaultConfig {
		t.Fatalf("expected default config %q, got %q", defaultConfig, string(got))
	}
}
