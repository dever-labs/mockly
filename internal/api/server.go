// Package api implements the Mockly management REST API.
package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"context"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ProtocolServer is implemented by each protocol server so the API can read
// and write their mock lists.
type ProtocolServer interface {
	StatusInfo() map[string]interface{}
}

// HTTPProtocol is the subset of httpserver.Server used by the API.
type HTTPProtocol interface {
	ProtocolServer
	GetMocks() []config.HTTPMock
	SetMocks([]config.HTTPMock)
}

// WSProtocol is the subset of wsserver.Server used by the API.
type WSProtocol interface {
	ProtocolServer
	GetMocks() []config.WebSocketMock
	SetMocks([]config.WebSocketMock)
}

// GRPCProtocol is the subset of grpcserver.Server used by the API.
type GRPCProtocol interface {
	ProtocolServer
	GetMocks() []config.GRPCMock
	SetMocks([]config.GRPCMock)
}

// Server is the management API HTTP server.
type Server struct {
	cfg       *config.MocklyConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger
	http      HTTPProtocol
	ws        WSProtocol
	grpc      GRPCProtocol
	server    *http.Server
	uiFiles   http.FileSystem
}

// New creates a management API Server.
func New(
	cfg *config.MocklyConfig,
	store *state.Store,
	sc *scenarios.Store,
	log *logger.Logger,
	httpSrv HTTPProtocol,
	wsSrv WSProtocol,
	grpcSrv GRPCProtocol,
) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		http:      httpSrv,
		ws:        wsSrv,
		grpc:      grpcSrv,
	}
}

// Start begins listening. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := s.buildRouter()
	addr := fmt.Sprintf(":%d", s.cfg.API.Port)
	s.server = &http.Server{Addr: addr, Handler: r}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("api server listen %s: %w", addr, err)
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

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}))

	r.Group(func(r chi.Router) {
		r.Use(middleware.SetHeader("Content-Type", "application/json"))

		r.Get("/api/health", s.health)
		r.Get("/api/protocols", s.listProtocols)

		r.Get("/api/mocks/http", s.listHTTPMocks)
		r.Post("/api/mocks/http", s.addHTTPMock)
		r.Put("/api/mocks/http/{id}", s.updateHTTPMock)
		r.Patch("/api/mocks/http/{id}", s.patchHTTPMock)
		r.Delete("/api/mocks/http/{id}", s.deleteHTTPMock)

		r.Get("/api/mocks/websocket", s.listWSMocks)
		r.Post("/api/mocks/websocket", s.addWSMock)
		r.Put("/api/mocks/websocket/{id}", s.updateWSMock)
		r.Patch("/api/mocks/websocket/{id}", s.patchWSMock)
		r.Delete("/api/mocks/websocket/{id}", s.deleteWSMock)

		r.Get("/api/mocks/grpc", s.listGRPCMocks)
		r.Post("/api/mocks/grpc", s.addGRPCMock)
		r.Put("/api/mocks/grpc/{id}", s.updateGRPCMock)
		r.Patch("/api/mocks/grpc/{id}", s.patchGRPCMock)
		r.Delete("/api/mocks/grpc/{id}", s.deleteGRPCMock)

		r.Get("/api/state", s.getState)
		r.Post("/api/state", s.setState)
		r.Delete("/api/state/{key}", s.deleteState)

		// Scenarios
		r.Get("/api/scenarios", s.listScenarios)
		r.Post("/api/scenarios", s.createScenario)
		r.Get("/api/scenarios/active", s.listActiveScenarios)
		r.Get("/api/scenarios/{id}", s.getScenario)
		r.Put("/api/scenarios/{id}", s.updateScenario)
		r.Delete("/api/scenarios/{id}", s.deleteScenario)
		r.Post("/api/scenarios/{id}/activate", s.activateScenario)
		r.Delete("/api/scenarios/{id}/activate", s.deactivateScenario)

		// Global fault injection
		r.Get("/api/fault", s.getFault)
		r.Post("/api/fault", s.setFault)
		r.Delete("/api/fault", s.clearFault)

		r.Get("/api/logs", s.getLogs)
		r.Delete("/api/logs", s.clearLogs)
		r.Get("/api/logs/stream", s.streamLogs)

		r.Post("/api/reset", s.reset)
	})

	// Serve embedded UI with SPA fallback for all non-/api routes.
	if s.uiFiles != nil {
		r.Handle("/*", spaHandler(s.uiFiles))
	}

	return r
}

// AttachUI registers an embedded file system to be served as the single-page app.
func (s *Server) AttachUI(files http.FileSystem) {
	s.uiFiles = files
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Protocols
// ---------------------------------------------------------------------------

func (s *Server) listProtocols(w http.ResponseWriter, r *http.Request) {
	var protocols []map[string]interface{}
	if s.http != nil {
		protocols = append(protocols, s.http.StatusInfo())
	}
	if s.ws != nil {
		protocols = append(protocols, s.ws.StatusInfo())
	}
	if s.grpc != nil {
		protocols = append(protocols, s.grpc.StatusInfo())
	}
	writeJSON(w, http.StatusOK, protocols)
}

// ---------------------------------------------------------------------------
// HTTP mocks
// ---------------------------------------------------------------------------

func (s *Server) listHTTPMocks(w http.ResponseWriter, r *http.Request) {
	if s.http == nil {
		writeJSON(w, http.StatusOK, []config.HTTPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.http.GetMocks())
}

func (s *Server) addHTTPMock(w http.ResponseWriter, r *http.Request) {
	var m config.HTTPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("http-%d", time.Now().UnixNano())
	}
	mocks := s.http.GetMocks()
	mocks = append(mocks, m)
	s.http.SetMocks(mocks)
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateHTTPMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.HTTPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.http.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.http.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteHTTPMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mocks := s.http.GetMocks()
	filtered := mocks[:0]
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.http.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// httpResponsePatch carries only the response fields that should change.
type httpResponsePatch struct {
	Status  *int              `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    *string           `json:"body,omitempty"`
	Delay   *config.Duration  `json:"delay,omitempty"`
}

func (s *Server) patchHTTPMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var patch httpResponsePatch
	if err := decodeBody(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mocks := s.http.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			if patch.Status != nil {
				mocks[i].Response.Status = *patch.Status
			}
			if patch.Body != nil {
				mocks[i].Response.Body = *patch.Body
			}
			if patch.Headers != nil {
				if mocks[i].Response.Headers == nil {
					mocks[i].Response.Headers = make(map[string]string)
				}
				for k, v := range patch.Headers {
					mocks[i].Response.Headers[k] = v
				}
			}
			if patch.Delay != nil {
				mocks[i].Response.Delay = *patch.Delay
			}
			s.http.SetMocks(mocks)
			writeJSON(w, http.StatusOK, mocks[i])
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

// ---------------------------------------------------------------------------
// WebSocket mocks
// ---------------------------------------------------------------------------

func (s *Server) listWSMocks(w http.ResponseWriter, r *http.Request) {
	if s.ws == nil {
		writeJSON(w, http.StatusOK, []config.WebSocketMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.ws.GetMocks())
}

func (s *Server) addWSMock(w http.ResponseWriter, r *http.Request) {
	var m config.WebSocketMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("ws-%d", time.Now().UnixNano())
	}
	mocks := s.ws.GetMocks()
	mocks = append(mocks, m)
	s.ws.SetMocks(mocks)
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateWSMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.WebSocketMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.ws.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.ws.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteWSMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mocks := s.ws.GetMocks()
	filtered := mocks[:0]
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.ws.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// wsOnConnectPatch carries only the WS on_connect fields to change.
type wsOnConnectPatch struct {
	Send  *string          `json:"send,omitempty"`
	Delay *config.Duration `json:"delay,omitempty"`
}

func (s *Server) patchWSMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var patch wsOnConnectPatch
	if err := decodeBody(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mocks := s.ws.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			if mocks[i].OnConnect == nil {
				mocks[i].OnConnect = &config.WebSocketAction{}
			}
			if patch.Send != nil {
				mocks[i].OnConnect.Send = *patch.Send
			}
			if patch.Delay != nil {
				mocks[i].OnConnect.Delay = *patch.Delay
			}
			s.ws.SetMocks(mocks)
			writeJSON(w, http.StatusOK, mocks[i])
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

// ---------------------------------------------------------------------------
// gRPC mocks
// ---------------------------------------------------------------------------

func (s *Server) listGRPCMocks(w http.ResponseWriter, r *http.Request) {
	if s.grpc == nil {
		writeJSON(w, http.StatusOK, []config.GRPCMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.grpc.GetMocks())
}

func (s *Server) addGRPCMock(w http.ResponseWriter, r *http.Request) {
	var m config.GRPCMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("grpc-%d", time.Now().UnixNano())
	}
	mocks := s.grpc.GetMocks()
	mocks = append(mocks, m)
	s.grpc.SetMocks(mocks)
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateGRPCMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.GRPCMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.grpc.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.grpc.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteGRPCMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mocks := s.grpc.GetMocks()
	filtered := mocks[:0]
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.grpc.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// grpcResponsePatch carries only the gRPC response fields to change.
type grpcResponsePatch struct {
	Response map[string]interface{} `json:"response,omitempty"`
	Error    *config.GRPCError      `json:"error,omitempty"`
	Delay    *config.Duration       `json:"delay,omitempty"`
}

func (s *Server) patchGRPCMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var patch grpcResponsePatch
	if err := decodeBody(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mocks := s.grpc.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			if patch.Response != nil {
				mocks[i].Response = patch.Response
			}
			if patch.Error != nil {
				mocks[i].Error = patch.Error
			}
			if patch.Delay != nil {
				mocks[i].Delay = *patch.Delay
			}
			s.grpc.SetMocks(mocks)
			writeJSON(w, http.StatusOK, mocks[i])
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

func (s *Server) listScenarios(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.scenarios.All())
}

func (s *Server) createScenario(w http.ResponseWriter, r *http.Request) {
	var sc config.Scenario
	if err := decodeBody(r, &sc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created := s.scenarios.Set(sc)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) getScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sc, ok := s.scenarios.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (s *Server) updateScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sc config.Scenario
	if err := decodeBody(r, &sc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sc.ID = id
	updated := s.scenarios.Set(sc)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) deleteScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !s.scenarios.Delete(id) {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listActiveScenarios(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active":    s.scenarios.ActiveIDs(),
		"scenarios": s.scenarios.ActiveScenarios(),
	})
}

func (s *Server) activateScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !s.scenarios.Activate(id) {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"activated": id,
		"active":    s.scenarios.ActiveIDs(),
	})
}

func (s *Server) deactivateScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.scenarios.Deactivate(id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deactivated": id,
		"active":      s.scenarios.ActiveIDs(),
	})
}

// ---------------------------------------------------------------------------
// Global fault injection
// ---------------------------------------------------------------------------

func (s *Server) getFault(w http.ResponseWriter, r *http.Request) {
	fault := s.scenarios.GetFault()
	if fault == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, fault)
}

func (s *Server) setFault(w http.ResponseWriter, r *http.Request) {
	var f config.GlobalFault
	if err := decodeBody(r, &f); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	f.Enabled = true
	s.scenarios.SetFault(&f)
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) clearFault(w http.ResponseWriter, r *http.Request) {
	s.scenarios.ClearFault()
	writeJSON(w, http.StatusOK, map[string]string{"status": "fault cleared"})
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.All())
}

func (s *Server) setState(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for k, v := range body {
		s.store.Set(k, v)
	}
	writeJSON(w, http.StatusOK, s.store.All())
}

func (s *Server) deleteState(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	s.store.Delete(key)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": key})
}

// ---------------------------------------------------------------------------
// Logs
// ---------------------------------------------------------------------------

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.log.Entries())
}

func (s *Server) clearLogs(w http.ResponseWriter, r *http.Request) {
	s.log.Clear()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) streamLogs(w http.ResponseWriter, r *http.Request) {
	s.log.ServeSSE(w, r)
}

// ---------------------------------------------------------------------------
// Reset
// ---------------------------------------------------------------------------

func (s *Server) reset(w http.ResponseWriter, r *http.Request) {
	s.store.Reset()
	s.log.Clear()
	s.scenarios.ClearFault()
	for _, id := range s.scenarios.ActiveIDs() {
		s.scenarios.Deactivate(id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeBody(r *http.Request, v interface{}) error {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") || ct == "" {
		return json.NewDecoder(r.Body).Decode(v)
	}
	return fmt.Errorf("unsupported content-type: %s", ct)
}

// spaHandler serves static files and falls back to index.html for unknown paths
// so that React Router's client-side routing works on direct URL loads.
func spaHandler(files http.FileSystem) http.Handler {
	fs := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := files.Open(r.URL.Path)
		if err != nil {
			// Not found → serve index.html so the SPA can handle routing
			r2 := *r
			r2.URL = &*r.URL
			r2.URL.Path = "/"
			fs.ServeHTTP(w, &r2)
			return
		}
		f.Close()
		fs.ServeHTTP(w, r)
	})
}

// UIFileServer returns a handler that serves the embedded static UI.
// The files parameter should be an http.FileSystem of the embedded assets.
func UIFileServer(files http.FileSystem) http.Handler {
	return http.FileServer(files)
}
