// Package graphqlserver implements a GraphQL mock server.
package graphqlserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/streamregistry"
	"github.com/dever-labs/mockly/internal/tlsutil"
)

// Server is the GraphQL mock server.
type Server struct {
	cfg       *config.GraphQLConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger
	streams   *streamregistry.Registry

	mu     sync.RWMutex
	mocks  []config.GraphQLMock
	server *http.Server
}

// New creates a Server.
func New(cfg *config.GraphQLConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		streams:   streamregistry.New(),
		mocks:     append([]config.GraphQLMock(nil), cfg.Mocks...),
	}
}

func (s *Server) SetMocks(mocks []config.GraphQLMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.GraphQLMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.GraphQLMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.GraphQLMock(nil), s.mocks...)
}

// Start listens and serves GraphQL requests. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.HandleFunc(s.cfg.Path, s.handleGraphQL)

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.server = &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("graphql server listen %s: %w", addr, err)
	}
	ln, err = tlsutil.WrapListener(ln, s.cfg.TLS)
	if err != nil {
		return fmt.Errorf("graphql server tls: %w", err)
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

// graphqlRequest is the standard GraphQL over HTTP request body.
type graphqlRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		s.handleSubscriptionConn(w, r)
		return
	}

	start := time.Now()

	var req graphqlRequest

	if r.Method == http.MethodGet {
		req.Query = r.URL.Query().Get("query")
		req.OperationName = r.URL.Query().Get("operationName")
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"errors":[{"message":"invalid JSON body"}]}`, http.StatusBadRequest)
			return
		}
	}

	opType := extractOperationType(req.Query)

	// Handle GraphQL introspection automatically.
	if req.OperationName == "IntrospectionQuery" || strings.Contains(req.Query, "__schema") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, minimalIntrospectionResponse)
		return
	}

	// Protocol fault delay.
	fault := s.scenarios.EffectiveGraphQLFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	mock, matched := s.matchMock(opType, req.OperationName)

	if matched && len(mock.Stream) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"errors":[{"message":"this operation is a subscription; connect via WebSocket (graphql-transport-ws) on this same path to receive the stream"}]}`)
		s.log.Log(logger.Entry{
			Protocol:  "graphql",
			Method:    opType,
			Path:      req.OperationName,
			Status:    http.StatusOK,
			Duration:  time.Since(start).Milliseconds(),
			MatchedID: mock.ID,
		})
		return
	}

	if !matched {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		safeOp, _ := json.Marshal(req.OperationName)
		_, _ = fmt.Fprintf(w, `{"errors":[{"message":"no mock matched for operation %s"}]}`, safeOp)
		s.log.Log(logger.Entry{
			Protocol: "graphql",
			Method:   opType,
			Path:     req.OperationName,
			Status:   http.StatusOK,
			Duration: time.Since(start).Milliseconds(),
		})
		return
	}

	responseData := mock.Response
	responseErrors := mock.Errors
	delay := mock.Delay.Duration
	httpStatus := http.StatusOK

	if patch := s.scenarios.PatchFor(mock.ID); patch != nil {
		if patch.Disabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"errors":[{"message":"mock disabled by active scenario"}]}`)
			return
		}
		if patch.Body != "" {
			if delay > 0 {
				time.Sleep(delay)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(httpStatus)
			_, _ = fmt.Fprint(w, patch.Body)
			return
		}
		if patch.Status != 0 {
			httpStatus = patch.Status
		}
		if patch.Delay != nil {
			delay = patch.Delay.Duration
		}
	}

	if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
		st := fault.Status
		if st == 0 {
			st = http.StatusServiceUnavailable
		}
		body := fault.Body
		if body == "" {
			body = `{"error":"fault injected"}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(st)
		_, _ = fmt.Fprint(w, body)
		return
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	gqlResp := map[string]interface{}{}
	if responseData != nil {
		gqlResp["data"] = responseData
	} else {
		gqlResp["data"] = nil
	}
	if len(responseErrors) > 0 {
		gqlResp["errors"] = responseErrors
	}

	respBytes, _ := json.Marshal(gqlResp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_, _ = fmt.Fprint(w, string(respBytes))

	s.log.Log(logger.Entry{
		Protocol:  "graphql",
		Method:    opType,
		Path:      req.OperationName,
		Status:    httpStatus,
		Duration:  time.Since(start).Milliseconds(),
		MatchedID: mock.ID,
	})
}

// subscriptionProtocols lists the WebSocket subprotocol(s) the GraphQL
// subscription transport accepts. Mockly implements graphql-transport-ws
// (https://github.com/enisdenjo/graphql-ws/blob/master/PROTOCOL.md), the
// modern `graphql-ws` npm package's protocol; the legacy
// `subscriptions-transport-ws` ("graphql-ws" subprotocol, different message
// shapes) is not implemented.
var subscriptionProtocols = []string{"graphql-transport-ws"}

var gqlWSUpgrader = websocket.Upgrader{
	CheckOrigin:  func(r *http.Request) bool { return true },
	Subprotocols: subscriptionProtocols,
}

// graphqlWSMessage is the graphql-transport-ws envelope: every message in
// both directions has a Type, most carry a client-chosen ID identifying the
// subscription they belong to (so one connection can multiplex several
// concurrent subscriptions), and some carry a Payload.
type graphqlWSMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// handleSubscriptionConn upgrades the request to a graphql-transport-ws
// WebSocket connection and serves GraphQL subscriptions over it:
// connection_init -> connection_ack once, then any number of
// subscribe -> next*/complete (or error) exchanges multiplexed by ID on the
// same connection, each driven by a GraphQLMock.Stream.
func (s *Server) handleSubscriptionConn(w http.ResponseWriter, r *http.Request) {
	conn, err := gqlWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close() //nolint:errcheck

	var writeMu sync.Mutex
	var activeMu sync.Mutex
	active := map[string]context.CancelFunc{}
	defer func() {
		activeMu.Lock()
		for id, cancel := range active {
			cancel()
			delete(active, id)
		}
		activeMu.Unlock()
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg graphqlWSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "connection_init":
			writeWSMessage(conn, &writeMu, graphqlWSMessage{Type: "connection_ack"})
		case "ping":
			writeWSMessage(conn, &writeMu, graphqlWSMessage{Type: "pong"})
		case "subscribe":
			var payload graphqlRequest
			_ = json.Unmarshal(msg.Payload, &payload)
			id := msg.ID
			subCtx, cancel := context.WithCancel(r.Context())
			activeMu.Lock()
			active[id] = cancel
			activeMu.Unlock()
			go func() {
				defer func() {
					// Always release the context (required even on a
					// normal, non-cancelled finish) and drop the map entry
					// once this subscription is done, whether it finished
					// naturally (stream exhausted, no match, fault) or was
					// cancelled by a client "complete"/connection close.
					activeMu.Lock()
					delete(active, id)
					activeMu.Unlock()
					cancel()
				}()
				s.runSubscription(subCtx, conn, &writeMu, id, payload, r)
			}()
		case "complete":
			activeMu.Lock()
			if cancel, ok := active[msg.ID]; ok {
				cancel()
				delete(active, msg.ID)
			}
			activeMu.Unlock()
		}
	}
}

// writeWSMessage serializes and sends a single graphql-transport-ws
// message, holding mu for the duration of the write since gorilla/websocket
// connections aren't safe for concurrent writes and each active
// subscription writes from its own goroutine.
func writeWSMessage(conn *websocket.Conn, mu *sync.Mutex, msg graphqlWSMessage) {
	mu.Lock()
	defer mu.Unlock()
	_ = conn.WriteJSON(msg)
}

func wsErrorPayload(message string) json.RawMessage {
	b, _ := json.Marshal([]map[string]interface{}{{"message": message}})
	return b
}

// runSubscription drives a single "subscribe" request: it matches req
// against the configured mocks same as a query/mutation would, then pushes
// each of the matched mock's Stream events in order as "next" messages
// (respecting each event's Delay). Once the configured list is exhausted,
// the subscription is NOT completed automatically — it stays open,
// registered with the stream registry, waiting for either an externally
// pushed event (via the management API's stream-push endpoint), an
// explicit close (via the same API), the client sending "complete", or the
// connection closing. This mirrors how a real subscription behaves: it
// keeps delivering events for as long as the thing it's subscribed to is
// still producing them, not just for a fixed configured count.
func (s *Server) runSubscription(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, id string, req graphqlRequest, httpReq *http.Request) {
	start := time.Now()
	opType := extractOperationType(req.Query)
	mock, matched := s.matchMock(opType, req.OperationName)

	if !matched || len(mock.Stream) == 0 {
		writeWSMessage(conn, writeMu, graphqlWSMessage{
			ID: id, Type: "error",
			Payload: wsErrorPayload(fmt.Sprintf("no subscription mock matched for operation %s", req.OperationName)),
		})
		return
	}

	fault := s.scenarios.EffectiveGraphQLFault()
	if fault != nil && fault.Delay.Duration > 0 {
		select {
		case <-time.After(fault.Delay.Duration):
		case <-ctx.Done():
			return
		}
	}
	if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
		msg := fault.Body
		if msg == "" {
			msg = "fault injected"
		}
		writeWSMessage(conn, writeMu, graphqlWSMessage{ID: id, Type: "error", Payload: wsErrorPayload(msg)})
		return
	}

	reqCtx := engine.RequestContext{Path: httpReq.URL.Path, Body: req.Query}

	connID := fmt.Sprintf("graphql-%p-%s", conn, id)
	pushCh := make(chan string, 16)
	closeCh := make(chan string, 1)
	unregister := s.streams.Register(&streamregistry.Connection{
		Info: streamregistry.Info{ID: connID, Protocol: "graphql", MockID: mock.ID, StartedAt: start},
		Push: func(data string) error {
			select {
			case pushCh <- data:
				return nil
			default:
				return fmt.Errorf("push buffer full for subscription %q", connID)
			}
		},
		Close: func(errMsg string) {
			select {
			case closeCh <- errMsg:
			default:
			}
		},
	})
	defer unregister()

	sendEvent := func(data string) (ok bool) {
		if !json.Valid([]byte(data)) {
			writeWSMessage(conn, writeMu, graphqlWSMessage{
				ID: id, Type: "error",
				Payload: wsErrorPayload(fmt.Sprintf("stream event data is not valid JSON: %q", data)),
			})
			return false
		}
		writeWSMessage(conn, writeMu, graphqlWSMessage{ID: id, Type: "next", Payload: json.RawMessage(data)})
		return true
	}

	idx := 0
	for {
		var timerC <-chan time.Time
		var timer *time.Timer
		if idx < len(mock.Stream) {
			timer = time.NewTimer(mock.Stream[idx].Delay.Duration)
			timerC = timer.C
		}

		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case errMsg := <-closeCh:
			if timer != nil {
				timer.Stop()
			}
			if errMsg != "" {
				writeWSMessage(conn, writeMu, graphqlWSMessage{ID: id, Type: "error", Payload: wsErrorPayload(errMsg)})
			} else {
				writeWSMessage(conn, writeMu, graphqlWSMessage{ID: id, Type: "complete"})
			}
			s.logSubscription(opType, req.OperationName, mock.ID, start)
			return
		case data := <-pushCh:
			if timer != nil {
				timer.Stop()
			}
			if !sendEvent(data) {
				s.logSubscription(opType, req.OperationName, mock.ID, start)
				return
			}
			// An externally pushed event doesn't consume the configured
			// schedule; keep waiting for idx's own timer next iteration.
		case <-timerC:
			ev := mock.Stream[idx]
			idx++
			if !sendEvent(engine.Render(ev.Data, reqCtx)) {
				s.logSubscription(opType, req.OperationName, mock.ID, start)
				return
			}
		}
	}
}

func (s *Server) logSubscription(opType, opName, mockID string, start time.Time) {
	s.log.Log(logger.Entry{
		Protocol:  "graphql",
		Method:    opType,
		Path:      opName,
		Status:    http.StatusOK,
		Duration:  time.Since(start).Milliseconds(),
		MatchedID: mockID,
	})
}

func (s *Server) matchMock(opType, opName string) (config.GraphQLMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if m.OperationType != "" && !strings.EqualFold(m.OperationType, opType) {
			continue
		}
		if m.OperationName != "" && !matchPattern(m.OperationName, opName) {
			continue
		}
		return m, true
	}
	return config.GraphQLMock{}, false
}

func matchPattern(pattern, value string) bool {
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(value)
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, pattern[:len(pattern)-1])
	}
	return pattern == value
}

func extractOperationType(query string) string {
	q := strings.TrimSpace(query)
	if strings.HasPrefix(q, "mutation") {
		return "mutation"
	}
	if strings.HasPrefix(q, "subscription") {
		return "subscription"
	}
	return "query"
}

// ListStreams returns info about every currently active subscription.
func (s *Server) ListStreams() []streamregistry.Info {
	return s.streams.List()
}

// PushStream sends data as an ad-hoc "next" message to either one specific
// subscription (connID) or every currently active subscription serving
// mockID, in addition to whatever that subscription's configured stream
// list already sends. data must be valid JSON (the GraphQL execution
// result, e.g. `{"data":{...}}`); it is sent verbatim, not templated.
func (s *Server) PushStream(connID, mockID, data string) (int, error) {
	if !json.Valid([]byte(data)) {
		return 0, fmt.Errorf("data is not valid JSON: %q", data)
	}
	return s.streams.Push(connID, mockID, data)
}

// CloseStream ends either one specific subscription (connID) or every
// currently active subscription serving mockID. If errMsg is non-empty the
// subscription ends with a graphql-transport-ws "error" message instead of
// "complete".
func (s *Server) CloseStream(connID, mockID, errMsg string) (int, error) {
	return s.streams.CloseConn(connID, mockID, errMsg)
}

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	n := len(s.mocks)
	s.mu.RUnlock()
	tlsEnabled := s.cfg.TLS != nil && s.cfg.TLS.Enabled
	return map[string]interface{}{
		"protocol": "graphql",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"path":     s.cfg.Path,
		"tls":      tlsEnabled,
		"mocks":    n,
	}
}

const minimalIntrospectionResponse = `{"data":{"__schema":{"queryType":{"name":"Query"},"mutationType":{"name":"Mutation"},"subscriptionType":{"name":"Subscription"},"types":[],"directives":[]}}}`
