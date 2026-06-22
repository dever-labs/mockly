// Package engine provides the core request matching logic and response rendering
// used by all protocol servers.
package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/state"
)

// regexCache caches compiled regexes by pattern to avoid recompiling per request.
var regexCache sync.Map // map[string]*regexp.Regexp

// templateCache caches parsed templates by template string.
var templateCache sync.Map // map[string]*template.Template

// CachedRegex returns a cached *regexp.Regexp for pattern, compiling on first use.
// Exported so protocol servers can share the same cache without reimporting regexp.
func CachedRegex(pattern string) (*regexp.Regexp, error) {
	return compiledRegex(pattern)
}

// compiledRegex returns a cached *regexp.Regexp for pattern, compiling it on first use.
func compiledRegex(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		return v.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.Store(pattern, re)
	return re, nil
}

// cachedTemplate returns a cached *template.Template for tmpl, parsing it on first use.
func cachedTemplate(tmpl string) (*template.Template, error) {
	if v, ok := templateCache.Load(tmpl); ok {
		return v.(*template.Template), nil
	}
	t, err := template.New("response").Funcs(BuildFuncMap()).Parse(tmpl)
	if err != nil {
		return nil, err
	}
	templateCache.Store(tmpl, t)
	return t, nil
}

// MatchResult is the result of matching a request against a set of mocks.
type MatchResult struct {
	Matched    bool
	MockID     string
	Body       string
	Headers    map[string]string
	Status     int
	Delay      time.Duration
	Fault      *config.MockFault
	PathParams map[string]string // named path parameter captures (e.g. {region} → "fr-par")
}

// HTTPMatch attempts to find the first HTTPMock matching the given request fields.
// It respects state conditions when a Store is provided.
func HTTPMatch(
	mocks []config.HTTPMock,
	method, path string,
	query map[string]string,
	headers map[string]string,
	body string,
	store *state.Store,
) (MatchResult, bool) {
	for _, m := range mocks {
		if !matchMethod(m.Request.Method, method) {
			continue
		}
		var (
			pathMatched bool
			pathParams  map[string]string
		)
		if m.Request.PathRegex != "" {
			re, err := compiledRegex(m.Request.PathRegex)
			if err != nil || !re.MatchString(path) {
				continue
			}
			pathMatched = true
		} else {
			pathMatched, pathParams = matchPath(m.Request.Path, path)
		}
		if !pathMatched {
			continue
		}
		if !matchHeaders(m.Request.Headers, headers) {
			continue
		}
		if !matchQuery(m.Request.Query, query) {
			continue
		}
		if m.Request.Body != "" && !matchBody(m.Request.Body, body) {
			continue
		}
		if len(m.Request.BodyJSON) > 0 && !matchBodyJSON(m.Request.BodyJSON, body) {
			continue
		}
		if !matchState(m.State, store) {
			continue
		}

		req := RequestContext{
			Method:     method,
			Path:       path,
			Query:      query,
			Headers:    headers,
			Body:       body,
			PathParams: pathParams,
		}

		rendered, err := renderTemplate(m.Response.Body, req)
		if err != nil {
			rendered = m.Response.Body
		}

		renderedHeaders := make(map[string]string, len(m.Response.Headers))
		for k, v := range m.Response.Headers {
			rv, herr := renderTemplate(v, req)
			if herr != nil {
				rv = v
			}
			renderedHeaders[k] = rv
		}

		status := m.Response.Status
		if status == 0 {
			status = 200
		}

		return MatchResult{
			Matched:    true,
			MockID:     m.ID,
			Body:       rendered,
			Headers:    renderedHeaders,
			Status:     status,
			Delay:      m.Response.Delay.Duration,
			Fault:      m.Fault,
			PathParams: pathParams,
		}, true
	}
	return MatchResult{}, false
}

// WSMatch finds the first WebSocketRule matching the given message text.
func WSMatch(rules []config.WebSocketRule, message string) (config.WebSocketRule, bool) {
	for _, r := range rules {
		if matchPattern(r.Match, message) {
			return r, true
		}
	}
	return config.WebSocketRule{}, false
}

// MatchPath is the exported form of matchPath for use by protocol servers
// that perform their own path/URI matching outside of HTTPMatch.
func MatchPath(pattern, path string) (bool, map[string]string) {
	return matchPath(pattern, path)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func matchMethod(want, got string) bool {
	if want == "" || want == "*" {
		return true
	}
	return strings.EqualFold(want, got)
}

// matchPath supports exact matches, prefix wildcards (/*), mid-segment wildcards,
// named parameters, and regex (/re:…).
//
// Wildcard rules:
//   - "/*" suffix  – matches the prefix and any path beneath it (multi-segment).
//   - "*" segment  – each * matches exactly one path segment (no slashes);
//     the total segment count must match.
//   - "{name}"     – like * but also captures the segment value in the params map
//     (e.g. "{region}" on "fr-par" → {"region":"fr-par"}).
func matchPath(pattern, path string) (bool, map[string]string) {
	if pattern == "" || pattern == "*" {
		return true, nil
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := compiledRegex(strings.TrimPrefix(pattern, "re:"))
		if err != nil {
			return false, nil
		}
		return re.MatchString(path), nil
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		matched := path == prefix || strings.HasPrefix(path, prefix+"/")
		return matched, nil
	}
	if strings.Contains(pattern, "*") || (strings.Contains(pattern, "{") && strings.Contains(pattern, "}")) {
		return matchPathSegments(pattern, path)
	}
	return pattern == path, nil
}

// matchPathSegments splits pattern and path on "/" and matches segment by
// segment. A "*" pattern segment matches exactly one path segment (anonymous
// wildcard). A "{name}" segment also matches one segment and captures its value
// under the given name (e.g. "{id}" on "42" → params["id"]="42").
func matchPathSegments(pattern, path string) (bool, map[string]string) {
	pp := strings.Split(pattern, "/")
	sp := strings.Split(path, "/")
	if len(pp) != len(sp) {
		return false, nil
	}
	var params map[string]string
	for i, seg := range pp {
		if seg == "*" {
			continue
		}
		// {name} named parameter
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			if name := seg[1 : len(seg)-1]; name != "" {
				if params == nil {
					params = make(map[string]string)
				}
				params[name] = sp[i]
			}
			continue
		}
		if seg != sp[i] {
			return false, nil
		}
	}
	return true, params
}

func matchPattern(pattern, text string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := compiledRegex(strings.TrimPrefix(pattern, "re:"))
		if err != nil {
			return false
		}
		return re.MatchString(text)
	}
	return strings.Contains(text, pattern)
}

func matchHeaders(want, got map[string]string) bool {
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// matchQuery checks that all required query params are present in the request.
// A value of "*" matches any value for that key.
func matchQuery(want, got map[string]string) bool {
	for k, v := range want {
		actual, ok := got[k]
		if !ok {
			return false
		}
		if v != "*" && v != actual {
			return false
		}
	}
	return true
}

func matchBody(pattern, body string) bool {
	return matchPattern(pattern, body)
}

// matchBodyJSON checks that dot-notation paths in want resolve to the expected
// values in the JSON body. The dot path "user.role" resolves to
// {"user":{"role":"..."}}.  Values are compared as strings; numeric JSON values
// are normalised (e.g. 42 == "42").
func matchBodyJSON(want map[string]string, body string) bool {
	if body == "" {
		return false
	}
	var root interface{}
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return false
	}
	for dotPath, expected := range want {
		actual, ok := jsonPathGet(root, strings.Split(dotPath, "."))
		if !ok {
			return false
		}
		if expected == "*" {
			continue
		}
		if jsonValueString(actual) != expected {
			return false
		}
	}
	return true
}

// jsonPathGet traverses a decoded JSON value following the given key segments.
func jsonPathGet(v interface{}, path []string) (interface{}, bool) {
	if len(path) == 0 {
		return v, true
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, false
	}
	next, ok := m[path[0]]
	if !ok {
		return nil, false
	}
	return jsonPathGet(next, path[1:])
}

// jsonValueString converts a decoded JSON value to a comparable string.
func jsonValueString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return "null"
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func matchState(cond *config.StateCondition, store *state.Store) bool {
	if cond == nil || store == nil {
		return true
	}
	v, _ := store.Get(cond.Key)
	return v == cond.Value
}

// RequestContext carries the inbound request fields that are available inside
// response body / header templates.
type RequestContext struct {
	Method     string
	Path       string
	Query      map[string]string
	Headers    map[string]string
	Body       string
	PathParams map[string]string
}

// renderTemplate executes a Go text/template against a context that includes
// both the legacy top-level keys (query, headers, body) and a nested
// "request" key with method, path, params, and the body parsed as JSON.
func renderTemplate(tmpl string, req RequestContext) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	t, err := cachedTemplate(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	// Parse body as JSON for {{.request.body.field}} access.
	// Falls back to an empty map so templates don't panic on non-JSON bodies.
	var bodyJSON interface{}
	if err := json.Unmarshal([]byte(req.Body), &bodyJSON); err != nil {
		bodyJSON = map[string]interface{}{}
	}

	ctx := map[string]interface{}{
		// Backward-compatible top-level keys.
		"query":   req.Query,
		"headers": req.Headers,
		"body":    req.Body,
		// Namespaced request context.
		"request": map[string]interface{}{
			"method":  req.Method,
			"path":    req.Path,
			"query":   req.Query,
			"headers": req.Headers,
			"body":    bodyJSON,
			"params":  req.PathParams,
		},
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}

// Render is the exported counterpart of renderTemplate for use by protocol
// servers that need to template-render a string outside of HTTPMatch
// (e.g. sequence entry bodies/headers and scenario patch bodies/headers).
// On template error it returns the original string unchanged.
func Render(tmpl string, req RequestContext) string {
	out, err := renderTemplate(tmpl, req)
	if err != nil {
		return tmpl
	}
	return out
}
