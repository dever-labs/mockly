package webhook

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
)

func TestSenderDispatchSuccess(t *testing.T) {
	received := make(chan struct{}, 1)
	var gotBody, gotMethod, gotHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Event")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer ts.Close()

	s := New(10)
	wh := config.Webhook{
		URL:     ts.URL + "/cb",
		Method:  "POST",
		Headers: map[string]string{"X-Event": "{{.request.body.event}}"},
		Body:    `{"id":"{{.request.body.id}}"}`,
	}
	reqCtx := engine.RequestContext{Body: `{"event":"created","id":"pay_123"}`}

	s.Dispatch("http", "mock-1", wh, reqCtx)

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("webhook was not received in time")
	}
	// Allow the goroutine to finish recording after responding.
	time.Sleep(50 * time.Millisecond)

	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotHeader != "created" {
		t.Errorf("X-Event header = %q, want created", gotHeader)
	}
	if gotBody != `{"id":"pay_123"}` {
		t.Errorf("body = %q, want {\"id\":\"pay_123\"}", gotBody)
	}

	records := s.History().All()
	if len(records) != 1 {
		t.Fatalf("history len = %d, want 1", len(records))
	}
	rec := records[0]
	if rec.StatusCode != http.StatusOK {
		t.Errorf("recorded status = %d, want 200", rec.StatusCode)
	}
	if rec.MockID != "mock-1" || rec.Protocol != "http" {
		t.Errorf("recorded mockID/protocol = %q/%q, want mock-1/http", rec.MockID, rec.Protocol)
	}
}

func TestSenderDispatchRetriesOn5xx(t *testing.T) {
	var attempts int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := New(10)
	wh := config.Webhook{
		URL:        ts.URL,
		Retries:    2,
		RetryDelay: config.Duration{Duration: 10 * time.Millisecond},
	}
	rec := s.dispatch("http", "mock-2", wh, engine.RequestContext{})

	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if rec.StatusCode != http.StatusOK {
		t.Errorf("final status = %d, want 200", rec.StatusCode)
	}
	if len(s.History().All()) != 3 {
		t.Errorf("history len = %d, want 3 (one per attempt)", len(s.History().All()))
	}
}

func TestSenderSendAdHoc(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	s := New(10)
	rec := s.SendAdHoc(config.Webhook{URL: ts.URL, Method: "PUT"})
	if rec.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.StatusCode)
	}
	if rec.Protocol != "manual" {
		t.Errorf("protocol = %q, want manual", rec.Protocol)
	}
	if rec.ResponseBody != "ok" {
		t.Errorf("response body = %q, want ok", rec.ResponseBody)
	}
}

func TestSenderDispatchNetworkError(t *testing.T) {
	s := New(10)
	rec := s.dispatch("http", "mock-3", config.Webhook{URL: "http://127.0.0.1:1"}, engine.RequestContext{})
	if rec.Error == "" {
		t.Error("expected an error to be recorded for an unreachable URL")
	}
}

func TestSenderDispatchSkipsOnURLRenderFailure(t *testing.T) {
	s := New(10)
	wh := config.Webhook{
		URL:        "{{ (index .request.body.webhooks 0).url }}",
		Method:     "POST",
		Delay:      config.Duration{Duration: time.Hour},
		Retries:    2,
		RetryDelay: config.Duration{Duration: time.Hour},
	}
	// Request body has no "webhooks" field, so the template fails to render.
	reqCtx := engine.RequestContext{Body: `{"order":{"amount":1000}}`}

	start := time.Now()
	rec := s.dispatch("http", "mock-skip", wh, reqCtx)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("dispatch took %s, want it to skip immediately without waiting for the configured delay", elapsed)
	}
	if rec.Error == "" || !strings.Contains(rec.Error, "skipped") {
		t.Errorf("Error = %q, want it to explain the webhook was skipped", rec.Error)
	}
	if rec.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 (no HTTP attempt should have been made)", rec.StatusCode)
	}
}

func TestStoreRingBufferWraps(t *testing.T) {
	st := NewStore(3)
	for i := 0; i < 5; i++ {
		st.add(Record{ID: string(rune('a' + i))})
	}
	all := st.All()
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	if all[0].ID != "c" || all[2].ID != "e" {
		t.Errorf("unexpected ring buffer contents: %+v", all)
	}
}

func TestStoreClear(t *testing.T) {
	st := NewStore(3)
	st.add(Record{ID: "a"})
	st.Clear()
	if len(st.All()) != 0 {
		t.Error("expected empty history after Clear")
	}
}
