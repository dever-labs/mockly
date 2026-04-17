// Package httpserver implements the HTTP mock server.
package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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
)

// Server is the HTTP mock server.
type Server struct {
	cfg       *config.HTTPConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger

	mu         sync.RWMutex
	mocks      []config.HTTPMock
	callCounts map[string]int64 // mock ID → total call count (for sequences + API)

	server *http.Server
}

// New creates a Server. The mocks slice is taken from cfg initially but can
// be replaced at runtime via SetMocks.
func New(cfg *config.HTTPConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	s := &Server{
		cfg:        cfg,
		store:      store,
		scenarios:  sc,
		log:        log,
		mocks:      append([]config.HTTPMock(nil), cfg.Mocks...),
		callCounts: make(map[string]int64),
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

	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close() //nolint:errcheck

	hdrs := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		hdrs[k] = strings.Join(v, ", ")
	}

	query := make(map[string]string, len(r.URL.Query()))
	for k, v := range r.URL.Query() {
		query[k] = v[0]
	}

	// Global fault: inject latency before processing.
	fault := s.scenarios.GetFault()
	if fault != nil && fault.Enabled && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	s.mu.RLock()
	mocks := s.mocks
	s.mu.RUnlock()

	result, matched := engine.HTTPMatch(mocks, r.Method, r.URL.Path, query, hdrs, string(body), s.store)

	status := http.StatusNotFound
	respBody := `{"error":"no mock matched"}`
	respHdrs := map[string]string{"Content-Type": "application/json"}
	matchedID := ""
	delay := time.Duration(0)

	if matched {
		status = result.Status
		respBody = result.Body
		respHdrs = result.Headers
		matchedID = result.MockID
		delay = result.Delay

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

		if matchedMock != nil && len(matchedMock.Sequence) > 0 {
			idx := int(callN) - 1 // 0-based
			seq := matchedMock.Sequence
			switch {
			case idx < len(seq):
				entry := seq[idx]
				if entry.Status != 0 {
					status = entry.Status
				}
				if entry.Body != "" {
					respBody = engine.Render(entry.Body, query, hdrs, string(body))
				}
				for k, v := range entry.Headers {
					respHdrs[k] = engine.Render(v, query, hdrs, string(body))
				}
				if entry.Delay.Duration > 0 {
					delay = entry.Delay.Duration
				}
			default:
				exhausted := matchedMock.SequenceExhausted
				if exhausted == "" {
					exhausted = "hold_last"
				}
				switch exhausted {
				case "loop":
					loopIdx := (idx) % len(seq)
					entry := seq[loopIdx]
					if entry.Status != 0 {
						status = entry.Status
					}
					if entry.Body != "" {
						respBody = engine.Render(entry.Body, query, hdrs, string(body))
					}
					for k, v := range entry.Headers {
						respHdrs[k] = engine.Render(v, query, hdrs, string(body))
					}
					if entry.Delay.Duration > 0 {
						delay = entry.Delay.Duration
					}
				case "not_found":
					status = http.StatusNotFound
					respBody = `{"error":"sequence exhausted"}`
				default: // hold_last
					entry := seq[len(seq)-1]
					if entry.Status != 0 {
						status = entry.Status
					}
					if entry.Body != "" {
						respBody = engine.Render(entry.Body, query, hdrs, string(body))
					}
					for k, v := range entry.Headers {
						respHdrs[k] = engine.Render(v, query, hdrs, string(body))
					}
					if entry.Delay.Duration > 0 {
						delay = entry.Delay.Duration
					}
				}
			}
		}

		// Apply the first active scenario patch for this mock (if any).
		if patch := s.scenarios.PatchFor(matchedID); patch != nil {
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
					respBody = patch.Body
				}
				for k, v := range patch.Headers {
					respHdrs[k] = v
				}
				if patch.Delay != nil {
					delay = patch.Delay.Duration
				}
			}
		}

		// Per-mock fault (applied after scenario patches).
		if matchedMock != nil && matchedMock.Fault != nil {
			mf := matchedMock.Fault
			if mf.Delay.Duration > 0 {
				delay += mf.Delay.Duration
			}
			if mf.StatusOverride != 0 && s.scenarios.RollFault(mf.ErrorRate) {
				status = mf.StatusOverride
				if mf.Body != "" {
					respBody = mf.Body
				}
			}
		}
	}

	// Global fault: probabilistically override status/body (chaos testing).
	if fault != nil && fault.Enabled && fault.StatusOverride != 0 && s.scenarios.RollFault(fault.ErrorRate) {
		status = fault.StatusOverride
		if fault.Body != "" {
			respBody = fault.Body
		}
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	for k, v := range respHdrs {
		w.Header().Set(k, v)
	}
	w.WriteHeader(status)
	_, _ = fmt.Fprint(w, respBody)

	s.log.Log(logger.Entry{
		Protocol:  "http",
		Method:    r.Method,
		Path:      r.URL.Path,
		Status:    status,
		Duration:  time.Since(start).Milliseconds(),
		Headers:   hdrs,
		Body:      string(body),
		MatchedID: matchedID,
	})
}

// StatusInfo returns JSON-serialisable info about this server.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"protocol": "http",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
	}
}

// MarshalMocks returns the mock list as JSON bytes.
func (s *Server) MarshalMocks() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.mocks)
}

