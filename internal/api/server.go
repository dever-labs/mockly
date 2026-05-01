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
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/tlsutil"
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
	CallCount(mockID string) int64
	ResetCallCounts()
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

// GraphQLProtocol is the subset of graphqlserver.Server used by the API.
type GraphQLProtocol interface {
	ProtocolServer
	GetMocks() []config.GraphQLMock
	SetMocks([]config.GraphQLMock)
}

// TCPProtocol is the subset of tcpserver.Server used by the API.
type TCPProtocol interface {
	ProtocolServer
	GetMocks() []config.TCPMock
	SetMocks([]config.TCPMock)
}

// RedisProtocol is the subset of redisserver.Server used by the API.
type RedisProtocol interface {
	ProtocolServer
	GetMocks() []config.RedisMock
	SetMocks([]config.RedisMock)
}

// SMTPProtocol is the subset of smtpserver.Server used by the API.
type SMTPProtocol interface {
	ProtocolServer
	GetRules() []config.SMTPRule
	SetRules([]config.SMTPRule)
	GetInbox() *smtpserver.Inbox
}

// MQTTProtocol is the subset of mqttserver.Server used by the API.
type MQTTProtocol interface {
	ProtocolServer
	GetMocks() []config.MQTTMock
	SetMocks([]config.MQTTMock)
	GetMessageStore() *mqttserver.MessageStore
}

// SNMPProtocol is the subset of snmpserver.Server used by the API.
type SNMPProtocol interface {
	ProtocolServer
	GetMocks() []config.SNMPMock
	SetMocks([]config.SNMPMock)
	GetTraps() []config.SNMPTrap
	SetTraps([]config.SNMPTrap)
	SendTrap(id string) error
}

// Server is the management API HTTP server.
type Server struct {
	cfg       *config.Config
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger
	http      HTTPProtocol
	ws        WSProtocol
	grpc      GRPCProtocol
	graphql   GraphQLProtocol
	tcp       TCPProtocol
	redis     RedisProtocol
	smtp      SMTPProtocol
	mqtt      MQTTProtocol
	snmp      SNMPProtocol
	server    *http.Server
	uiFiles   http.FileSystem
}

// New creates a management API Server.
func New(
	cfg *config.Config,
	store *state.Store,
	sc *scenarios.Store,
	log *logger.Logger,
	httpSrv HTTPProtocol,
	wsSrv WSProtocol,
	grpcSrv GRPCProtocol,
	graphqlSrv GraphQLProtocol,
	tcpSrv TCPProtocol,
	redisSrv RedisProtocol,
	smtpSrv SMTPProtocol,
	mqttSrv MQTTProtocol,
	snmpSrv SNMPProtocol,
) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		http:      httpSrv,
		ws:        wsSrv,
		grpc:      grpcSrv,
		graphql:   graphqlSrv,
		tcp:       tcpSrv,
		redis:     redisSrv,
		smtp:      smtpSrv,
		mqtt:      mqttSrv,
		snmp:      snmpSrv,
	}
}

// Start begins listening. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := s.buildRouter()
	addr := fmt.Sprintf(":%d", s.cfg.Mockly.API.Port)
	s.server = &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("api server listen %s: %w", addr, err)
	}
	ln, err = tlsutil.WrapListener(ln, s.cfg.Mockly.API.TLS)
	if err != nil {
		return fmt.Errorf("api server tls: %w", err)
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

		// GraphQL mocks
		r.Get("/api/mocks/graphql", s.listGraphQLMocks)
		r.Post("/api/mocks/graphql", s.addGraphQLMock)
		r.Put("/api/mocks/graphql/{id}", s.updateGraphQLMock)
		r.Delete("/api/mocks/graphql/{id}", s.deleteGraphQLMock)

		// TCP mocks
		r.Get("/api/mocks/tcp", s.listTCPMocks)
		r.Post("/api/mocks/tcp", s.addTCPMock)
		r.Put("/api/mocks/tcp/{id}", s.updateTCPMock)
		r.Delete("/api/mocks/tcp/{id}", s.deleteTCPMock)

		// Redis mocks
		r.Get("/api/mocks/redis", s.listRedisMocks)
		r.Post("/api/mocks/redis", s.addRedisMock)
		r.Put("/api/mocks/redis/{id}", s.updateRedisMock)
		r.Delete("/api/mocks/redis/{id}", s.deleteRedisMock)

		// SMTP rules + inbox
		r.Get("/api/mocks/smtp", s.listSMTPRules)
		r.Post("/api/mocks/smtp", s.addSMTPRule)
		r.Put("/api/mocks/smtp/{id}", s.updateSMTPRule)
		r.Delete("/api/mocks/smtp/{id}", s.deleteSMTPRule)
		r.Get("/api/emails", s.listEmails)
		r.Delete("/api/emails", s.clearEmails)

		// MQTT mocks + captured messages
		r.Get("/api/mocks/mqtt", s.listMQTTMocks)
		r.Post("/api/mocks/mqtt", s.addMQTTMock)
		r.Put("/api/mocks/mqtt/{id}", s.updateMQTTMock)
		r.Delete("/api/mocks/mqtt/{id}", s.deleteMQTTMock)
		r.Get("/api/mqtt/messages", s.listMQTTMessages)
		r.Delete("/api/mqtt/messages", s.clearMQTTMessages)

		// SNMP mocks + traps
		r.Get("/api/mocks/snmp", s.listSNMPMocks)
		r.Post("/api/mocks/snmp", s.addSNMPMock)
		r.Put("/api/mocks/snmp/{id}", s.updateSNMPMock)
		r.Delete("/api/mocks/snmp/{id}", s.deleteSNMPMock)
		r.Get("/api/snmp/traps", s.listSNMPTraps)
		r.Post("/api/snmp/traps", s.addSNMPTrap)
		r.Post("/api/snmp/traps/{id}/send", s.sendSNMPTrap)

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
		r.Post("/api/scenarios/{id}/deactivate", s.deactivateScenario)

		// Global fault injection
		r.Get("/api/fault", s.getFault)
		r.Post("/api/fault", s.setFault)
		r.Delete("/api/fault", s.clearFault)

		r.Get("/api/logs", s.getLogs)
		r.Delete("/api/logs", s.clearLogs)
		r.Get("/api/logs/stream", s.streamLogs)

		// Call verification
		r.Get("/api/calls/http/{mockId}", s.getHTTPCalls)
		r.Delete("/api/calls/http/{mockId}", s.clearHTTPMockCalls)
		r.Delete("/api/calls/http", s.clearAllHTTPCalls)
		r.Post("/api/calls/http/{mockId}/wait", s.waitHTTPCalls)

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
	for _, p := range []ProtocolServer{s.http, s.ws, s.grpc, s.graphql, s.tcp, s.redis, s.smtp, s.mqtt, s.snmp} {
		if p != nil {
			protocols = append(protocols, p.StatusInfo())
		}
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
	filtered := make([]config.HTTPMock, 0, len(mocks))
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
	filtered := make([]config.WebSocketMock, 0, len(mocks))
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
	filtered := make([]config.GRPCMock, 0, len(mocks))
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
// GraphQL mocks
// ---------------------------------------------------------------------------

func (s *Server) listGraphQLMocks(w http.ResponseWriter, r *http.Request) {
	if s.graphql == nil {
		writeJSON(w, http.StatusOK, []config.GraphQLMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.graphql.GetMocks())
}

func (s *Server) addGraphQLMock(w http.ResponseWriter, r *http.Request) {
	var m config.GraphQLMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("gql-%d", time.Now().UnixNano())
	}
	mocks := s.graphql.GetMocks()
	s.graphql.SetMocks(append(mocks, m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateGraphQLMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.GraphQLMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.graphql.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.graphql.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteGraphQLMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mocks := s.graphql.GetMocks()
	filtered := make([]config.GraphQLMock, 0, len(mocks))
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.graphql.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// ---------------------------------------------------------------------------
// TCP mocks
// ---------------------------------------------------------------------------

func (s *Server) listTCPMocks(w http.ResponseWriter, r *http.Request) {
	if s.tcp == nil {
		writeJSON(w, http.StatusOK, []config.TCPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.tcp.GetMocks())
}

func (s *Server) addTCPMock(w http.ResponseWriter, r *http.Request) {
	var m config.TCPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("tcp-%d", time.Now().UnixNano())
	}
	s.tcp.SetMocks(append(s.tcp.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateTCPMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.TCPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.tcp.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.tcp.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteTCPMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mocks := s.tcp.GetMocks()
	filtered := make([]config.TCPMock, 0, len(mocks))
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.tcp.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// ---------------------------------------------------------------------------
// Redis mocks
// ---------------------------------------------------------------------------

func (s *Server) listRedisMocks(w http.ResponseWriter, r *http.Request) {
	if s.redis == nil {
		writeJSON(w, http.StatusOK, []config.RedisMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.redis.GetMocks())
}

func (s *Server) addRedisMock(w http.ResponseWriter, r *http.Request) {
	var m config.RedisMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("redis-%d", time.Now().UnixNano())
	}
	s.redis.SetMocks(append(s.redis.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateRedisMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.RedisMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.redis.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.redis.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteRedisMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mocks := s.redis.GetMocks()
	filtered := make([]config.RedisMock, 0, len(mocks))
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.redis.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// ---------------------------------------------------------------------------
// SMTP rules + inbox
// ---------------------------------------------------------------------------

func (s *Server) listSMTPRules(w http.ResponseWriter, r *http.Request) {
	if s.smtp == nil {
		writeJSON(w, http.StatusOK, []config.SMTPRule{})
		return
	}
	writeJSON(w, http.StatusOK, s.smtp.GetRules())
}

func (s *Server) addSMTPRule(w http.ResponseWriter, r *http.Request) {
	var rule config.SMTPRule
	if err := decodeBody(r, &rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("smtp-%d", time.Now().UnixNano())
	}
	if rule.Action == "" {
		rule.Action = "accept"
	}
	s.smtp.SetRules(append(s.smtp.GetRules(), rule))
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) updateSMTPRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.SMTPRule
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	rules := s.smtp.GetRules()
	for i, rule := range rules {
		if rule.ID == id {
			rules[i] = updated
			s.smtp.SetRules(rules)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "rule not found")
}

func (s *Server) deleteSMTPRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rules := s.smtp.GetRules()
	filtered := make([]config.SMTPRule, 0, len(rules))
	for _, rule := range rules {
		if rule.ID != id {
			filtered = append(filtered, rule)
		}
	}
	s.smtp.SetRules(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listEmails(w http.ResponseWriter, r *http.Request) {
	if s.smtp == nil {
		writeJSON(w, http.StatusOK, []config.ReceivedEmail{})
		return
	}
	writeJSON(w, http.StatusOK, s.smtp.GetInbox().All())
}

func (s *Server) clearEmails(w http.ResponseWriter, r *http.Request) {
	if s.smtp != nil {
		s.smtp.GetInbox().Clear()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// ---------------------------------------------------------------------------
// MQTT mocks + captured messages
// ---------------------------------------------------------------------------

func (s *Server) listMQTTMocks(w http.ResponseWriter, r *http.Request) {
	if s.mqtt == nil {
		writeJSON(w, http.StatusOK, []config.MQTTMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.mqtt.GetMocks())
}

func (s *Server) addMQTTMock(w http.ResponseWriter, r *http.Request) {
	var m config.MQTTMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("mqtt-%d", time.Now().UnixNano())
	}
	s.mqtt.SetMocks(append(s.mqtt.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateMQTTMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updated config.MQTTMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.mqtt.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.mqtt.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteMQTTMock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mocks := s.mqtt.GetMocks()
	filtered := make([]config.MQTTMock, 0, len(mocks))
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.mqtt.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listMQTTMessages(w http.ResponseWriter, r *http.Request) {
	if s.mqtt == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, s.mqtt.GetMessageStore().All())
}

func (s *Server) clearMQTTMessages(w http.ResponseWriter, r *http.Request) {
	if s.mqtt != nil {
		s.mqtt.GetMessageStore().Clear()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// ---------------------------------------------------------------------------
// SNMP mocks + traps
// ---------------------------------------------------------------------------

func (s *Server) listSNMPMocks(w http.ResponseWriter, r *http.Request) {
	if s.snmp == nil {
		writeJSON(w, http.StatusOK, []config.SNMPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.snmp.GetMocks())
}

func (s *Server) addSNMPMock(w http.ResponseWriter, r *http.Request) {
	if s.snmp == nil {
		writeError(w, http.StatusServiceUnavailable, "snmp protocol not enabled")
		return
	}
	var m config.SNMPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("snmp-%d", time.Now().UnixNano())
	}
	s.snmp.SetMocks(append(s.snmp.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateSNMPMock(w http.ResponseWriter, r *http.Request) {
	if s.snmp == nil {
		writeError(w, http.StatusServiceUnavailable, "snmp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.SNMPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.snmp.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.snmp.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteSNMPMock(w http.ResponseWriter, r *http.Request) {
	if s.snmp == nil {
		writeError(w, http.StatusServiceUnavailable, "snmp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.snmp.GetMocks()
	filtered := make([]config.SNMPMock, 0, len(mocks))
	for _, m := range mocks {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	s.snmp.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listSNMPTraps(w http.ResponseWriter, r *http.Request) {
	if s.snmp == nil {
		writeJSON(w, http.StatusOK, []config.SNMPTrap{})
		return
	}
	writeJSON(w, http.StatusOK, s.snmp.GetTraps())
}

func (s *Server) addSNMPTrap(w http.ResponseWriter, r *http.Request) {
	if s.snmp == nil {
		writeError(w, http.StatusServiceUnavailable, "snmp protocol not enabled")
		return
	}
	var t config.SNMPTrap
	if err := decodeBody(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("trap-%d", time.Now().UnixNano())
	}
	s.snmp.SetTraps(append(s.snmp.GetTraps(), t))
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) sendSNMPTrap(w http.ResponseWriter, r *http.Request) {
	if s.snmp == nil {
		writeError(w, http.StatusServiceUnavailable, "snmp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.snmp.SendTrap(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"sent": id})
}

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
	engine.ResetSequences()
	for _, id := range s.scenarios.ActiveIDs() {
		s.scenarios.Deactivate(id)
	}

	// Restore each protocol's mocks to the values from the original config.
	if s.http != nil {
		var mocks []config.HTTPMock
		if s.cfg.Protocols.HTTP != nil {
			mocks = s.cfg.Protocols.HTTP.Mocks
		}
		s.http.SetMocks(mocks) // SetMocks also resets call counts
	}
	if s.ws != nil {
		var mocks []config.WebSocketMock
		if s.cfg.Protocols.WebSocket != nil {
			mocks = s.cfg.Protocols.WebSocket.Mocks
		}
		s.ws.SetMocks(mocks)
	}
	if s.grpc != nil {
		var mocks []config.GRPCMock
		if s.cfg.Protocols.GRPC != nil {
			for _, svc := range s.cfg.Protocols.GRPC.Services {
				mocks = append(mocks, svc.Mocks...)
			}
		}
		s.grpc.SetMocks(mocks)
	}
	if s.graphql != nil {
		var mocks []config.GraphQLMock
		if s.cfg.Protocols.GraphQL != nil {
			mocks = s.cfg.Protocols.GraphQL.Mocks
		}
		s.graphql.SetMocks(mocks)
	}
	if s.tcp != nil {
		var mocks []config.TCPMock
		if s.cfg.Protocols.TCP != nil {
			mocks = s.cfg.Protocols.TCP.Mocks
		}
		s.tcp.SetMocks(mocks)
	}
	if s.redis != nil {
		var mocks []config.RedisMock
		if s.cfg.Protocols.Redis != nil {
			mocks = s.cfg.Protocols.Redis.Mocks
		}
		s.redis.SetMocks(mocks)
	}
	if s.smtp != nil {
		var rules []config.SMTPRule
		if s.cfg.Protocols.SMTP != nil {
			rules = s.cfg.Protocols.SMTP.Rules
		}
		s.smtp.SetRules(rules)
		s.smtp.GetInbox().Clear()
	}
	if s.mqtt != nil {
		var mocks []config.MQTTMock
		if s.cfg.Protocols.MQTT != nil {
			mocks = s.cfg.Protocols.MQTT.Mocks
		}
		s.mqtt.SetMocks(mocks)
		s.mqtt.GetMessageStore().Clear()
	}
	if s.snmp != nil {
		var mocks []config.SNMPMock
		if s.cfg.Protocols.SNMP != nil {
			mocks = s.cfg.Protocols.SNMP.Mocks
		}
		s.snmp.SetMocks(mocks)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// ---------------------------------------------------------------------------
// Call verification
// ---------------------------------------------------------------------------

type callSummary struct {
	MockID string         `json:"mock_id"`
	Count  int64          `json:"count"`
	Calls  []logger.Entry `json:"calls"`
}

func (s *Server) getHTTPCalls(w http.ResponseWriter, r *http.Request) {
	if s.http == nil {
		writeError(w, http.StatusServiceUnavailable, "http protocol not enabled")
		return
	}
	mockID := chi.URLParam(r, "mockId")
	writeJSON(w, http.StatusOK, callSummary{
		MockID: mockID,
		Count:  s.http.CallCount(mockID),
		Calls:  s.log.EntriesByMockID(mockID),
	})
}

func (s *Server) clearHTTPMockCalls(w http.ResponseWriter, r *http.Request) {
	if s.http == nil {
		writeError(w, http.StatusServiceUnavailable, "http protocol not enabled")
		return
	}
	// Clear log entries for this specific mock by rebuilding without them.
	mockID := chi.URLParam(r, "mockId")
	s.log.ClearByMockID(mockID)
	s.http.ResetCallCounts() // simplest: reset all counts (full reset per mock not tracked separately)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) clearAllHTTPCalls(w http.ResponseWriter, r *http.Request) {
	if s.http == nil {
		writeError(w, http.StatusServiceUnavailable, "http protocol not enabled")
		return
	}
	s.log.Clear()
	s.http.ResetCallCounts()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// waitHTTPCalls blocks until a mock has been called at least N times,
// or until the timeout expires.
//
//	POST /api/calls/http/{mockId}/wait
//	Body: {"count": 3, "timeout": "5s"}
func (s *Server) waitHTTPCalls(w http.ResponseWriter, r *http.Request) {
	if s.http == nil {
		writeError(w, http.StatusServiceUnavailable, "http protocol not enabled")
		return
	}
	mockID := chi.URLParam(r, "mockId")

	var req struct {
		Count   int    `json:"count"`
		Timeout string `json:"timeout"`
	}
	req.Count = 1
	req.Timeout = "10s"
	_ = json.NewDecoder(r.Body).Decode(&req)

	timeout, err := time.ParseDuration(req.Timeout)
	if err != nil || timeout <= 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	entries, err := s.log.WaitFor(ctx, mockID, req.Count)
	if err != nil {
		writeJSON(w, http.StatusRequestTimeout, map[string]interface{}{
			"error":   "timeout waiting for calls",
			"mock_id": mockID,
			"want":    req.Count,
			"got":     len(entries),
		})
		return
	}
	writeJSON(w, http.StatusOK, callSummary{
		MockID: mockID,
		Count:  int64(len(entries)),
		Calls:  entries,
	})
}
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
			urlCopy := *r.URL
			r2.URL = &urlCopy
			r2.URL.Path = "/"
			fs.ServeHTTP(w, &r2)
			return
		}
		_ = f.Close()
		fs.ServeHTTP(w, r)
	})
}

// UIFileServer returns a handler that serves the embedded static UI.
// The files parameter should be an http.FileSystem of the embedded assets.
func UIFileServer(files http.FileSystem) http.Handler {
	return http.FileServer(files)
}
