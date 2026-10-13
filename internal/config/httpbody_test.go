package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHTTPBody_YAML_Literal(t *testing.T) {
	var b HTTPBody
	if err := yaml.Unmarshal([]byte(`"hello world"`), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if b.Literal != "hello world" || b.File != "" || b.Dir != "" {
		t.Fatalf("got %+v", b)
	}

	out, err := yaml.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != "hello world\n" {
		t.Fatalf("marshal round-trip: got %q", out)
	}
}

func TestHTTPBody_YAML_File(t *testing.T) {
	var b HTTPBody
	if err := yaml.Unmarshal([]byte(`file: ./checkout.html`), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if b.File != "./checkout.html" || b.Literal != "" || b.Dir != "" {
		t.Fatalf("got %+v", b)
	}

	out, err := yaml.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != "file: ./checkout.html\n" {
		t.Fatalf("marshal round-trip: got %q", out)
	}
}

func TestHTTPBody_YAML_Dir(t *testing.T) {
	var b HTTPBody
	if err := yaml.Unmarshal([]byte(`dir: ./site/`), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if b.Dir != "./site/" || b.Literal != "" || b.File != "" {
		t.Fatalf("got %+v", b)
	}
}

func TestHTTPBody_YAML_MutuallyExclusive(t *testing.T) {
	var b HTTPBody
	err := yaml.Unmarshal([]byte("file: a.html\ndir: b/"), &b)
	if err == nil {
		t.Fatal("expected error for file+dir both set")
	}
}

func TestHTTPBody_YAML_MissingBoth(t *testing.T) {
	var b HTTPBody
	err := yaml.Unmarshal([]byte("{}"), &b)
	if err == nil {
		t.Fatal("expected error when neither file nor dir is set")
	}
}

func TestHTTPBody_YAML_InvalidKind(t *testing.T) {
	var b HTTPBody
	err := yaml.Unmarshal([]byte("- 1\n- 2"), &b)
	if err == nil {
		t.Fatal("expected error for sequence node")
	}
}

func TestHTTPBody_JSON_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want HTTPBody
	}{
		{"literal", `"hi"`, HTTPBody{Literal: "hi"}},
		{"file", `{"file":"a.html"}`, HTTPBody{File: "a.html"}},
		{"dir", `{"dir":"site/"}`, HTTPBody{Dir: "site/"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b HTTPBody
			if err := json.Unmarshal([]byte(tc.in), &b); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if b.Literal != tc.want.Literal || b.File != tc.want.File || b.Dir != tc.want.Dir {
				t.Fatalf("got %+v, want %+v", b, tc.want)
			}
			out, err := json.Marshal(b)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var back HTTPBody
			if err := json.Unmarshal(out, &back); err != nil {
				t.Fatalf("round-trip unmarshal: %v", err)
			}
			if back.Literal != tc.want.Literal || back.File != tc.want.File || back.Dir != tc.want.Dir {
				t.Fatalf("round-trip got %+v, want %+v", back, tc.want)
			}
		})
	}
}

func TestHTTPBody_JSON_MutuallyExclusive(t *testing.T) {
	var b HTTPBody
	err := json.Unmarshal([]byte(`{"file":"a.html","dir":"b/"}`), &b)
	if err == nil {
		t.Fatal("expected error for file+dir both set")
	}
}

func TestHTTPBody_IsEmptyIsZero(t *testing.T) {
	var b HTTPBody
	if !b.IsEmpty() || !b.IsZero() {
		t.Fatal("zero-value HTTPBody should be empty/zero")
	}
	b = NewHTTPBodyLiteral("")
	if !b.IsEmpty() {
		t.Fatal("empty literal should be empty")
	}
	b = NewHTTPBodyLiteral("x")
	if b.IsEmpty() {
		t.Fatal("non-empty literal should not be empty")
	}
}

func TestHTTPBody_Resolve_Literal(t *testing.T) {
	b := NewHTTPBodyLiteral("plain text")
	content, ct, err := b.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if content != "plain text" || ct != "" {
		t.Fatalf("got content=%q ct=%q", content, ct)
	}
}

func TestHTTPBody_Resolve_File(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(htmlPath, []byte("<h1>{{.Request.Path}}</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := HTTPBody{File: "index.html", resolvedFile: htmlPath}
	content, ct, err := b.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if content != "<h1>{{.Request.Path}}</h1>" {
		t.Fatalf("got content %q", content)
	}
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("got content-type %q", ct)
	}
}

func TestHTTPBody_Resolve_FileMissing(t *testing.T) {
	b := HTTPBody{File: "nope.html", resolvedFile: "/does/not/exist.html"}
	if _, _, err := b.Resolve(); err == nil {
		t.Fatal("expected error reading missing file")
	}
}

func TestHTTPBody_Resolve_UnresolvedFileFallsBackToRawPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// resolvedFile intentionally left empty, as happens for an HTTPBody built
	// directly in Go code/tests rather than passed through ExpandHTTPMocks.
	b := HTTPBody{File: p}
	content, _, err := b.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if content != "data" {
		t.Fatalf("got %q", content)
	}
}

func TestIsBinaryContentType(t *testing.T) {
	cases := map[string]bool{
		"":                          false,
		"text/html; charset=utf-8":  false,
		"application/json":          false,
		"image/png":                 true,
		"image/jpeg":                true,
		"font/woff2":                true,
		"application/octet-stream":  true,
		"application/pdf":           true,
		"application/wasm":          true,
		"audio/mpeg":                true,
		"video/mp4":                 true,
		"application/javascript":    false,
		"image/svg+xml":             true,
	}
	for ct, want := range cases {
		if got := IsBinaryContentType(ct); got != want {
			t.Errorf("IsBinaryContentType(%q) = %v, want %v", ct, got, want)
		}
	}
}
