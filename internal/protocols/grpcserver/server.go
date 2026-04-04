// Package grpcserver implements a dynamic gRPC mock server.
// It uses grpc.UnknownServiceHandler with a raw-bytes codec so that any gRPC
// client can call any method and receive the mocked JSON-encoded response,
// without needing to compile .proto files into the binary.
package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

func init() {
	// Override the default "proto" codec with a raw bytes passthrough so we can
	// receive and send arbitrary byte payloads without generated protobuf code.
	encoding.RegisterCodec(rawCodec{})
}

// rawCodec passes bytes through unchanged, overriding the default protobuf codec.
type rawCodec struct{}

func (rawCodec) Name() string { return "proto" }

func (rawCodec) Marshal(v interface{}) ([]byte, error) {
	if b, ok := v.(*[]byte); ok {
		return *b, nil
	}
	return json.Marshal(v)
}

func (rawCodec) Unmarshal(data []byte, v interface{}) error {
	if b, ok := v.(*[]byte); ok {
		*b = data
		return nil
	}
	return json.Unmarshal(data, v)
}

// Server is the gRPC mock server.
type Server struct {
	cfg    *config.GRPCConfig
	store  *state.Store
	log    *logger.Logger
	mocks  []config.GRPCMock
	server *grpc.Server
}

// New creates a Server.
func New(cfg *config.GRPCConfig, store *state.Store, log *logger.Logger) *Server {
	mocks := []config.GRPCMock{}
	for _, svc := range cfg.Services {
		mocks = append(mocks, svc.Mocks...)
	}
	return &Server{
		cfg:   cfg,
		store: store,
		log:   log,
		mocks: mocks,
	}
}

// SetMocks replaces the current mock list.
func (s *Server) SetMocks(mocks []config.GRPCMock) {
	s.mocks = append([]config.GRPCMock(nil), mocks...)
}

// GetMocks returns the current mock list.
func (s *Server) GetMocks() []config.GRPCMock {
	return append([]config.GRPCMock(nil), s.mocks...)
}

// Start begins listening. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc server listen %s: %w", addr, err)
	}

	s.server = grpc.NewServer(
		grpc.UnknownServiceHandler(s.unknownServiceHandler),
	)
	reflection.Register(s.server)

	errCh := make(chan error, 1)
	go func() { errCh <- s.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		s.server.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

// unknownServiceHandler intercepts all calls and returns a mocked response.
func (s *Server) unknownServiceHandler(_ interface{}, stream grpc.ServerStream) error {
	start := time.Now()

	method, _ := grpc.Method(stream.Context())
	shortMethod := methodShortName(method)

	mock, ok := s.findMock(shortMethod)

	var incoming []byte
	_ = stream.RecvMsg(&incoming)

	if ok {
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}

		if mock.Error != nil {
			s.logEntry(method, shortMethod, int(codes.Code(mock.Error.Code)), start, mock.ID)
		return status.Error(codes.Code(mock.Error.Code), mock.Error.Message)
		}

		body, err := json.Marshal(mock.Response)
		if err != nil {
			return status.Errorf(codes.Internal, "marshalling mock response: %v", err)
		}

		if err := stream.SendMsg(&body); err != nil {
			return err
		}

		s.logEntry(method, shortMethod, int(codes.OK), start, mock.ID)
		return nil
	}

	s.logEntry(method, shortMethod, int(codes.Unimplemented), start, "")
	return status.Errorf(codes.Unimplemented, "method %s not mocked", shortMethod)
}

func (s *Server) findMock(method string) (config.GRPCMock, bool) {
	for _, m := range s.mocks {
		if m.Method == method || m.Method == "*" {
			return m, true
		}
	}
	return config.GRPCMock{}, false
}

func (s *Server) logEntry(path, method string, grpcStatus int, start time.Time, matchedID string) {
	s.log.Log(logger.Entry{
		Protocol:  "grpc",
		Method:    method,
		Path:      path,
		Status:    grpcStatus,
		Duration:  time.Since(start).Milliseconds(),
		MatchedID: matchedID,
	})
}

func methodShortName(full string) string {
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '/' {
			return full[i+1:]
		}
	}
	return full
}

// StatusInfo returns JSON-serialisable info about this server.
func (s *Server) StatusInfo() map[string]interface{} {
	return map[string]interface{}{
		"protocol": "grpc",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
	}
}

