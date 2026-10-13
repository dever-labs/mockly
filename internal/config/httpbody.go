package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// HTTPBody is the type of HTTPResponse.Body (and each HTTPMock.Sequence
// entry's Body): it accepts more than one shape, same field, mirroring the
// Duration type's custom UnmarshalYAML/UnmarshalJSON precedent elsewhere in
// this file.
//
//   - Plain string (unchanged, 100% backward compatible):
//     body: "<html>...</html>"
//
//   - A single file on disk, loaded (and re-read on every match, so edits
//     on disk are picked up live — the same way a literal Body's "{{ }}"
//     templates already re-render every request) at response time:
//     body: { file: ./checkout.html }
//
//   - A whole directory, expanded into one HTTPMock per file at
//     config-load time (see ExpandHTTPMocks) — only ever valid on a
//     top-level HTTPMock.Response.Body, never inside Sequence:
//     body: { dir: ./checkout/ }
//
// File/Dir paths as written in config are resolved relative to whichever
// base directory is meaningful for the caller (config.Load resolves
// relative to the defining YAML file's own directory and rejects ".."
// escapes out of it; the "mockly add page" CLI command resolves relative
// to its own working directory) — see ResolveHTTPBodyPaths.
type HTTPBody struct {
	Literal string // used when File and Dir are both empty
	File    string // as written in config, before path resolution
	Dir     string // as written in config, before path resolution; only valid pre-expansion

	// resolvedFile is File resolved to an absolute path by
	// ResolveHTTPBodyPaths (or left empty if Body was never passed through
	// it, e.g. a mock built directly in Go code/tests) — Resolve falls
	// back to File itself in that case.
	resolvedFile string
}

// NewHTTPBodyLiteral returns an HTTPBody holding a plain literal string,
// equivalent to what a bare YAML/JSON string scalar decodes to. Useful for
// call sites that build/patch a Body from a plain Go string (e.g. the
// mock-patch API, or the "add http"/"add page" CLI commands).
func NewHTTPBodyLiteral(s string) HTTPBody {
	return HTTPBody{Literal: s}
}

// IsEmpty reports whether b carries no content at all (the zero value) —
// e.g. a response that only sets Stream, with no static Body.
func (b HTTPBody) IsEmpty() bool {
	return b.Literal == "" && b.File == "" && b.Dir == ""
}

// IsZero makes HTTPBody satisfy gopkg.in/yaml.v3's IsZeroer interface, so
// "yaml:\"body,omitempty\"" correctly omits an unset Body instead of always
// emitting an empty mapping/string.
func (b HTTPBody) IsZero() bool {
	return b.IsEmpty()
}

// Resolve returns b's actual content and, for a file reference, its
// inferred Content-Type (empty for a literal — callers already control
// Content-Type via Headers in that case). File content is read fresh on
// every call (no caching), so edits on disk take effect immediately.
func (b HTTPBody) Resolve() (content string, contentType string, err error) {
	if b.File == "" {
		return b.Literal, "", nil
	}
	path := b.resolvedFile
	if path == "" {
		path = b.File // not resolved via ResolveHTTPBodyPaths (e.g. built directly in tests/Go code)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path resolved/validated at config-load or CLI time
	if err != nil {
		return "", "", fmt.Errorf("reading body file %q: %w", b.File, err)
	}
	ct := mime.TypeByExtension(filepath.Ext(path))
	return string(data), ct, nil
}

// IsBinaryContentType reports whether contentType names a format that
// should never be passed through Go template rendering (images, fonts,
// and generic/unknown binary data) — guarding against corrupting a binary
// asset expanded from a body.dir folder on the rare chance it happens to
// contain the literal "{{" byte sequence. Text-ish formats (html, css, js,
// json, svg, plain text, …) are rendered normally.
func IsBinaryContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	if semi := strings.Index(ct, ";"); semi != -1 {
		ct = ct[:semi]
	}
	ct = strings.TrimSpace(ct)
	switch {
	case ct == "":
		return false
	case strings.HasPrefix(ct, "image/"):
		return true
	case strings.HasPrefix(ct, "font/"):
		return true
	case ct == "application/octet-stream",
		ct == "application/pdf",
		ct == "application/wasm",
		ct == "application/vnd.ms-fontobject",
		ct == "audio/mpeg", ct == "audio/wav", ct == "audio/ogg",
		ct == "video/mp4", ct == "video/webm", ct == "video/ogg":
		return true
	default:
		return false
	}
}

func (b *HTTPBody) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		b.Literal = value.Value
		b.File, b.Dir = "", ""
		return nil
	}
	if value.Kind == yaml.MappingNode {
		var spec struct {
			File string `yaml:"file"`
			Dir  string `yaml:"dir"`
		}
		if err := value.Decode(&spec); err != nil {
			return fmt.Errorf("body: %w", err)
		}
		return b.setStructured(spec.File, spec.Dir)
	}
	return fmt.Errorf("body: expected a string or a {file: ...}/{dir: ...} mapping")
}

func (b HTTPBody) MarshalYAML() (interface{}, error) {
	switch {
	case b.File != "":
		return map[string]string{"file": b.File}, nil
	case b.Dir != "":
		return map[string]string{"dir": b.Dir}, nil
	default:
		return b.Literal, nil
	}
}

func (b *HTTPBody) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		b.Literal = s
		b.File, b.Dir = "", ""
		return nil
	}
	var spec struct {
		File string `json:"file"`
		Dir  string `json:"dir"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("body: expected a string or a {\"file\": ...}/{\"dir\": ...} object: %w", err)
	}
	return b.setStructured(spec.File, spec.Dir)
}

func (b HTTPBody) MarshalJSON() ([]byte, error) {
	switch {
	case b.File != "":
		return json.Marshal(map[string]string{"file": b.File})
	case b.Dir != "":
		return json.Marshal(map[string]string{"dir": b.Dir})
	default:
		return json.Marshal(b.Literal)
	}
}

func (b *HTTPBody) setStructured(file, dir string) error {
	if file == "" && dir == "" {
		return fmt.Errorf(`body: structured form requires "file" or "dir"`)
	}
	if file != "" && dir != "" {
		return fmt.Errorf(`body: "file" and "dir" are mutually exclusive`)
	}
	b.Literal = ""
	b.File = file
	b.Dir = dir
	return nil
}

// ---------------------------------------------------------------------------
// Path resolution + directory expansion (issue #202)
// ---------------------------------------------------------------------------

// resolveBodyPath resolves raw (as written in a body.file/body.dir
// reference) against baseDir, returning an absolute path. When restrict is
// true, the result is rejected if it would escape baseDir (guards against
// "../../etc/passwd"-style traversal from a config file; not applied for
// the "mockly add page" CLI, which resolves against the user's own
// invocation directory and has no equivalent trust boundary to enforce).
func resolveBodyPath(raw, baseDir string, restrict bool) (string, error) {
	p := raw
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolving path %q: %w", raw, err)
	}
	if restrict {
		baseAbs, err := filepath.Abs(baseDir)
		if err != nil {
			return "", fmt.Errorf("resolving base directory %q: %w", baseDir, err)
		}
		rel, err := filepath.Rel(baseAbs, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("path %q escapes config directory %q", raw, baseDir)
		}
	}
	return abs, nil
}

// ExpandHTTPMocks resolves every body.file/body.dir reference in mocks
// (relative to baseDir — see resolveBodyPath) and expands any mock whose
// Response.Body is a {dir: ...} reference into one generated HTTPMock per
// file found underneath it, per issue #202's "Directory expansion"
// section. Non-file, non-dir mocks pass through completely unchanged,
// still in their original relative order; a dir mock is replaced in-place
// by its generated children.
//
// restrict controls whether resolved paths are allowed to escape baseDir
// (true for config.Load; false for the "mockly add page" CLI command,
// which shares this same helper — see cmd/mockly/mocks.go).
//
// After expansion, it's an error for two mocks (hand-written or generated)
// to end up with the same (Method, Request.Path) pair — this always
// catches a dir expansion colliding with another mock, even though the
// uniqueness check itself only runs once, over the fully expanded list.
func ExpandHTTPMocks(mocks []HTTPMock, baseDir string, restrict bool) ([]HTTPMock, error) {
	out := make([]HTTPMock, 0, len(mocks))
	// generated tracks, in lockstep with out, whether each entry came from
	// a body.dir expansion — used below to scope the duplicate-path check
	// (see the comment at that check for why).
	generated := make([]bool, 0, len(mocks))
	for _, m := range mocks {
		switch {
		case m.Response.Body.Dir != "":
			children, err := expandDirMock(m, baseDir, restrict)
			if err != nil {
				return nil, err
			}
			out = append(out, children...)
			for range children {
				generated = append(generated, true)
			}
		case m.Response.Body.File != "":
			resolved, err := resolveBodyPath(m.Response.Body.File, baseDir, restrict)
			if err != nil {
				return nil, fmt.Errorf("mock %q: body.file: %w", m.ID, err)
			}
			if _, statErr := os.Stat(resolved); statErr != nil {
				return nil, fmt.Errorf("mock %q: body.file %q: %w", m.ID, m.Response.Body.File, statErr)
			}
			m.Response.Body.resolvedFile = resolved
			if err := resolveSequenceBodies(m.Sequence, baseDir, restrict); err != nil {
				return nil, fmt.Errorf("mock %q: %w", m.ID, err)
			}
			out = append(out, m)
			generated = append(generated, false)
		default:
			if err := resolveSequenceBodies(m.Sequence, baseDir, restrict); err != nil {
				return nil, fmt.Errorf("mock %q: %w", m.ID, err)
			}
			out = append(out, m)
			generated = append(generated, false)
		}
	}

	// Hand-written mocks are allowed to legitimately share the same
	// (Method, Path) pair — e.g. differentiated by query params, headers,
	// body_json, or a "state:" condition for state-machine-style branching
	// — so we only flag a collision when at least one side of it came from
	// a body.dir expansion: that's the one case where a silent duplicate
	// (two files routing to the same URL, or a generated page silently
	// shadowing/being shadowed by a hand-written mock) is always a mistake.
	type owner struct {
		id        string
		generated bool
	}
	seen := make(map[string]owner, len(out)) // "METHOD path" -> first owner seen
	for i, m := range out {
		key := strings.ToUpper(m.Request.Method) + " " + m.Request.Path
		if prev, ok := seen[key]; ok && (prev.generated || generated[i]) {
			return nil, fmt.Errorf("mock %q: request %s conflicts with mock %q (duplicate method+path, likely from a body.dir expansion)", m.ID, key, prev.id)
		}
		if _, ok := seen[key]; !ok {
			seen[key] = owner{id: m.ID, generated: generated[i]}
		}
	}
	return out, nil
}

// resolveSequenceBodies resolves body.file references (not body.dir, which
// isn't supported inside Sequence — see expandDirMock's sibling check
// below) for every entry of a Sequence slice, in place.
func resolveSequenceBodies(seq []HTTPResponse, baseDir string, restrict bool) error {
	for i := range seq {
		if seq[i].Body.Dir != "" {
			return fmt.Errorf("sequence[%d].body: \"dir\" is not supported inside sequence entries, only on a mock's top-level response", i)
		}
		if seq[i].Body.File == "" {
			continue
		}
		resolved, err := resolveBodyPath(seq[i].Body.File, baseDir, restrict)
		if err != nil {
			return fmt.Errorf("sequence[%d].body.file: %w", i, err)
		}
		if _, statErr := os.Stat(resolved); statErr != nil {
			return fmt.Errorf("sequence[%d].body.file %q: %w", i, seq[i].Body.File, statErr)
		}
		seq[i].Body.resolvedFile = resolved
	}
	return nil
}

// expandDirMock expands a single HTTPMock whose Response.Body.Dir is set
// into one HTTPMock per file found (recursively) under that directory,
// skipping dotfiles/dot-directories. See issue #202's path-mapping table:
// each file's slash-separated path relative to the directory root is
// path.Join-ed onto the parent mock's own request.path (URL semantics, not
// OS separators); an "index.html" additionally registers the parent path
// itself verbatim (standard "directory index" convention), at every depth.
func expandDirMock(m HTTPMock, baseDir string, restrict bool) ([]HTTPMock, error) {
	if m.Request.PathRegex != "" {
		return nil, fmt.Errorf("mock %q: body.dir requires a static request.path, not path_regex", m.ID)
	}
	if strings.ContainsAny(m.Request.Path, "*{") || strings.HasPrefix(m.Request.Path, "re:") {
		return nil, fmt.Errorf("mock %q: body.dir requires a static, literal request.path (no wildcards, named params, or templating)", m.ID)
	}
	if len(m.Sequence) > 0 {
		return nil, fmt.Errorf("mock %q: body.dir cannot be combined with sequence", m.ID)
	}

	dirRoot, err := resolveBodyPath(m.Response.Body.Dir, baseDir, restrict)
	if err != nil {
		return nil, fmt.Errorf("mock %q: body.dir: %w", m.ID, err)
	}
	info, err := os.Stat(dirRoot)
	if err != nil {
		return nil, fmt.Errorf("mock %q: body.dir %q: %w", m.ID, m.Response.Body.Dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("mock %q: body.dir %q is not a directory", m.ID, m.Response.Body.Dir)
	}

	parentPath := strings.TrimSuffix(m.Request.Path, "/")
	if parentPath == "" {
		parentPath = "/"
	}
	baseID := m.ID
	if baseID == "" {
		baseID = strings.Trim(strings.ReplaceAll(parentPath, "/", "-"), "-")
		if baseID == "" {
			baseID = "page"
		}
	}

	var relFiles []string
	walkErr := filepath.WalkDir(dirRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if p != dirRoot && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dirRoot, p)
		if relErr != nil {
			return relErr
		}
		relFiles = append(relFiles, filepath.ToSlash(rel))
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("mock %q: walking body.dir %q: %w", m.ID, m.Response.Body.Dir, walkErr)
	}
	sort.Strings(relFiles)
	if len(relFiles) == 0 {
		return nil, fmt.Errorf("mock %q: body.dir %q contains no files", m.ID, m.Response.Body.Dir)
	}

	children := make([]HTTPMock, 0, len(relFiles)+1)
	for _, rel := range relFiles {
		absFile := filepath.Join(dirRoot, filepath.FromSlash(rel))
		idSuffix := "__" + strings.ReplaceAll(rel, "/", "__")
		routePath := joinURLPath(parentPath, rel)

		fileChild := m
		fileChild.ID = baseID + idSuffix
		fileChild.Request = HTTPRequest{Method: "GET", Path: routePath}
		fileChild.Response = HTTPResponse{
			Status:  valueOr(m.Response.Status, 200),
			Headers: m.Response.Headers,
			// File is set to the fully resolved absolute path (not the
			// dir-relative name) so the generated mock is self-contained:
			// resolvedFile is unexported and never survives a JSON
			// round-trip, and the "mockly add page --dir" CLI command
			// expands directories client-side then POSTs each resulting
			// mock to the (possibly different-cwd) API server, which must
			// be able to resolve File on its own.
			Body:  HTTPBody{File: absFile, resolvedFile: absFile},
			Delay: m.Response.Delay,
		}
		fileChild.Sequence = nil
		children = append(children, fileChild)

		if path.Base(rel) == "index.html" {
			indexChild := fileChild
			indexChild.ID = baseID
			indexChild.Request = HTTPRequest{Method: "GET", Path: parentPath}
			children = append(children, indexChild)
		}
	}
	return children, nil
}

// joinURLPath joins base and rel using URL (forward-slash) semantics,
// regardless of OS, matching issue #202's "Join with URL semantics, not OS
// semantics" rule.
func joinURLPath(base, rel string) string {
	if base == "/" || base == "" {
		return "/" + rel
	}
	return path.Join(base, rel)
}

func valueOr(v, fallback int) int {
	if v != 0 {
		return v
	}
	return fallback
}
