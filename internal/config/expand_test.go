package config

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExpandHTTPMocks_File(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "page.html"), "<h1>hi</h1>")

	mocks := []HTTPMock{{
		ID:      "m1",
		Request: HTTPRequest{Method: "GET", Path: "/page"},
		Response: HTTPResponse{
			Status: 200,
			Body:   HTTPBody{File: "page.html"},
		},
	}}
	out, err := ExpandHTTPMocks(mocks, dir, true)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 mock, got %d", len(out))
	}
	content, _, err := out[0].Response.Body.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if content != "<h1>hi</h1>" {
		t.Fatalf("got %q", content)
	}
}

func TestExpandHTTPMocks_FileMissing(t *testing.T) {
	dir := t.TempDir()
	mocks := []HTTPMock{{
		ID:       "m1",
		Request:  HTTPRequest{Method: "GET", Path: "/page"},
		Response: HTTPResponse{Status: 200, Body: HTTPBody{File: "nope.html"}},
	}}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExpandHTTPMocks_RestrictRejectsEscape(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(parent, "secret.html"), "top secret")

	mocks := []HTTPMock{{
		ID:       "m1",
		Request:  HTTPRequest{Method: "GET", Path: "/page"},
		Response: HTTPResponse{Status: 200, Body: HTTPBody{File: "../secret.html"}},
	}}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error for path escaping baseDir with restrict=true")
	}
	// A *relative* escape (classic CWE-22 "../" traversal) is rejected
	// even with restrict=false — this guards the /api/mocks/http endpoint
	// against path-injection from an untrusted caller, since restrict=false
	// is also used there (see internal/api/server.go's
	// resolveAddedMockBody).
	if _, err := ExpandHTTPMocks(mocks, dir, false); err == nil {
		t.Fatal("expected error for relative path escaping baseDir even with restrict=false")
	}
}

// TestExpandHTTPMocks_UnrestrictedAllowsExplicitAbsolutePath verifies that
// restrict=false still permits an explicit absolute path reference outside
// baseDir — unlike a relative "../" escape, this is a deliberate reference
// by the caller (e.g. the "mockly add page --dir" CLI command, which
// resolves directory-expansion mocks client-side into absolute paths
// before POSTing them to the API, specifically so they survive the
// round-trip to a server with a different working directory).
func TestExpandHTTPMocks_UnrestrictedAllowsExplicitAbsolutePath(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	absFile := filepath.Join(parent, "secret.html")
	writeFile(t, absFile, "top secret")

	mocks := []HTTPMock{{
		ID:       "m1",
		Request:  HTTPRequest{Method: "GET", Path: "/page"},
		Response: HTTPResponse{Status: 200, Body: HTTPBody{File: absFile}},
	}}
	out, err := ExpandHTTPMocks(mocks, dir, false)
	if err != nil {
		t.Fatalf("expected no error for an explicit absolute path with restrict=false, got %v", err)
	}
	content, _, err := out[0].Response.Body.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if content != "top secret" {
		t.Fatalf("got %q", content)
	}

	// The same absolute path is rejected when restrict=true (config.Load).
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error for absolute path escaping baseDir with restrict=true")
	}
}

func TestExpandHTTPMocks_Dir_Basic(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "index.html"), "home")
	writeFile(t, filepath.Join(site, "about.html"), "about")
	writeFile(t, filepath.Join(site, "assets", "style.css"), "css")
	writeFile(t, filepath.Join(site, ".hidden"), "skip me")
	if err := os.MkdirAll(filepath.Join(site, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(site, ".git", "config"), "ignored")

	mocks := []HTTPMock{{
		ID:       "site",
		Request:  HTTPRequest{Method: "GET", Path: "/site"},
		Response: HTTPResponse{Body: HTTPBody{Dir: "site"}},
	}}
	out, err := ExpandHTTPMocks(mocks, dir, true)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	paths := make([]string, 0, len(out))
	for _, m := range out {
		paths = append(paths, m.Request.Path)
	}
	sort.Strings(paths)
	want := []string{"/site", "/site/about.html", "/site/assets/style.css", "/site/index.html"}
	if len(paths) != len(want) {
		t.Fatalf("got paths %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("got paths %v, want %v", paths, want)
		}
	}

	// index.html is aliased at the parent path with the parent's own ID.
	var rootMock, indexMock *HTTPMock
	for i := range out {
		switch out[i].Request.Path {
		case "/site":
			rootMock = &out[i]
		case "/site/index.html":
			indexMock = &out[i]
		}
	}
	if rootMock == nil || indexMock == nil {
		t.Fatal("expected both /site and /site/index.html mocks")
	}
	if rootMock.ID != "site" {
		t.Fatalf("root alias ID = %q, want %q", rootMock.ID, "site")
	}
	rootContent, _, err := rootMock.Response.Body.Resolve()
	if err != nil || rootContent != "home" {
		t.Fatalf("root content = %q, err=%v", rootContent, err)
	}
}

func TestExpandHTTPMocks_Dir_IDsHaveNoSlash(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "deep", "nested", "page.html"), "deep")

	mocks := []HTTPMock{{
		Request:  HTTPRequest{Method: "GET", Path: "/checkout"},
		Response: HTTPResponse{Body: HTTPBody{Dir: "site"}},
	}}
	out, err := ExpandHTTPMocks(mocks, dir, true)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, m := range out {
		if filepath.Base(m.ID) != m.ID || containsSlash(m.ID) {
			t.Errorf("generated mock ID %q contains a slash", m.ID)
		}
	}
}

func containsSlash(s string) bool {
	for _, r := range s {
		if r == '/' {
			return true
		}
	}
	return false
}

func TestExpandHTTPMocks_Dir_CollisionWithHandWritten(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "about.html"), "about")

	mocks := []HTTPMock{
		{
			ID:       "hand-written",
			Request:  HTTPRequest{Method: "GET", Path: "/docs/about.html"},
			Response: HTTPResponse{Status: 200, Body: NewHTTPBodyLiteral("manual")},
		},
		{
			ID:       "docs",
			Request:  HTTPRequest{Method: "GET", Path: "/docs"},
			Response: HTTPResponse{Body: HTTPBody{Dir: "site"}},
		},
	}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected collision error between hand-written mock and dir expansion")
	}
}

func TestExpandHTTPMocks_Dir_RejectsPathRegex(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "a.html"), "a")

	mocks := []HTTPMock{{
		Request:  HTTPRequest{Method: "GET", PathRegex: "^/docs.*"},
		Response: HTTPResponse{Body: HTTPBody{Dir: "site"}},
	}}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error for body.dir combined with path_regex")
	}
}

func TestExpandHTTPMocks_Dir_RejectsWildcardPath(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "a.html"), "a")

	cases := []string{"/docs/*", "/docs/{name}", "re:^/docs"}
	for _, p := range cases {
		mocks := []HTTPMock{{
			Request:  HTTPRequest{Method: "GET", Path: p},
			Response: HTTPResponse{Body: HTTPBody{Dir: "site"}},
		}}
		if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
			t.Errorf("path %q: expected error for non-literal path with body.dir", p)
		}
	}
}

func TestExpandHTTPMocks_Dir_RejectsSequence(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "a.html"), "a")

	mocks := []HTTPMock{{
		Request: HTTPRequest{Method: "GET", Path: "/docs"},
		Response: HTTPResponse{
			Body: HTTPBody{Dir: "site"},
		},
		Sequence: []HTTPResponse{{Status: 200, Body: NewHTTPBodyLiteral("x")}},
	}}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error for body.dir combined with sequence")
	}
}

func TestExpandHTTPMocks_Dir_EmptyDirErrors(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	mocks := []HTTPMock{{
		Request:  HTTPRequest{Method: "GET", Path: "/empty"},
		Response: HTTPResponse{Body: HTTPBody{Dir: "empty"}},
	}}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error for empty directory")
	}
}

func TestExpandHTTPMocks_Dir_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notadir.html"), "x")
	mocks := []HTTPMock{{
		Request:  HTTPRequest{Method: "GET", Path: "/x"},
		Response: HTTPResponse{Body: HTTPBody{Dir: "notadir.html"}},
	}}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error when body.dir points at a file")
	}
}

func TestExpandHTTPMocks_SequenceBodyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.html"), "A")
	writeFile(t, filepath.Join(dir, "b.html"), "B")

	mocks := []HTTPMock{{
		ID:       "seq",
		Request:  HTTPRequest{Method: "GET", Path: "/seq"},
		Response: HTTPResponse{Status: 200, Body: NewHTTPBodyLiteral("first")},
		Sequence: []HTTPResponse{
			{Status: 200, Body: HTTPBody{File: "a.html"}},
			{Status: 200, Body: HTTPBody{File: "b.html"}},
		},
	}}
	out, err := ExpandHTTPMocks(mocks, dir, true)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	c0, _, _ := out[0].Sequence[0].Body.Resolve()
	c1, _, _ := out[0].Sequence[1].Body.Resolve()
	if c0 != "A" || c1 != "B" {
		t.Fatalf("got sequence bodies %q, %q", c0, c1)
	}
}

func TestExpandHTTPMocks_SequenceBodyDirRejected(t *testing.T) {
	dir := t.TempDir()
	mocks := []HTTPMock{{
		ID:      "seq",
		Request: HTTPRequest{Method: "GET", Path: "/seq"},
		Response: HTTPResponse{
			Status: 200,
			Body:   NewHTTPBodyLiteral("first"),
		},
		Sequence: []HTTPResponse{
			{Status: 200, Body: HTTPBody{Dir: "site"}},
		},
	}}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected error for body.dir inside sequence entry")
	}
}

// TestExpandHTTPMocks_AllowsDuplicatePathAmongHandWrittenMocks guards
// against a regression where the body.dir collision check was applied too
// broadly and rejected ordinary hand-written mocks that legitimately
// share a (method, path) pair, differentiated by other match conditions
// (query params, headers, body_json, state) — exactly the real-world e2e
// scenario (e.g. an exact-match mock plus a wildcard/fallback mock on the
// same path) that this must continue to support.
func TestExpandHTTPMocks_AllowsDuplicatePathAmongHandWrittenMocks(t *testing.T) {
	dir := t.TempDir()
	mocks := []HTTPMock{
		{
			ID:       "admin-users",
			Request:  HTTPRequest{Method: "GET", Path: "/api/users", Query: map[string]string{"role": "admin"}},
			Response: HTTPResponse{Status: 200, Body: NewHTTPBodyLiteral("admins")},
		},
		{
			ID:       "all-users",
			Request:  HTTPRequest{Method: "GET", Path: "/api/users"},
			Response: HTTPResponse{Status: 200, Body: NewHTTPBodyLiteral("everyone")},
		},
	}
	out, err := ExpandHTTPMocks(mocks, dir, true)
	if err != nil {
		t.Fatalf("expected no collision error between hand-written mocks sharing a path, got %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(out))
	}
}

// TestExpandHTTPMocks_Dir_StillRejectsHandWrittenSharingGeneratedPath
// ensures the narrowed collision check still catches the one case that
// matters: a dir-expansion-generated mock silently colliding with another
// mock at the same path, regardless of which one appears first in the
// input list.
func TestExpandHTTPMocks_Dir_StillRejectsHandWrittenSharingGeneratedPath(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "about.html"), "about")

	mocks := []HTTPMock{
		{
			ID:       "docs",
			Request:  HTTPRequest{Method: "GET", Path: "/docs"},
			Response: HTTPResponse{Body: HTTPBody{Dir: "site"}},
		},
		{
			ID:       "hand-written",
			Request:  HTTPRequest{Method: "GET", Path: "/docs/about.html"},
			Response: HTTPResponse{Status: 200, Body: NewHTTPBodyLiteral("manual")},
		},
	}
	if _, err := ExpandHTTPMocks(mocks, dir, true); err == nil {
		t.Fatal("expected collision error when generated mock comes before the conflicting hand-written one")
	}
}

func TestExpandDirToLiteralMocks_Basic(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(site, "index.html"), "home")
	writeFile(t, filepath.Join(site, "sub", "page.html"), "sub page")

	parent := HTTPMock{
		ID:       "site",
		Request:  HTTPRequest{Method: "GET", Path: "/site"},
		Response: HTTPResponse{Body: HTTPBody{Dir: "site"}},
	}
	out, err := ExpandDirToLiteralMocks(parent, dir)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	var root, sub *HTTPMock
	for i := range out {
		switch out[i].Request.Path {
		case "/site":
			root = &out[i]
		case "/site/sub/page.html":
			sub = &out[i]
		}
	}
	if root == nil || sub == nil {
		t.Fatalf("expected /site and /site/sub/page.html mocks, got %+v", out)
	}
	// Content is literal, not a file reference — no server-side filesystem
	// access needed to serve it, and it survives JSON serialization as-is.
	if root.Response.Body.File != "" || root.Response.Body.Dir != "" {
		t.Errorf("expected a pure literal body, got File=%q Dir=%q", root.Response.Body.File, root.Response.Body.Dir)
	}
	content, _, err := root.Response.Body.Resolve()
	if err != nil || content != "home" {
		t.Fatalf("root content = %q, err=%v", content, err)
	}
	if root.Response.Headers["Content-Type"] != "text/html; charset=utf-8" {
		t.Errorf("expected inferred Content-Type, got %q", root.Response.Headers["Content-Type"])
	}
}

func TestExpandDirToLiteralMocks_AllowsAbsoluteDirOutsideBaseDir(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "a.html"), "A")
	baseDir := t.TempDir()

	parent := HTTPMock{
		Request:  HTTPRequest{Method: "GET", Path: "/ext"},
		Response: HTTPResponse{Body: HTTPBody{Dir: outside}},
	}
	out, err := ExpandDirToLiteralMocks(parent, baseDir)
	if err != nil {
		t.Fatalf("expected an absolute --dir outside baseDir to be allowed for client-side CLI expansion, got %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected at least one generated mock")
	}
}
