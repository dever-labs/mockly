// Package wsserver implements the WebSocket mock server.
package wsserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/tlsutil"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server is the WebSocket mock server.
type Server struct {
	cfg   *config.WebSocketConfig
	store *state.Store
	log   *logger.Logger

	mu    sync.RWMutex
	mocks []config.WebSocketMock
	srv   *http.Server
}

// New creates a Server.
func New(cfg *config.WebSocketConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:   cfg,
		store: store,
		log:   log,
		mocks: append([]config.WebSocketMock(nil), cfg.Mocks...),
	}
}

// SetMocks replaces the current mock list.
func (s *Server) SetMocks(mocks []config.WebSocketMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.WebSocketMock(nil), mocks...)
}

// GetMocks returns the current mock list.
func (s *Server) GetMocks() []config.WebSocketMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.WebSocketMock(nil), s.mocks...)
}

// Start begins listening. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := chi.NewRouter()
	for _, m := range s.mocks {
		path := m.Path
		mock := m
		r.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			s.handleConn(w, r, &mock)
		})
	}
	// Catch-all for dynamically added mocks
	r.HandleFunc("/*", s.handleDynamic)

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.srv = &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ws server listen %s: %w", addr, err)
	}
	ln, err = tlsutil.WrapListener(ln, s.cfg.TLS)
	if err != nil {
		return fmt.Errorf("ws server tls: %w", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleDynamic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	s.mu.RLock()
	var found *config.WebSocketMock
	for i := range s.mocks {
		if s.mocks[i].Path == path {
			cp := s.mocks[i]
			found = &cp
			break
		}
	}
	s.mu.RUnlock()
	if found != nil {
		s.handleConn(w, r, found)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleConn(w http.ResponseWriter, r *http.Request, mock *config.WebSocketMock) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close() //nolint:errcheck
	start := time.Now()

	if mock.OnConnect != nil {
		if mock.OnConnect.Delay.Duration > 0 {
			time.Sleep(mock.OnConnect.Delay.Duration)
		}
		if mock.OnConnect.Send != "" {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(mock.OnConnect.Send))
		}
	}

	s.log.Log(logger.Entry{
		Protocol: "websocket",
		Path:     r.URL.Path,
		Duration: time.Since(start).Milliseconds(),
		MatchedID: mock.ID,
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		text := string(msg)
		rule, ok := engine.WSMatch(mock.OnMessage, text)
		if !ok {
			continue
		}

		if rule.Delay.Duration > 0 {
			time.Sleep(rule.Delay.Duration)
		}

		if rule.Close {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			break
		}

		if rule.Respond != "" {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(rule.Respond))
		}

		s.log.Log(logger.Entry{
			Protocol:  "websocket",
			Path:      r.URL.Path,
			Method:    "MESSAGE",
			Body:      text,
			Duration:  time.Since(start).Milliseconds(),
			MatchedID: mock.ID,
		})
	}
}

// StatusInfo returns JSON-serialisable info about this server.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	n := len(s.mocks)
	s.mu.RUnlock()
	tlsEnabled := s.cfg.TLS != nil && s.cfg.TLS.Enabled
	return map[string]interface{}{
		"protocol": "websocket",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"tls":      tlsEnabled,
		"mocks":    n,
	}
}
