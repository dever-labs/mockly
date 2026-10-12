// Package streamregistry tracks currently-open, push-capable stream
// connections (HTTP SSE/chunked responses, gRPC server/bidirectional
// streams, GraphQL subscriptions) so the management API can push ad-hoc
// events into them on demand, in addition to whatever their mock's
// configured stream list already sends, and end them early.
package streamregistry

import (
	"fmt"
	"sync"
	"time"
)

// Info is the JSON-serialisable, read-only view of a connection exposed to
// API callers via List.
type Info struct {
	ID        string    `json:"id"`
	Protocol  string    `json:"protocol"`
	MockID    string    `json:"mock_id"`
	StartedAt time.Time `json:"started_at"`
}

// Connection is how a protocol server registers one open stream with the
// registry. Push and Close are called from the registry's own
// goroutine (e.g. an API request handler), concurrently with the
// connection's own scheduled sends, so both must be safe for that.
type Connection struct {
	Info

	// Push delivers data as one more event over this connection, on top of
	// (not instead of) whatever the mock's configured stream list already
	// sends. It should not block for long; a typical implementation
	// enqueues onto a small buffered channel consumed by the connection's
	// own send loop, returning an error if the buffer is full rather than
	// blocking the caller indefinitely.
	Push func(data string) error

	// Close ends the connection/subscription/RPC. If errMsg is empty the
	// connection ends normally (e.g. GraphQL "complete", gRPC OK status,
	// HTTP stream simply closes); otherwise it ends as an error, with
	// errMsg as the message, in whatever way is idiomatic for the
	// protocol (e.g. a GraphQL "error" message, a gRPC status error).
	Close func(errMsg string)
}

// Registry is safe for concurrent use.
type Registry struct {
	mu    sync.RWMutex
	conns map[string]*Connection
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{conns: map[string]*Connection{}}
}

// Register adds c to the registry and returns a function that removes it;
// callers must call the returned function when the connection ends
// (typically via defer), whether that's a natural close, a client
// disconnect, or a Close call routed through this registry.
func (r *Registry) Register(c *Connection) (unregister func()) {
	r.mu.Lock()
	r.conns[c.ID] = c
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.conns, c.ID)
		r.mu.Unlock()
	}
}

// List returns info for every currently registered connection.
func (r *Registry) List() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, c.Info)
	}
	return out
}

func (r *Registry) find(id string) (*Connection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.conns[id]
	return c, ok
}

func (r *Registry) byMock(mockID string) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Connection, 0)
	for _, c := range r.conns {
		if c.MockID == mockID {
			out = append(out, c)
		}
	}
	return out
}

// Push sends data to exactly the connection identified by connID (if set),
// or to every currently active connection serving mockID otherwise,
// returning how many connections it reached. Exactly one of connID/mockID
// should be non-empty; connID takes precedence if both are.
func (r *Registry) Push(connID, mockID, data string) (int, error) {
	if connID != "" {
		c, ok := r.find(connID)
		if !ok {
			return 0, fmt.Errorf("no active stream connection with id %q", connID)
		}
		if err := c.Push(data); err != nil {
			return 0, err
		}
		return 1, nil
	}
	targets := r.byMock(mockID)
	if len(targets) == 0 {
		return 0, fmt.Errorf("no active stream connections for mock %q", mockID)
	}
	sent := 0
	var firstErr error
	for _, c := range targets {
		if err := c.Push(data); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sent++
	}
	return sent, firstErr
}

// CloseConn ends connection(s) targeted the same way as Push, optionally as
// an error (errMsg non-empty), returning how many connections it reached.
func (r *Registry) CloseConn(connID, mockID, errMsg string) (int, error) {
	if connID != "" {
		c, ok := r.find(connID)
		if !ok {
			return 0, fmt.Errorf("no active stream connection with id %q", connID)
		}
		c.Close(errMsg)
		return 1, nil
	}
	targets := r.byMock(mockID)
	if len(targets) == 0 {
		return 0, fmt.Errorf("no active stream connections for mock %q", mockID)
	}
	for _, c := range targets {
		c.Close(errMsg)
	}
	return len(targets), nil
}
