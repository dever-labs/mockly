// Package engine provides the core request matching logic and response rendering
// used by all protocol servers.
package engine

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/state"
)

// MatchResult is the result of matching a request against a set of mocks.
type MatchResult struct {
	Matched bool
	MockID  string
	Body    string
	Headers map[string]string
	Status  int
	Delay   time.Duration
}

// HTTPMatch attempts to find the first HTTPMock matching the given request fields.
// It respects state conditions when a Store is provided.
func HTTPMatch(
	mocks []config.HTTPMock,
	method, path string,
	headers map[string]string,
	body string,
	store *state.Store,
) (MatchResult, bool) {
	for _, m := range mocks {
		if !matchMethod(m.Request.Method, method) {
			continue
		}
		if !matchPath(m.Request.Path, path) {
			continue
		}
		if !matchHeaders(m.Request.Headers, headers) {
			continue
		}
		if m.Request.Body != "" && !matchBody(m.Request.Body, body) {
			continue
		}
		if !matchState(m.State, store) {
			continue
		}

		rendered, err := renderTemplate(m.Response.Body, headers, body)
		if err != nil {
			rendered = m.Response.Body
		}

		status := m.Response.Status
		if status == 0 {
			status = 200
		}

		return MatchResult{
			Matched: true,
			MockID:  m.ID,
			Body:    rendered,
			Headers: m.Response.Headers,
			Status:  status,
			Delay:   m.Response.Delay.Duration,
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

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func matchMethod(want, got string) bool {
	if want == "" || want == "*" {
		return true
	}
	return strings.EqualFold(want, got)
}

// matchPath supports exact matches, prefix wildcards (/*), and regex (/re:…).
func matchPath(pattern, path string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(strings.TrimPrefix(pattern, "re:"))
		if err != nil {
			return false
		}
		return re.MatchString(path)
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	return pattern == path
}

func matchPattern(pattern, text string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(strings.TrimPrefix(pattern, "re:"))
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

func matchBody(pattern, body string) bool {
	return matchPattern(pattern, body)
}

func matchState(cond *config.StateCondition, store *state.Store) bool {
	if cond == nil || store == nil {
		return true
	}
	v, _ := store.Get(cond.Key)
	return v == cond.Value
}

// renderTemplate executes a Go text/template against a simple context object.
func renderTemplate(tmpl string, headers map[string]string, body string) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	t, err := template.New("response").Funcs(template.FuncMap{
		"now": func() string { return time.Now().Format(time.RFC3339) },
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
	}).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	ctx := map[string]interface{}{
		"headers": headers,
		"body":    body,
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}
