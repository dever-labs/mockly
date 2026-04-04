// Package logger provides a structured request log with an SSE broadcaster
// so the management UI can stream live events.
package logger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Entry is a single logged request/event.
type Entry struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Protocol  string            `json:"protocol"`
	Method    string            `json:"method,omitempty"`
	Path      string            `json:"path"`
	Status    int               `json:"status,omitempty"`
	Duration  int64             `json:"duration_ms"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	MatchedID string            `json:"matched_id,omitempty"`
}

// Logger stores recent entries and broadcasts to SSE subscribers.
type Logger struct {
	mu          sync.RWMutex
	entries     []Entry
	maxEntries  int
	subscribers map[string]chan Entry
}

// New creates a Logger that keeps at most maxEntries recent log entries.
func New(maxEntries int) *Logger {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	return &Logger{
		maxEntries:  maxEntries,
		subscribers: make(map[string]chan Entry),
	}
}

// Log appends an entry and broadcasts it.
func (l *Logger) Log(e Entry) {
	if e.ID == "" {
		e.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	l.mu.Lock()
	l.entries = append(l.entries, e)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
	subs := make([]chan Entry, 0, len(l.subscribers))
	for _, ch := range l.subscribers {
		subs = append(subs, ch)
	}
	l.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Entries returns all stored entries (most recent last).
func (l *Logger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Clear removes all stored entries.
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

// Subscribe returns a channel that receives new log entries and a cancel func.
func (l *Logger) Subscribe(id string) (<-chan Entry, func()) {
	ch := make(chan Entry, 64)
	l.mu.Lock()
	l.subscribers[id] = ch
	l.mu.Unlock()
	cancel := func() {
		l.mu.Lock()
		delete(l.subscribers, id)
		l.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

// ServeSSE handles an HTTP SSE connection, streaming new log entries.
func (l *Logger) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	subID := fmt.Sprintf("%d", time.Now().UnixNano())
	ch, cancel := l.Subscribe(subID)
	defer cancel()

	for {
		select {
		case entry, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
