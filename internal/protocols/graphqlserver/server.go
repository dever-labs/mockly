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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// Server is the GraphQL mock server.
type Server struct {
	cfg       *config.GraphQLConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger
	mocks     []config.GraphQLMock
	server    *http.Server
}

// New creates a Server.
func New(cfg *config.GraphQLConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		mocks:     append([]config.GraphQLMock(nil), cfg.Mocks...),
	}
}

func (s *Server) SetMocks(mocks []config.GraphQLMock) {
	s.mocks = append([]config.GraphQLMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.GraphQLMock {
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

	// Global fault delay.
	fault := s.scenarios.GetFault()
	if fault != nil && fault.Enabled && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	mock, matched := s.matchMock(opType, req.OperationName)

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

	// Global fault status override.
	if fault != nil && fault.Enabled && fault.StatusOverride != 0 && s.scenarios.RollFault(fault.ErrorRate) {
		httpStatus = fault.StatusOverride
		if fault.Body != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(httpStatus)
			_, _ = fmt.Fprint(w, fault.Body)
			return
		}
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

func (s *Server) matchMock(opType, opName string) (config.GraphQLMock, bool) {
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

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	return map[string]interface{}{
		"protocol": "graphql",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"path":     s.cfg.Path,
		"mocks":    len(s.mocks),
	}
}

const minimalIntrospectionResponse = `{"data":{"__schema":{"queryType":{"name":"Query"},"mutationType":{"name":"Mutation"},"subscriptionType":{"name":"Subscription"},"types":[],"directives":[]}}}`
