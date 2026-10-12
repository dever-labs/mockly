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
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/streamregistry"
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
	cfg       *config.GRPCConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger
	streams   *streamregistry.Registry
	mu        sync.RWMutex
	mocks     []config.GRPCMock
	server    *grpc.Server
}

// New creates a Server.
func New(cfg *config.GRPCConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	mocks := []config.GRPCMock{}
	for _, svc := range cfg.Services {
		mocks = append(mocks, svc.Mocks...)
	}
	return &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		streams:   streamregistry.New(),
		mocks:     mocks,
	}
}

// SetMocks replaces the current mock list.
func (s *Server) SetMocks(mocks []config.GRPCMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.GRPCMock(nil), mocks...)
}

// GetMocks returns the current mock list.
func (s *Server) GetMocks() []config.GRPCMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
// It supports unary, server-streaming, client-streaming, and
// bidirectional-streaming RPCs — see GRPCMock.ClientStream/Stream.
func (s *Server) unknownServiceHandler(_ interface{}, stream grpc.ServerStream) error {
	start := time.Now()

	fault := s.scenarios.EffectiveGRPCFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	method, _ := grpc.Method(stream.Context())
	shortMethod := methodShortName(method)

	mock, ok := s.findMock(shortMethod)
	bidi := ok && mock.ClientStream && len(mock.Stream) > 0

	if !bidi {
		// Bidirectional streaming interleaves its own Recv/Send loop below;
		// every other mode reads its client request(s) up front.
		if ok && mock.ClientStream {
			drainClientStream(stream) // client-streaming: read until EOF
		} else {
			var incoming []byte
			_ = stream.RecvMsg(&incoming) // unary/server-streaming: one request
		}
	}

	if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
		msg := fault.Message
		matchedID := ""
		if ok {
			matchedID = mock.ID
		}
		if msg == "" {
			msg = "fault injected"
		}
		code := codes.Unavailable
		switch strings.ToUpper(fault.Code) {
		case "NOT_FOUND":
			code = codes.NotFound
		case "DEADLINE_EXCEEDED":
			code = codes.DeadlineExceeded
		case "PERMISSION_DENIED":
			code = codes.PermissionDenied
		case "RESOURCE_EXHAUSTED":
			code = codes.ResourceExhausted
		case "INTERNAL":
			code = codes.Internal
		}
		s.logEntry(method, shortMethod, int(code), start, matchedID)
		return status.Error(code, msg)
	}

	if ok {
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}

		switch {
		case bidi:
			return s.serveBidiStream(stream, mock, method, shortMethod, start)
		case len(mock.Stream) > 0:
			return s.serveServerStream(stream, mock, method, shortMethod, start)
		case mock.Error != nil:
			code := safeGRPCCode(mock.Error.Code)
			s.logEntry(method, shortMethod, int(code), start, mock.ID)
			return status.Error(code, mock.Error.Message)
		default:
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
	}

	s.logEntry(method, shortMethod, int(codes.Unimplemented), start, "")
	return status.Errorf(codes.Unimplemented, "method %s not mocked", shortMethod)
}

// drainClientStream reads messages from a client-streaming (or
// bidirectional-streaming) call until the client half-closes (io.EOF) or
// the stream otherwise errors, discarding every message.
func drainClientStream(stream grpc.ServerStream) {
	for {
		var incoming []byte
		if err := stream.RecvMsg(&incoming); err != nil {
			return
		}
	}
}

// serveServerStream sends each of mock.Stream's messages in order (after the
// client's single request has already been read), stopping early if one
// specifies Error.
//
// If mock.KeepOpen is false (the default), the call returns normally once
// Stream is exhausted, preserving prior behavior. If true, the call stays
// open afterwards — registered with the stream registry — so the stream
// management API can push further ad-hoc messages or explicitly close the
// call (optionally as an error); it only ends when the client disconnects
// or the API closes it.
func (s *Server) serveServerStream(stream grpc.ServerStream, mock config.GRPCMock, method, shortMethod string, start time.Time) error {
	if !mock.KeepOpen {
		for _, msg := range mock.Stream {
			if msg.Delay.Duration > 0 {
				time.Sleep(msg.Delay.Duration)
			}
			if msg.Error != nil {
				code := safeGRPCCode(msg.Error.Code)
				s.logEntry(method, shortMethod, int(code), start, mock.ID)
				return status.Error(code, msg.Error.Message)
			}
			body, err := json.Marshal(msg.Response)
			if err != nil {
				return status.Errorf(codes.Internal, "marshalling mock stream message: %v", err)
			}
			if err := stream.SendMsg(&body); err != nil {
				return err
			}
		}
		s.logEntry(method, shortMethod, int(codes.OK), start, mock.ID)
		return nil
	}

	pushCh, closeCh, unregister := s.registerStream(mock.ID)
	defer unregister()

	idx := 0
	for {
		var timerC <-chan time.Time
		var timer *time.Timer
		if idx < len(mock.Stream) {
			timer = time.NewTimer(mock.Stream[idx].Delay.Duration)
			timerC = timer.C
		}

		select {
		case <-stream.Context().Done():
			if timer != nil {
				timer.Stop()
			}
			s.logEntry(method, shortMethod, int(codes.OK), start, mock.ID)
			return nil
		case errMsg := <-closeCh:
			if timer != nil {
				timer.Stop()
			}
			if errMsg != "" {
				s.logEntry(method, shortMethod, int(codes.Aborted), start, mock.ID)
				return status.Error(codes.Aborted, errMsg)
			}
			s.logEntry(method, shortMethod, int(codes.OK), start, mock.ID)
			return nil
		case data := <-pushCh:
			if timer != nil {
				timer.Stop()
			}
			body := []byte(data)
			if err := stream.SendMsg(&body); err != nil {
				return err
			}
		case <-timerC:
			msg := mock.Stream[idx]
			idx++
			if msg.Error != nil {
				code := safeGRPCCode(msg.Error.Code)
				s.logEntry(method, shortMethod, int(code), start, mock.ID)
				return status.Error(code, msg.Error.Message)
			}
			body, err := json.Marshal(msg.Response)
			if err != nil {
				return status.Errorf(codes.Internal, "marshalling mock stream message: %v", err)
			}
			if err := stream.SendMsg(&body); err != nil {
				return err
			}
		}
	}
}

// serveBidiStream interleaves Recv/Send: each message received from the
// client is answered with the correspondingly-indexed mock.Stream message.
// Once mock.Stream is exhausted, further client messages are drained
// silently until it half-closes (io.EOF) or the stream otherwise errors.
//
// If mock.KeepOpen is false (the default), the call returns once the client
// half-closes, preserving prior behavior. If true, the call stays open —
// registered with the stream registry — after the client half-closes, so
// the stream management API can push further ad-hoc messages or explicitly
// close the call (optionally as an error); it only ends when the API
// closes it (a half-closed client alone no longer ends a KeepOpen call).
func (s *Server) serveBidiStream(stream grpc.ServerStream, mock config.GRPCMock, method, shortMethod string, start time.Time) error {
	if !mock.KeepOpen {
		for i := 0; ; i++ {
			var incoming []byte
			if err := stream.RecvMsg(&incoming); err != nil {
				break
			}
			if i >= len(mock.Stream) {
				continue
			}
			msg := mock.Stream[i]
			if msg.Delay.Duration > 0 {
				time.Sleep(msg.Delay.Duration)
			}
			if msg.Error != nil {
				code := safeGRPCCode(msg.Error.Code)
				s.logEntry(method, shortMethod, int(code), start, mock.ID)
				return status.Error(code, msg.Error.Message)
			}
			body, err := json.Marshal(msg.Response)
			if err != nil {
				return status.Errorf(codes.Internal, "marshalling mock stream message: %v", err)
			}
			if err := stream.SendMsg(&body); err != nil {
				return err
			}
		}
		s.logEntry(method, shortMethod, int(codes.OK), start, mock.ID)
		return nil
	}

	pushCh, closeCh, unregister := s.registerStream(mock.ID)
	defer unregister()

	type recvResult struct {
		data []byte
		err  error
	}
	recvCh := make(chan recvResult)
	recvStop := make(chan struct{})
	defer close(recvStop)
	go func() {
		for {
			var incoming []byte
			err := stream.RecvMsg(&incoming)
			select {
			case recvCh <- recvResult{data: incoming, err: err}:
			case <-recvStop:
				// The call has already ended (ctx done / explicit close);
				// nobody will ever read recvCh again, so give up instead
				// of blocking forever on this send.
				return
			}
			if err != nil {
				return
			}
		}
	}()

	idx := 0
	recvDone := false
	for {
		var rc chan recvResult
		if !recvDone {
			rc = recvCh
		}

		select {
		case <-stream.Context().Done():
			s.logEntry(method, shortMethod, int(codes.OK), start, mock.ID)
			return nil
		case errMsg := <-closeCh:
			if errMsg != "" {
				s.logEntry(method, shortMethod, int(codes.Aborted), start, mock.ID)
				return status.Error(codes.Aborted, errMsg)
			}
			s.logEntry(method, shortMethod, int(codes.OK), start, mock.ID)
			return nil
		case data := <-pushCh:
			body := []byte(data)
			if err := stream.SendMsg(&body); err != nil {
				return err
			}
		case res := <-rc:
			if res.err != nil {
				// Client half-closed (or the stream errored): a KeepOpen
				// call stays open regardless, waiting for pushes/closes.
				recvDone = true
				continue
			}
			if idx >= len(mock.Stream) {
				continue // configured list exhausted: drain silently
			}
			msg := mock.Stream[idx]
			idx++
			if msg.Delay.Duration > 0 {
				time.Sleep(msg.Delay.Duration)
			}
			if msg.Error != nil {
				code := safeGRPCCode(msg.Error.Code)
				s.logEntry(method, shortMethod, int(code), start, mock.ID)
				return status.Error(code, msg.Error.Message)
			}
			body, err := json.Marshal(msg.Response)
			if err != nil {
				return status.Errorf(codes.Internal, "marshalling mock stream message: %v", err)
			}
			if err := stream.SendMsg(&body); err != nil {
				return err
			}
		}
	}
}

// registerStream registers a new streamregistry.Connection for mockID,
// returning channels fed by the registry's Push/Close closures and an
// unregister func the caller must defer.
func (s *Server) registerStream(mockID string) (pushCh chan string, closeCh chan string, unregister func()) {
	pushCh = make(chan string, 16)
	closeCh = make(chan string, 1)
	connID := fmt.Sprintf("grpc-%p", pushCh)
	unregister = s.streams.Register(&streamregistry.Connection{
		Info: streamregistry.Info{ID: connID, Protocol: "grpc", MockID: mockID, StartedAt: time.Now()},
		Push: func(data string) error {
			select {
			case pushCh <- data:
				return nil
			default:
				return fmt.Errorf("push buffer full for stream connection %q", connID)
			}
		},
		Close: func(errMsg string) {
			select {
			case closeCh <- errMsg:
			default:
			}
		},
	})
	return pushCh, closeCh, unregister
}

// ListStreams returns info about every currently open (KeepOpen) gRPC
// streaming call.
func (s *Server) ListStreams() []streamregistry.Info {
	return s.streams.List()
}

// PushStream delivers data as an ad-hoc message to either one specific
// streaming call (connID) or every currently open call serving mockID, in
// addition to whatever that call's configured Stream messages already
// send. data must be a JSON object (it is sent as-is, matching how
// configured GRPCStreamMessage.Response values are marshalled) — it is not
// templated. Only calls made with GRPCMock.keep_open: true can be
// targeted; others return on their own once Stream is sent.
func (s *Server) PushStream(connID, mockID, data string) (int, error) {
	trimmed := strings.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid([]byte(trimmed)) {
		return 0, fmt.Errorf("data must be a JSON object: %q", data)
	}
	return s.streams.Push(connID, mockID, data)
}

// CloseStream ends either one specific streaming call (connID) or every
// currently open call serving mockID. If errMsg is non-empty the call ends
// with a codes.Aborted status carrying errMsg; otherwise it ends with
// codes.OK.
func (s *Server) CloseStream(connID, mockID, errMsg string) (int, error) {
	return s.streams.CloseConn(connID, mockID, errMsg)
}


func (s *Server) findMock(method string) (config.GRPCMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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

// safeGRPCCode converts an int to a gRPC status code without overflow.
// Valid gRPC codes are 0–16; anything outside that range maps to Unknown (2).
func safeGRPCCode(n int) codes.Code {
	if n < 0 || n > 16 {
		return codes.Unknown
	}
	return codes.Code(n)
}

// StatusInfo returns JSON-serialisable info about this server.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	n := len(s.mocks)
	s.mu.RUnlock()
	return map[string]interface{}{
		"protocol": "grpc",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    n,
	}
}
