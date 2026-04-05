// Package tcpserver implements a raw TCP mock server.
package tcpserver

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

// Server is the TCP mock server.
type Server struct {
	cfg      *config.TCPConfig
	store    *state.Store
	log      *logger.Logger
	mocks    []config.TCPMock
	listener net.Listener
}

// New creates a Server.
func New(cfg *config.TCPConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:   cfg,
		store: store,
		log:   log,
		mocks: append([]config.TCPMock(nil), cfg.Mocks...),
	}
}

func (s *Server) SetMocks(mocks []config.TCPMock) {
	s.mocks = append([]config.TCPMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.TCPMock {
	return append([]config.TCPMock(nil), s.mocks...)
}

// Start listens for TCP connections. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("tcp server listen :%d: %w", s.cfg.Port, err)
	}
	s.listener = ln

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("tcp accept: %w", err)
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 65536)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		data := buf[:n]

		mock, matched := s.matchMock(data)
		if !matched {
			s.log.Log(logger.Entry{
				Protocol: "tcp",
				Method:   "RECV",
				Path:     fmt.Sprintf(":%d", s.cfg.Port),
				Status:   0,
				Body:     fmt.Sprintf("(unmatched) %s", string(data)),
			})
			return
		}

		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}

		resp := decodePayload(mock.Response)
		_, _ = conn.Write(resp)

		s.log.Log(logger.Entry{
			Protocol:  "tcp",
			Method:    "RECV",
			Path:      fmt.Sprintf(":%d", s.cfg.Port),
			Status:    0,
			Body:      string(data),
			MatchedID: mock.ID,
		})

		if mock.Close {
			return
		}
	}
}

func (s *Server) matchMock(data []byte) (config.TCPMock, bool) {
	text := string(data)
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if matchTCPPattern(m.Match, text, data) {
			return m, true
		}
	}
	return config.TCPMock{}, false
}

// matchTCPPattern supports: "re:…" regex, "hex:…" hex prefix, or exact string.
func matchTCPPattern(pattern, text string, raw []byte) bool {
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(text)
	}
	if strings.HasPrefix(pattern, "hex:") {
		want, err := hex.DecodeString(strings.ReplaceAll(pattern[4:], " ", ""))
		if err != nil {
			return false
		}
		if len(raw) < len(want) {
			return false
		}
		return string(raw[:len(want)]) == string(want)
	}
	return text == pattern
}

// decodePayload converts "hex:…" to bytes, otherwise returns raw UTF-8.
func decodePayload(s string) []byte {
	if strings.HasPrefix(s, "hex:") {
		b, err := hex.DecodeString(strings.ReplaceAll(s[4:], " ", ""))
		if err == nil {
			return b
		}
	}
	return []byte(s)
}

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	return map[string]interface{}{
		"protocol": "tcp",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
	}
}
