// Package webhook implements outbound HTTP callback delivery for mocks that
// need to simulate server-initiated notifications rather than only
// responding synchronously — payment gateway webhooks, message broker push
// callbacks, job/CI completion notifications, and similar patterns.
//
// The package is protocol-agnostic: any protocol server (not just HTTP) can
// call Sender.Dispatch to fire a templated outbound HTTP request and have
// the attempt recorded for later inspection via the management API.
package webhook

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/xid"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
)

// defaultHistorySize is used when Sender is created with a non-positive size.
const defaultHistorySize = 500

// defaultTimeout bounds how long a single webhook HTTP call may take.
const defaultTimeout = 30 * time.Second

// defaultRetryDelay is used when a Webhook specifies Retries but no RetryDelay.
const defaultRetryDelay = time.Second

// maxCapturedResponseBody bounds how much of a callback's response body is
// stored in a Record, to keep the in-memory history bounded.
const maxCapturedResponseBody = 64 * 1024

// Record captures the outcome of a single outbound webhook attempt.
type Record struct {
	ID           string    `json:"id"`
	MockID       string    `json:"mock_id,omitempty"`
	Protocol     string    `json:"protocol,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	URL          string    `json:"url"`
	Method       string    `json:"method"`
	RequestBody  string    `json:"request_body,omitempty"`
	StatusCode   int       `json:"status_code,omitempty"`
	ResponseBody string    `json:"response_body,omitempty"`
	Error        string    `json:"error,omitempty"`
	Attempt      int       `json:"attempt"`
	DurationMS   int64     `json:"duration_ms"`
}

// Store is a bounded, thread-safe ring buffer of recent webhook Records.
// When full, the oldest entry is overwritten (O(1) add, no allocation).
type Store struct {
	mu      sync.RWMutex
	buf     []Record
	head    int
	count   int
	maxSize int
}

// NewStore creates a Store with the given capacity (defaults to
// defaultHistorySize when maxSize <= 0).
func NewStore(maxSize int) *Store {
	if maxSize <= 0 {
		maxSize = defaultHistorySize
	}
	return &Store{buf: make([]Record, maxSize), maxSize: maxSize}
}

func (s *Store) add(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tail := (s.head + s.count) % s.maxSize
	s.buf[tail] = r
	if s.count == s.maxSize {
		s.head = (s.head + 1) % s.maxSize
	} else {
		s.count++
	}
}

// All returns all recorded webhook attempts, oldest first.
func (s *Store) All() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, s.count)
	for i := 0; i < s.count; i++ {
		out[i] = s.buf[(s.head+i)%s.maxSize]
	}
	return out
}

// Clear removes all recorded webhook attempts.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.head = 0
	s.count = 0
}

// Sender dispatches outbound webhook calls and records their outcome.
type Sender struct {
	store  *Store
	client *http.Client
}

// New creates a Sender whose attempt history holds up to historySize
// records (defaults applied when historySize <= 0).
func New(historySize int) *Sender {
	return &Sender{
		store:  NewStore(historySize),
		client: &http.Client{Timeout: defaultTimeout},
	}
}

// History returns the Store backing this Sender's webhook attempt history.
func (s *Sender) History() *Store { return s.store }

// Dispatch renders wh's URL/headers/body against reqCtx and sends the
// resulting HTTP request asynchronously in a background goroutine — it
// returns immediately without blocking the caller's response. wh.Delay and
// wh.Retries are honoured; every attempt is recorded in s.History().
//
// protocol and mockID are metadata attached to the resulting Record(s) so
// the management API can show which mock/protocol triggered the callback.
func (s *Sender) Dispatch(protocol, mockID string, wh config.Webhook, reqCtx engine.RequestContext) {
	go s.dispatch(protocol, mockID, wh, reqCtx)
}

// SendAdHoc renders and sends a one-off webhook synchronously, with no mock
// association. Used by the management API's manual "send now" endpoint —
// handy for simulating a callback that isn't tied to any specific mock
// (e.g. a gateway's out-of-band "payment completed" notification).
func (s *Sender) SendAdHoc(wh config.Webhook) Record {
	return s.dispatch("manual", "", wh, engine.RequestContext{})
}

func (s *Sender) dispatch(protocol, mockID string, wh config.Webhook, reqCtx engine.RequestContext) Record {
	url, err := engine.RenderErr(wh.URL, reqCtx)
	if err != nil {
		// The URL template failed to execute — commonly because it indexes
		// an optional, caller-supplied field (e.g. a "webhooks" array) that
		// wasn't present on this particular request. Attempting the call
		// anyway would just send a broken literal-template string as the
		// URL; skip the network call (and the configured delay) entirely
		// and record why.
		rec := Record{
			ID:        xid.New().String(),
			MockID:    mockID,
			Protocol:  protocol,
			Timestamp: time.Now().UTC(),
			URL:       wh.URL,
			Method:    wh.Method,
			Error:     "skipped: webhook URL template failed to render: " + err.Error(),
			Attempt:   1,
		}
		s.store.add(rec)
		return rec
	}

	if wh.Delay.Duration > 0 {
		time.Sleep(wh.Delay.Duration)
	}
	method := wh.Method
	if method == "" {
		method = http.MethodPost
	}
	body := engine.Render(wh.Body, reqCtx)
	headers := make(map[string]string, len(wh.Headers))
	for k, v := range wh.Headers {
		headers[k] = engine.Render(v, reqCtx)
	}

	attempts := wh.Retries + 1
	retryDelay := wh.RetryDelay.Duration
	if retryDelay <= 0 {
		retryDelay = defaultRetryDelay
	}

	var rec Record
	for attempt := 1; attempt <= attempts; attempt++ {
		rec = s.attempt(protocol, mockID, url, method, headers, body, attempt)
		s.store.add(rec)
		if rec.Error == "" && rec.StatusCode < 500 {
			break
		}
		if attempt < attempts {
			time.Sleep(retryDelay)
		}
	}
	return rec
}

func (s *Sender) attempt(protocol, mockID, url, method string, headers map[string]string, body string, attempt int) Record {
	start := time.Now()
	rec := Record{
		ID:          xid.New().String(),
		MockID:      mockID,
		Protocol:    protocol,
		Timestamp:   time.Now().UTC(),
		URL:         url,
		Method:      method,
		RequestBody: body,
		Attempt:     attempt,
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBufferString(body))
	if err != nil {
		rec.Error = err.Error()
		rec.DurationMS = time.Since(start).Milliseconds()
		return rec
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	rec.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		rec.Error = err.Error()
		return rec
	}
	defer resp.Body.Close() //nolint:errcheck
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxCapturedResponseBody))
	rec.StatusCode = resp.StatusCode
	rec.ResponseBody = string(respBody)
	return rec
}
