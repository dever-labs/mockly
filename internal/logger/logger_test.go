package logger_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/logger"
)

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew_DefaultMaxEntries(t *testing.T) {
	l := logger.New(0) // 0 → defaults to 500
	if l == nil {
		t.Fatal("New(0) returned nil")
	}
}

func TestNew_NegativeMaxEntries(t *testing.T) {
	l := logger.New(-1)
	if l == nil {
		t.Fatal("New(-1) returned nil")
	}
}

func TestNew_PositiveMaxEntries(t *testing.T) {
	l := logger.New(10)
	if l == nil {
		t.Fatal("New(10) returned nil")
	}
}

// ---------------------------------------------------------------------------
// Log / Entries
// ---------------------------------------------------------------------------

func TestLog_AppendsEntry(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Protocol: "http", Path: "/foo", Status: 200})
	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/foo" {
		t.Errorf("unexpected path %q", entries[0].Path)
	}
}

func TestLog_AutoID(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Protocol: "http", Path: "/no-id"})
	e := l.Entries()[0]
	if e.ID == "" {
		t.Error("expected auto-generated ID, got empty string")
	}
}

func TestLog_PreservesExplicitID(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{ID: "my-id", Path: "/x"})
	if l.Entries()[0].ID != "my-id" {
		t.Error("explicit ID should be preserved")
	}
}

func TestLog_AutoTimestamp(t *testing.T) {
	l := logger.New(100)
	before := time.Now()
	l.Log(logger.Entry{Path: "/ts"})
	after := time.Now()

	e := l.Entries()[0]
	if e.Timestamp.Before(before) || e.Timestamp.After(after) {
		t.Errorf("auto timestamp %v out of expected range [%v, %v]", e.Timestamp, before, after)
	}
}

func TestLog_TrimsWhenExceedingMax(t *testing.T) {
	l := logger.New(3)
	for i := range 5 {
		l.Log(logger.Entry{Path: "/x", Status: i})
	}
	entries := l.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (max), got %d", len(entries))
	}
	// Should have kept the last 3 entries (status 2, 3, 4).
	if entries[0].Status != 2 {
		t.Errorf("expected first kept entry status=2, got %d", entries[0].Status)
	}
	if entries[2].Status != 4 {
		t.Errorf("expected last kept entry status=4, got %d", entries[2].Status)
	}
}

func TestEntries_ReturnsSnapshot(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/snap"})

	snap := l.Entries()
	snap[0].Path = "mutated"

	fresh := l.Entries()
	if fresh[0].Path != "/snap" {
		t.Error("Entries() should return a copy, not a live reference")
	}
}

func TestEntries_Empty(t *testing.T) {
	l := logger.New(100)
	if got := l.Entries(); len(got) != 0 {
		t.Fatalf("expected empty, got %d entries", len(got))
	}
}

// ---------------------------------------------------------------------------
// Clear
// ---------------------------------------------------------------------------

func TestClear(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/a"})
	l.Log(logger.Entry{Path: "/b"})
	l.Clear()
	if entries := l.Entries(); len(entries) != 0 {
		t.Fatalf("expected empty after Clear, got %d", len(entries))
	}
}

func TestClear_ThenLog(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/before"})
	l.Clear()
	l.Log(logger.Entry{Path: "/after"})
	entries := l.Entries()
	if len(entries) != 1 || entries[0].Path != "/after" {
		t.Fatalf("unexpected entries after Clear+Log: %v", entries)
	}
}

// ---------------------------------------------------------------------------
// ClearByMockID
// ---------------------------------------------------------------------------

func TestClearByMockID_RemovesMatchingOnly(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/a", MatchedID: "m1"})
	l.Log(logger.Entry{Path: "/b", MatchedID: "m2"})
	l.Log(logger.Entry{Path: "/c", MatchedID: "m1"})

	l.ClearByMockID("m1")

	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry remaining, got %d", len(entries))
	}
	if entries[0].MatchedID != "m2" {
		t.Errorf("expected m2 to remain, got %q", entries[0].MatchedID)
	}
}

func TestClearByMockID_UnknownID(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/a", MatchedID: "m1"})
	l.ClearByMockID("nonexistent")
	if len(l.Entries()) != 1 {
		t.Fatal("ClearByMockID with unknown ID should not remove anything")
	}
}

// ---------------------------------------------------------------------------
// EntriesByMockID
// ---------------------------------------------------------------------------

func TestEntriesByMockID(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/a", MatchedID: "m1"})
	l.Log(logger.Entry{Path: "/b", MatchedID: "m2"})
	l.Log(logger.Entry{Path: "/c", MatchedID: "m1"})

	got := l.EntriesByMockID("m1")
	if len(got) != 2 {
		t.Fatalf("expected 2 entries for m1, got %d", len(got))
	}
	for _, e := range got {
		if e.MatchedID != "m1" {
			t.Errorf("unexpected MatchedID %q", e.MatchedID)
		}
	}
}

func TestEntriesByMockID_Empty(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/x", MatchedID: "m1"})
	if got := l.EntriesByMockID("m2"); len(got) != 0 {
		t.Fatalf("expected no entries for m2, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// Subscribe / cancel
// ---------------------------------------------------------------------------

func TestSubscribe_ReceivesEntry(t *testing.T) {
	l := logger.New(100)
	ch, cancel := l.Subscribe("sub-1")
	defer cancel()

	l.Log(logger.Entry{Path: "/broadcast"})

	select {
	case e := <-ch:
		if e.Path != "/broadcast" {
			t.Errorf("unexpected path %q", e.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast entry")
	}
}

func TestSubscribe_CancelClosesChannel(t *testing.T) {
	l := logger.New(100)
	ch, cancel := l.Subscribe("sub-cancel")
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected closed channel after cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel was not closed after cancel")
	}
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	l := logger.New(100)
	ch1, cancel1 := l.Subscribe("s1")
	ch2, cancel2 := l.Subscribe("s2")
	defer cancel1()
	defer cancel2()

	l.Log(logger.Entry{Path: "/multi"})

	for _, ch := range []<-chan logger.Entry{ch1, ch2} {
		select {
		case e := <-ch:
			if e.Path != "/multi" {
				t.Errorf("unexpected path %q", e.Path)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for subscriber")
		}
	}
}

// ---------------------------------------------------------------------------
// WaitFor
// ---------------------------------------------------------------------------

func TestWaitFor_FastPath(t *testing.T) {
	l := logger.New(100)
	l.Log(logger.Entry{Path: "/x", MatchedID: "m1"})
	l.Log(logger.Entry{Path: "/y", MatchedID: "m1"})

	ctx := context.Background()
	entries, err := l.WaitFor(ctx, "m1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected >=2 entries, got %d", len(entries))
	}
}

func TestWaitFor_WaitsForEntry(t *testing.T) {
	l := logger.New(100)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		l.Log(logger.Entry{Path: "/delayed", MatchedID: "m-wait"})
	}()

	entries, err := l.WaitFor(ctx, "m-wait", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) < 1 {
		t.Fatal("expected at least 1 entry")
	}
	wg.Wait()
}

func TestWaitFor_ContextTimeout(t *testing.T) {
	l := logger.New(100)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := l.WaitFor(ctx, "never-logged", 1)
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
}

// ---------------------------------------------------------------------------
// ServeSSE
// ---------------------------------------------------------------------------

func TestServeSSE_StreamsEntries(t *testing.T) {
	l := logger.New(100)

	// Start SSE response recorder via httptest.
	req := httptest.NewRequest("GET", "/logs/stream", nil)
	reqCtx, cancelReq := context.WithCancel(req.Context())
	req = req.WithContext(reqCtx)

	// Use a pipe to read SSE data without blocking.
	pr, pw := syncPipe()
	rw := &sseResponseWriter{header: http.Header{}, body: pw}

	done := make(chan struct{})
	go func() {
		defer close(done)
		l.ServeSSE(rw, req)
	}()

	// Give the SSE handler time to subscribe, then log an entry.
	time.Sleep(20 * time.Millisecond)
	l.Log(logger.Entry{Path: "/sse-test", MatchedID: "m1"})

	// Read until we see the expected data line.
	line := pr.ReadLine(t, time.Second)
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("expected SSE data line, got %q", line)
	}
	var entry logger.Entry
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &entry); err != nil {
		t.Fatalf("unmarshal SSE entry: %v", err)
	}
	if entry.Path != "/sse-test" {
		t.Errorf("unexpected path %q in SSE entry", entry.Path)
	}

	cancelReq()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ServeSSE did not exit after context cancel")
	}
}

func TestServeSSE_NonFlusher(t *testing.T) {
	l := logger.New(100)
	req := httptest.NewRequest("GET", "/logs/stream", nil)
	rw := &nonFlusherWriter{header: http.Header{}}
	l.ServeSSE(rw, req)
	if rw.status != http.StatusInternalServerError {
		t.Errorf("expected 500 for non-flusher, got %d", rw.status)
	}
}

// nonFlusherWriter is a minimal http.ResponseWriter that does NOT implement http.Flusher.
type nonFlusherWriter struct {
	header http.Header
	status int
	body   strings.Builder
}

func (w *nonFlusherWriter) Header() http.Header         { return w.header }
func (w *nonFlusherWriter) WriteHeader(code int)        { w.status = code }
func (w *nonFlusherWriter) Write(b []byte) (int, error) { return w.body.Write(b) }

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestLog_ConcurrentSafe(t *testing.T) {
	l := logger.New(200)
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.Log(logger.Entry{Path: "/concurrent"})
			l.Entries()
			if n%5 == 0 {
				l.Clear()
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sseResponseWriter is a minimal http.ResponseWriter + http.Flusher backed by a pipe writer.
type sseResponseWriter struct {
	header http.Header
	body   *pipeWriter
	status int
}

func (r *sseResponseWriter) Header() http.Header        { return r.header }
func (r *sseResponseWriter) WriteHeader(code int)       { r.status = code }
func (r *sseResponseWriter) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *sseResponseWriter) Flush()                      {}

// pipeWriter wraps a channel to collect written bytes line by line.
type pipeWriter struct {
	mu   sync.Mutex
	data []byte
	ch   chan struct{}
}

type pipeReader struct {
	pw *pipeWriter
}

func syncPipe() (*pipeReader, *pipeWriter) {
	pw := &pipeWriter{ch: make(chan struct{}, 64)}
	return &pipeReader{pw: pw}, pw
}

func (pw *pipeWriter) Write(b []byte) (int, error) {
	pw.mu.Lock()
	pw.data = append(pw.data, b...)
	pw.mu.Unlock()
	select {
	case pw.ch <- struct{}{}:
	default:
	}
	return len(b), nil
}

// ReadLine reads buffered data and returns the first complete "data: ..." line found,
// waiting up to timeout.
func (pr *pipeReader) ReadLine(t *testing.T, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pr.pw.mu.Lock()
		raw := string(pr.pw.data)
		pr.pw.mu.Unlock()
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.HasPrefix(line, "data: ") {
				return line
			}
		}
		select {
		case <-pr.pw.ch:
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatal("ReadLine: timed out waiting for data line")
	return ""
}
