// Package httpserver implements the HTTP mock server.
package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/tlsutil"
	"github.com/dever-labs/mockly/internal/webhook"
)

// Server is the HTTP mock server.
type Server struct {
	cfg       *config.HTTPConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger
	webhooks  *webhook.Sender

	mu         sync.RWMutex
	mocks      []config.HTTPMock
	callCounts map[string]int64 // mock ID → total call count (for sequences + API)

	recorder *recorder // non-nil when HTTP record mode is enabled

	server *http.Server
}

// New creates a Server. The mocks slice is taken from cfg initially but can
// be replaced at runtime via SetMocks. wh may be nil, in which case any
// mock-attached webhooks are silently skipped (used by tests that don't
// care about outbound callbacks).
func New(cfg *config.HTTPConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger, wh *webhook.Sender) *Server {
	s := &Server{
		cfg:        cfg,
		store:      store,
		scenarios:  sc,
		log:        log,
		webhooks:   wh,
		mocks:      append([]config.HTTPMock(nil), cfg.Mocks...),
		callCounts: make(map[string]int64),
	}
	if cfg.Record != nil && cfg.Record.Enabled {
		s.recorder = newRecorder(cfg.Record)
	}
	return s
}

// SetMocks replaces the current mock list and resets all call counts.
func (s *Server) SetMocks(mocks []config.HTTPMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.HTTPMock(nil), mocks...)
	s.callCounts = make(map[string]int64)
}

// GetMocks returns the current mock list.
func (s *Server) GetMocks() []config.HTTPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.HTTPMock(nil), s.mocks...)
}

// CallCount returns how many times the mock with the given ID has been called.
func (s *Server) CallCount(mockID string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.callCounts[mockID]
}

// ResetCallCounts zeroes all call counters.
func (s *Server) ResetCallCounts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCounts = make(map[string]int64)
}

// Start begins listening. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.HandleFunc("/*", s.handleRequest)

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.server = &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("http mock server listen %s: %w", addr, err)
	}
	ln, err = tlsutil.WrapListener(ln, s.cfg.TLS)
	if err != nil {
		return fmt.Errorf("http mock server tls: %w", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	bodyReader := io.Reader(r.Body)
	if s.cfg.MaxBodyBytes > 0 {
		bodyReader = io.LimitReader(r.Body, s.cfg.MaxBodyBytes)
	}
	body, err := io.ReadAll(bodyReader)
	defer r.Body.Close() //nolint:errcheck
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	hdrs := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		hdrs[k] = strings.Join(v, ", ")
	}

	queryValues := r.URL.Query()
	querySingle := make(map[string]string, len(queryValues))
	for k, v := range queryValues {
		if len(v) > 0 {
			querySingle[k] = v[0]
		}
	}

	// Protocol fault: inject latency before processing, and rate limiting
	// short-circuits everything else, simulating a throttled API returning
	// e.g. 429 once the configured requests-per-second is exceeded.
	fault := s.scenarios.EffectiveHTTPFault()
	if fault != nil && fault.RateLimit != nil &&
		s.scenarios.RateLimited("http:global", fault.RateLimit.RequestsPerSecond) {
		s.respondRateLimited(w, r, hdrs, string(body), start, fault.RateLimit)
		return
	}
	if fault != nil {
		if d := s.scenarios.ResolveDelay(fault.Delay.Duration, fault.DelayRange); d > 0 {
			time.Sleep(d)
		}
	}

	s.mu.RLock()
	mocks := s.mocks
	s.mu.RUnlock()

	// NTLM pre-flight: if any mock for this path/method requires NTLM auth,
	// handle the 3-step handshake before normal matching.
	if handled := s.handleNTLM(w, r, mocks, hdrs); handled {
		return
	}

	result, matched := engine.HTTPMatch(mocks, r.Method, r.URL.Path, queryValues, hdrs, string(body), s.store)

	// Record mode: an unmatched request is transparently proxied to the
	// real upstream and captured as a new mock, so every subsequent
	// identical request is replayed locally instead of reaching Target
	// again. Falls through to the normal "no mock matched" response if the
	// upstream can't be reached.
	if !matched && s.recorder != nil {
		if s.recordAndServe(w, r, hdrs, querySingle, body, start) {
			return
		}
	}

	status := http.StatusNotFound
	respBody := `{"error":"no mock matched"}`
	respHdrs := map[string]string{"Content-Type": "application/json"}
	matchedID := ""
	delay := time.Duration(0)
	var stream *config.HTTPStream
	var reqCtx engine.RequestContext

	// Opt-in near-miss diagnostics: only computed (and only changes the
	// response) when explicitly requested, so normal 404 behavior for real
	// client integrations is unchanged.
	var nearMisses []engine.NearMiss
	if !matched && debugRequested(r) {
		nearMisses = engine.HTTPDiagnose(mocks, r.Method, r.URL.Path, queryValues, hdrs, string(body), s.store)
		if len(nearMisses) > 0 {
			if b, err := json.Marshal(map[string]interface{}{
				"error":       "no mock matched",
				"near_misses": nearMisses,
			}); err == nil {
				respBody = string(b)
			}
		}
	}

	if matched {
		status = result.Status
		respBody = result.Body
		respHdrs = result.Headers
		matchedID = result.MockID
		delay = result.Delay
		stream = result.Stream

		reqCtx = engine.RequestContext{
			Method:     r.Method,
			Path:       r.URL.Path,
			Query:      querySingle,
			Headers:    hdrs,
			Body:       string(body),
			PathParams: result.PathParams,
		}

		// Increment call counter and select sequence response if configured.
		s.mu.Lock()
		s.callCounts[matchedID]++
		callN := s.callCounts[matchedID] // 1-based
		s.mu.Unlock()

		// Find the matched mock for sequence + per-mock fault.
		var matchedMock *config.HTTPMock
		for i := range mocks {
			if mocks[i].ID == matchedID {
				matchedMock = &mocks[i]
				break
			}
		}

		// Fire any outbound webhooks configured on this mock. Dispatch is
		// asynchronous and never blocks or affects the HTTP response below.
		if matchedMock != nil && s.webhooks != nil {
			for _, wh := range matchedMock.Webhooks {
				s.webhooks.Dispatch("http", matchedMock.ID, wh, reqCtx)
			}
		}

		if matchedMock != nil && len(matchedMock.Sequence) > 0 {
			stream = nil
			idx := int(callN) - 1 // 0-based
			seq := matchedMock.Sequence
			switch {
			case idx < len(seq):
				entry := seq[idx]
				if entry.Status != 0 {
					status = entry.Status
				}
				if entry.Body != "" {
					respBody = engine.Render(entry.Body, reqCtx)
				}
				for k, v := range entry.Headers {
					respHdrs[k] = engine.Render(v, reqCtx)
				}
				if entry.Delay.Duration > 0 {
					delay = entry.Delay.Duration
				}
			default:
				exhausted := matchedMock.SequenceExhausted
				if exhausted == "" {
					exhausted = config.SequenceExhaustedHoldLast
				}
				switch exhausted {
				case config.SequenceExhaustedLoop:
					loopIdx := (idx) % len(seq)
					entry := seq[loopIdx]
					if entry.Status != 0 {
						status = entry.Status
					}
					if entry.Body != "" {
						respBody = engine.Render(entry.Body, reqCtx)
					}
					for k, v := range entry.Headers {
						respHdrs[k] = engine.Render(v, reqCtx)
					}
					if entry.Delay.Duration > 0 {
						delay = entry.Delay.Duration
					}
				case config.SequenceExhaustedNotFound:
					status = http.StatusNotFound
					respBody = `{"error":"sequence exhausted"}`
				default: // hold_last
					entry := seq[len(seq)-1]
					if entry.Status != 0 {
						status = entry.Status
					}
					if entry.Body != "" {
						respBody = engine.Render(entry.Body, reqCtx)
					}
					for k, v := range entry.Headers {
						respHdrs[k] = engine.Render(v, reqCtx)
					}
					if entry.Delay.Duration > 0 {
						delay = entry.Delay.Duration
					}
				}
			}
		}

		// Apply the first active scenario patch for this mock (if any).
		if patch := s.scenarios.PatchFor(matchedID); patch != nil {
			stream = nil
			if patch.Disabled {
				status = http.StatusNotFound
				respBody = `{"error":"mock disabled by active scenario"}`
				matchedID = ""
				delay = 0
			} else {
				if patch.Status != 0 {
					status = patch.Status
				}
				if patch.Body != "" {
					respBody = engine.Render(patch.Body, reqCtx)
				}
				for k, v := range patch.Headers {
					respHdrs[k] = engine.Render(v, reqCtx)
				}
				if patch.Delay != nil {
					delay = patch.Delay.Duration
				}
			}
		}

		// Per-mock fault (applied after scenario patches).
		if matchedMock != nil && matchedMock.Fault != nil {
			mf := matchedMock.Fault
			delay += s.scenarios.ResolveDelay(mf.Delay.Duration, mf.DelayRange)
			switch {
			case mf.RateLimit != nil && s.scenarios.RateLimited("http:mock:"+matchedMock.ID, mf.RateLimit.RequestsPerSecond):
				stream = nil
				status = mf.RateLimit.OverLimitStatus
				if status == 0 {
					status = http.StatusTooManyRequests
				}
				if mf.RateLimit.Body != "" {
					respBody = mf.RateLimit.Body
				} else {
					respBody = `{"error":"rate limit exceeded"}`
				}
			case mf.StatusOverride != 0 && s.scenarios.ShouldFault(mf.ErrorRate):
				stream = nil
				status = mf.StatusOverride
				if mf.Body != "" {
					respBody = mf.Body
				}
				for k, v := range mf.Headers {
					respHdrs[k] = v
				}
			}
		}
	}

	// Protocol fault: probabilistically override status/body/headers.
	// Only fires if status or body is explicitly set — a delay-only fault
	// injects latency without altering the response.
	// The default error body is only added for error status codes (≥ 400);
	// 1xx/2xx/3xx faults (e.g. redirects) leave the response body unchanged
	// unless an explicit body is provided.
	faultFires := fault != nil && s.scenarios.RollFault(fault.ErrorRate) &&
		(fault.Abort || fault.TruncateBody > 0 || fault.Status != 0 || fault.Body != "")

	if faultFires && !fault.Abort {
		stream = nil
		// Apply status/body/header overrides regardless of truncate; truncate uses them.
		if fault.Status != 0 || fault.Body != "" {
			status = fault.Status
			if status == 0 {
				status = http.StatusServiceUnavailable
			}
			if fault.Body != "" {
				respBody = fault.Body
			} else if status >= 400 {
				respBody = `{"error":"fault injected"}`
			}
		}
		for k, v := range fault.Headers {
			respHdrs[k] = v
		}
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	// Connection-level faults: evaluated after delay, before writing response.
	if faultFires && fault.Abort {
		abortConn(w)
		return
	}
	if faultFires && fault.TruncateBody > 0 {
		truncateResponse(w, status, respHdrs, respBody, fault.TruncateBody)
		return
	}

	for k, v := range respHdrs {
		w.Header().Set(k, v)
	}
	if stream != nil && len(stream.Events) > 0 && w.Header().Get("Content-Type") == "" {
		// Guard against reflected XSS: streamed events may echo
		// request-derived data via templating, so default to a
		// non-renderable content type when the mock didn't specify one
		// (mirrors truncateResponse's existing Content-Type default).
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.WriteHeader(status)

	if stream != nil && len(stream.Events) > 0 {
		s.writeStream(w, respHdrs, stream, reqCtx)
	} else {
		_, _ = fmt.Fprint(w, respBody)
	}

	s.log.Log(logger.Entry{
		Protocol:   "http",
		Method:     r.Method,
		Path:       r.URL.Path,
		Status:     status,
		Duration:   time.Since(start).Milliseconds(),
		Headers:    hdrs,
		Body:       string(body),
		MatchedID:  matchedID,
		PathParams: result.PathParams,
		NearMisses: toLoggerNearMisses(nearMisses),
	})
}

// recordAndServe proxies r to the recorder's Target, writes the real
// response back to w, and captures it as a new mock appended to s.mocks so
// subsequent identical requests are replayed without reaching Target again.
// Returns false (writing nothing) if the upstream couldn't be reached, so
// the caller can fall back to the normal "no mock matched" response.
func (s *Server) recordAndServe(w http.ResponseWriter, r *http.Request, hdrs map[string]string, query map[string]string, body []byte, start time.Time) bool {
	status, respHdrs, respBody, err := s.recorder.forward(r, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: %v\n", err)
		return false
	}

	mock := s.recorder.capture(r.Method, r.URL.Path, query, status, respHdrs, respBody)
	s.mu.Lock()
	s.mocks = append(s.mocks, mock)
	s.mu.Unlock()

	for k, v := range respHdrs {
		w.Header().Set(k, v)
	}
	w.WriteHeader(status)
	_, _ = w.Write(respBody)

	s.log.Log(logger.Entry{
		Protocol:  "http",
		Method:    r.Method,
		Path:      r.URL.Path,
		Status:    status,
		Duration:  time.Since(start).Milliseconds(),
		Headers:   hdrs,
		Body:      string(body),
		MatchedID: mock.ID,
	})
	return true
}

// handleNTLM intercepts requests for mocks that require NTLM authentication and
// drives the 3-step handshake. It returns true when it has written a response
// (steps 1 and 2), meaning the caller should not process the request further.
// On step 3 (type-3 Authenticate token) it returns false so normal matching proceeds.
//
// Step 1 — no NTLM token present:
//
//	→ 401 Unauthorized + WWW-Authenticate: NTLM
//
// Step 2 — NTLM type-1 (Negotiate) token present:
//
//	→ 401 Unauthorized + WWW-Authenticate: NTLM <type2-challenge>
//
// Step 3 — NTLM type-3 (Authenticate) token present:
//
//	→ returns false; normal HTTPMatch runs and the mock is served.
func (s *Server) handleNTLM(w http.ResponseWriter, r *http.Request, mocks []config.HTTPMock, hdrs map[string]string) bool {
	// Check whether any mock on this path+method requires NTLM auth.
	requiresNTLM := false
	for i := range mocks {
		m := &mocks[i]
		if m.Request.Auth == nil || !strings.EqualFold(m.Request.Auth.Type, "ntlm") {
			continue
		}
		// Only activate NTLM handling when the path/method could match.
		pathOK, _ := engine.MatchPath(m.Request.Path, r.URL.Path)
		if m.Request.PathRegex != "" {
			if re, err := engine.CachedRegex(m.Request.PathRegex); err == nil {
				pathOK = re.MatchString(r.URL.Path)
			}
		}
		methodOK := m.Request.Method == "" || m.Request.Method == "*" ||
			strings.EqualFold(m.Request.Method, r.Method)
		if pathOK && methodOK {
			requiresNTLM = true
			break
		}
	}
	if !requiresNTLM {
		return false
	}

	authHdr := hdrs["Authorization"]
	if authHdr == "" {
		authHdr = hdrs["authorization"]
	}

	// Only intercept when the client is performing NTLM (no auth, or an NTLM
	// token). If the client sends a different scheme (Bearer, Basic, API key),
	// return false so the normal mock-matching pipeline can handle it — the
	// NTLM mock will simply not match via matchAuth.
	if authHdr != "" && !strings.HasPrefix(authHdr, "NTLM ") {
		return false
	}

	tokenType := NTLMTokenType(authHdr)
	switch tokenType {
	case 1:
		// Type-1 Negotiate: respond with a type-2 Challenge.
		w.Header().Set("WWW-Authenticate", "NTLM "+ntlmChallengeToken())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"NTLM negotiation in progress"}`)
		return true
	case 3:
		// Type-3 Authenticate: let normal matching proceed.
		return false
	default:
		// No token or unrecognised: initiate handshake.
		w.Header().Set("WWW-Authenticate", "NTLM")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"NTLM authentication required"}`)
		return true
	}
}

// StatusInfo returns JSON-serialisable info about this server.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tlsEnabled := s.cfg.TLS != nil && s.cfg.TLS.Enabled
	return map[string]interface{}{
		"protocol": "http",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"tls":      tlsEnabled,
		"mocks":    len(s.mocks),
	}
}

// MarshalMocks returns the mock list as JSON bytes.
func (s *Server) MarshalMocks() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.mocks)
}

// debugRequested reports whether the request opted into near-miss match
// diagnostics via "?debug=true" or an "X-Mockly-Debug: true" header.
func debugRequested(r *http.Request) bool {
	if r.URL.Query().Get("debug") == "true" {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Mockly-Debug"), "true")
}

// toLoggerNearMisses converts engine near-miss diagnostics to the logger
// package's own (dependency-free) NearMiss type for inclusion in log entries.
func toLoggerNearMisses(in []engine.NearMiss) []logger.NearMiss {
	if len(in) == 0 {
		return nil
	}
	out := make([]logger.NearMiss, len(in))
	for i, nm := range in {
		out[i] = logger.NearMiss{MockID: nm.MockID, Reason: nm.Reason}
	}
	return out
}

// respondRateLimited writes the configured over-limit response (default 429)
// for a rate_limit fault and logs the request. Used when a global (direct or
// scenario) HTTP fault's requests_per_second has been exceeded.
func (s *Server) respondRateLimited(w http.ResponseWriter, r *http.Request, hdrs map[string]string, body string, start time.Time, rl *config.RateLimitFault) {
	status := rl.OverLimitStatus
	if status == 0 {
		status = http.StatusTooManyRequests
	}
	respBody := rl.Body
	if respBody == "" {
		respBody = `{"error":"rate limit exceeded"}`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprint(w, respBody)

	s.log.Log(logger.Entry{
		Protocol: "http",
		Method:   r.Method,
		Path:     r.URL.Path,
		Status:   status,
		Duration: time.Since(start).Milliseconds(),
		Headers:  hdrs,
		Body:     body,
	})
}

// abortConn hijacks the connection and performs a TCP reset (RST) — the client
// receives a connection-reset error with no HTTP response.
func abortConn(w http.ResponseWriter) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetLinger(0) // triggers RST on close instead of FIN
	}
	_ = conn.Close()
}

// truncateResponse writes the response status and headers then sends only the
// first n bytes of body before abruptly closing the connection, simulating a
// mid-transfer server crash (client gets unexpected EOF).
func truncateResponse(w http.ResponseWriter, status int, headers map[string]string, body string, n int) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		// Fallback: just write normally when hijacking is unavailable.
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		// Ensure Content-Type is always set so browsers cannot sniff the body
		// as HTML and execute embedded scripts (reflected XSS).
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		return
	}
	truncated := body
	if n < len(truncated) {
		truncated = truncated[:n]
	}
	// Write raw HTTP/1.1 response with an inflated Content-Length so the client
	// expects more data than it will receive.
	claimedLen := len(body) + 128
	_, _ = fmt.Fprintf(buf, "HTTP/1.1 %d %s\r\n", status, http.StatusText(status))
	for k, v := range headers {
		_, _ = fmt.Fprintf(buf, "%s: %s\r\n", k, v)
	}
	_, _ = fmt.Fprintf(buf, "Content-Length: %d\r\nContent-Type: application/json\r\n\r\n", claimedLen)
	_, _ = buf.WriteString(truncated)
	_ = buf.Flush()
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	_ = conn.Close()
}

// writeStream writes a config.HTTPStream as a sequence of flushed events,
// sleeping each event's Delay beforehand. When the response's Content-Type
// header contains "text/event-stream" each event is framed as a Server-Sent
// Event (optional "event:"/"id:" lines, one "data:" line per line of
// rendered Data, then a blank line); otherwise the rendered Data is written
// and flushed as-is (plain chunked/flushed streaming, no SSE framing).
//
// If the ResponseWriter doesn't support http.Flusher (shouldn't happen with
// the standard net/http server used here), events are still written, just
// without incremental flushing.
func (s *Server) writeStream(w http.ResponseWriter, headers map[string]string, stream *config.HTTPStream, reqCtx engine.RequestContext) {
	fl, _ := w.(http.Flusher)
	isSSE := strings.Contains(strings.ToLower(headers["Content-Type"]), "text/event-stream")

	for _, ev := range stream.Events {
		if ev.Delay.Duration > 0 {
			time.Sleep(ev.Delay.Duration)
		}
		data := engine.Render(ev.Data, reqCtx)
		if isSSE {
			if ev.Event != "" {
				_, _ = fmt.Fprintf(w, "event: %s\n", ev.Event)
			}
			if ev.ID != "" {
				_, _ = fmt.Fprintf(w, "id: %s\n", ev.ID)
			}
			for _, line := range strings.Split(data, "\n") {
				_, _ = fmt.Fprintf(w, "data: %s\n", line)
			}
			_, _ = fmt.Fprint(w, "\n")
		} else {
			_, _ = fmt.Fprint(w, data)
		}
		if fl != nil {
			fl.Flush()
		}
	}
}

