package main

import (
	"testing"
)

// TestLoadResolvedConfig_DirectoryMode_ResolvesInlineSchemaRelativeToDir
// guards against a regression where directory-mode configs (--config
// pointing at a directory rather than a file) resolved inline schema refs
// (openapi/asyncapi/proto) relative to the directory's *parent* instead of
// the directory itself, because loadResolvedConfig blindly used
// filepath.Dir(path) — which only works correctly for an actual file path.
func TestLoadResolvedConfig_DirectoryMode_ResolvesInlineSchemaRelativeToDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "mockly.yaml", `
protocols:
  http:
    enabled: true
    port: 8080
    openapi: openapi.yaml
`)
	writeFile(t, dir, "openapi.yaml", `
openapi: 3.0.0
info: { title: t, version: "1" }
paths:
  /ping:
    get:
      responses:
        '200': { description: ok }
`)

	cfg, err := loadResolvedConfig(dir)
	if err != nil {
		t.Fatalf("loadResolvedConfig(%q): %v", dir, err)
	}
	if cfg.Protocols.HTTP == nil || len(cfg.Protocols.HTTP.Mocks) == 0 {
		t.Fatal("expected openapi.yaml's /ping operation to produce at least one HTTP mock")
	}
}

// TestLoadResolvedConfig_FileMode_ResolvesInlineSchemaRelativeToFileDir
// pins the pre-existing (file-path) behaviour so the directory-mode fix
// above can't silently break it.
func TestLoadResolvedConfig_FileMode_ResolvesInlineSchemaRelativeToFileDir(t *testing.T) {
	dir := t.TempDir()
	main := writeFile(t, dir, "mockly.yaml", `
protocols:
  http:
    enabled: true
    port: 8080
    openapi: openapi.yaml
`)
	writeFile(t, dir, "openapi.yaml", `
openapi: 3.0.0
info: { title: t, version: "1" }
paths:
  /ping:
    get:
      responses:
        '200': { description: ok }
`)

	cfg, err := loadResolvedConfig(main)
	if err != nil {
		t.Fatalf("loadResolvedConfig(%q): %v", main, err)
	}
	if cfg.Protocols.HTTP == nil || len(cfg.Protocols.HTTP.Mocks) == 0 {
		t.Fatal("expected openapi.yaml's /ping operation to produce at least one HTTP mock")
	}
}
