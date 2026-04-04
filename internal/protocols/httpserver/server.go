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
	mocks     []config.HTTPMock
	server    *http.Server
}

// New creates a Server. The mocks slice is taken from cfg initially but can
// be replaced at runtime via SetMocks.
func New(cfg *config.HTTPConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	s := &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		mocks:     append([]config.HTTPMock(nil), cfg.Mocks...),
	}
	return s
}

// SetMocks replaces the current mock list (called by the management API).
func (s *Server) SetMocks(mocks []config.HTTPMock) {
	s.mocks = append([]config.HTTPMock(nil), mocks...)
}

// GetMocks returns the current mock list.
func (s *Server) GetMocks() []config.HTTPMock {
	return append([]config.HTTPMock(nil), s.mocks...)
}

// Start begins listening. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.HandleFunc("/*", s.handleRequest)

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.server = &http.Server{Addr: addr, Handler: r}

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
	defer r.Body.Close()

	hdrs := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		hdrs[k] = strings.Join(v, ", ")
	}

	// Global fault: inject latency on every request before processing.
	fault := s.scenarios.GetFault()
	if fault != nil && fault.Enabled && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	result, matched := engine.HTTPMatch(s.mocks, r.Method, r.URL.Path, hdrs, string(body), s.store)

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
	}

	// Global fault: probabilistically override status/body (chaos testing).
	// This applies even to matched mocks, after scenario patches.
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
	fmt.Fprint(w, respBody)

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
	return map[string]interface{}{
		"protocol": "http",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
	}
}

// MarshalMocks returns the mock list as JSON bytes.
func (s *Server) MarshalMocks() ([]byte, error) {
	return json.Marshal(s.mocks)
}
