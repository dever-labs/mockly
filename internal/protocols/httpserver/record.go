package httpserver

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dever-labs/mockly/internal/config"
)

// hopByHopHeaders are stripped in both directions when proxying, mirroring
// standard reverse-proxy behavior (RFC 7230 §6.1 plus Content-Length/Host,
// which must be recalculated rather than forwarded verbatim).
var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailers":            {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"Content-Length":      {},
	"Host":                {},
}

// requestOnlyStrippedHeaders are additionally stripped from the outgoing
// (client → upstream) request only. Accept-Encoding is dropped so the
// upstream always answers uncompressed: Go's http.Transport only
// auto-decompresses a gzip response when *it* added the Accept-Encoding
// header, so forwarding the caller's own header verbatim would capture
// compressed, non-human-readable bodies into the recorded mock/save_to
// file.
var requestOnlyStrippedHeaders = map[string]struct{}{
	"Accept-Encoding": {},
}

// recorder implements HTTP record mode: it proxies requests that didn't
// match any existing mock to a real upstream Target, returns the real
// response to the caller, and captures it as a new mock so subsequent
// identical requests are replayed locally without hitting Target again.
type recorder struct {
	target string
	saveTo string
	client *http.Client

	mu       sync.Mutex
	counter  int
	recorded []config.HTTPMock
}

func newRecorder(cfg *config.HTTPRecordConfig) *recorder {
	return &recorder{
		target: strings.TrimRight(cfg.Target, "/"),
		saveTo: cfg.SaveTo,
		client: &http.Client{
			Timeout: 30 * time.Second,
			// Never follow redirects ourselves — relay the upstream's
			// response (including any 3xx) verbatim to the caller.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// forward replays the given request against the recorder's Target and
// returns the upstream's response. body is the already-read request body
// (the caller has consumed r.Body for matching, so it's passed explicitly).
func (rec *recorder) forward(r *http.Request, body []byte) (status int, headers map[string]string, respBody []byte, err error) {
	url := rec.target + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("record: building upstream request: %w", err)
	}
	for k, v := range r.Header {
		ck := http.CanonicalHeaderKey(k)
		if _, hop := hopByHopHeaders[ck]; hop {
			continue
		}
		if _, stripped := requestOnlyStrippedHeaders[ck]; stripped {
			continue
		}
		outReq.Header[k] = v
	}

	resp, err := rec.client.Do(outReq)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("record: proxying to %s: %w", rec.target, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("record: reading upstream response: %w", err)
	}

	headers = make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if _, hop := hopByHopHeaders[http.CanonicalHeaderKey(k)]; hop {
			continue
		}
		headers[k] = strings.Join(v, ", ")
	}

	return resp.StatusCode, headers, respBody, nil
}

// capture records a real request/response pair as a new HTTPMock, appends
// it to the in-memory recorded set, and (if SaveTo is configured) persists
// the full recorded set to disk. The returned mock still needs to be added
// to the live server's mock list by the caller so it's immediately replayed.
//
// If an equivalent request (same method/path/query) was already recorded —
// e.g. two concurrent requests for the same not-yet-recorded endpoint raced
// each other — the existing mock is returned unchanged instead of appending
// a duplicate, so the recorded set (and save_to file) stays de-duplicated.
func (rec *recorder) capture(method, path string, query map[string]string, status int, headers map[string]string, body []byte) config.HTTPMock {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(query) == 0 {
		query = nil
	}
	if existing, ok := rec.findLocked(method, path, query); ok {
		return existing
	}

	rec.counter++
	mock := config.HTTPMock{
		ID:      fmt.Sprintf("recorded-%d", rec.counter),
		Request: config.HTTPRequest{Method: method, Path: path, Query: query},
		Response: config.HTTPResponse{
			Status:  status,
			Headers: headers,
			Body:    string(body),
		},
	}
	rec.recorded = append(rec.recorded, mock)

	if rec.saveTo != "" {
		if err := rec.persistLocked(); err != nil {
			fmt.Fprintf(os.Stderr, "warn: record: saving %s: %v\n", rec.saveTo, err)
		}
	}
	return mock
}

// findLocked returns a previously recorded mock matching the given
// method/path/query, if any. Must be called with rec.mu held.
func (rec *recorder) findLocked(method, path string, query map[string]string) (config.HTTPMock, bool) {
	for _, m := range rec.recorded {
		if m.Request.Method == method && m.Request.Path == path && queryEqual(m.Request.Query, query) {
			return m, true
		}
	}
	return config.HTTPMock{}, false
}

func queryEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// persistLocked writes every mock recorded so far to SaveTo as a standalone
// YAML document (just the "mocks:" list, ready to paste under
// protocols.http in a hand-written config). Must be called with rec.mu held.
func (rec *recorder) persistLocked() error {
	out := struct {
		Mocks []config.HTTPMock `yaml:"mocks"`
	}{Mocks: rec.recorded}
	data, err := yaml.Marshal(out)
	if err != nil {
		return err
	}
	return os.WriteFile(rec.saveTo, data, 0o644)
}
