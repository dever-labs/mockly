package memcachedserver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

type Server struct {
	cfg       *config.MemcachedConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger

	mu       sync.RWMutex
	mocks    []config.MemcachedMock
	listener net.Listener
}

func New(cfg *config.MemcachedConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, scenarios: sc, log: log, mocks: append([]config.MemcachedMock(nil), cfg.Mocks...)}
}

func (s *Server) SetMocks(mocks []config.MemcachedMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.MemcachedMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.MemcachedMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.MemcachedMock(nil), s.mocks...)
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("memcached server listen :%d: %w", s.cfg.Port, err)
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
				return err
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close() //nolint:errcheck
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := strings.ToLower(parts[0])
		switch cmd {
		case "quit":
			return
		case "version":
			_, _ = conn.Write([]byte("VERSION 1.6.0\r\n"))
		case "stats":
			_, _ = conn.Write([]byte("STAT pid 1\r\nSTAT version 1.6.0\r\nEND\r\n"))
		case "flush_all":
			_, _ = conn.Write([]byte("OK\r\n"))
		case "get":
			s.handleGet(conn, parts[1:])
		case "set", "add", "replace":
			s.handleStore(conn, reader, cmd, parts)
		case "delete", "incr", "decr":
			s.handleSimpleMutation(conn, cmd, parts)
		default:
			_, _ = conn.Write([]byte("ERROR\r\n"))
		}
	}
}

func (s *Server) handleGet(conn net.Conn, keys []string) {
	if s.maybeInjectFault(conn) {
		return
	}
	var buf strings.Builder
	for _, key := range keys {
		mock, ok := s.matchMock("get", key)
		if !ok || mock.Response.Value == "" {
			continue
		}
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}
		value := mock.Response.Value
		fmt.Fprintf(&buf, "VALUE %s %d %d\r\n%s\r\n", key, mock.Response.Flags, len(value), value)
	}
	buf.WriteString("END\r\n")
	_, _ = conn.Write([]byte(buf.String()))
}

func (s *Server) handleStore(conn net.Conn, reader *bufio.Reader, command string, parts []string) {
	if s.maybeInjectFault(conn) {
		return
	}
	if len(parts) < 5 {
		_, _ = conn.Write([]byte("ERROR\r\n"))
		return
	}
	key := parts[1]
	noreply := len(parts) > 5 && strings.EqualFold(parts[5], "noreply")
	bytesLen, _ := strconv.Atoi(parts[4])
	if bytesLen < 0 || bytesLen > 1024*1024 {
		_, _ = conn.Write([]byte("CLIENT_ERROR bad data chunk\r\n"))
		return
	}
	buf := make([]byte, bytesLen+2)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return
	}
	mock, ok := s.matchMock(command, key)
	status := "STORED"
	if ok && mock.Response.Status != "" {
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}
		status = mock.Response.Status
	}
	if !noreply {
		_, _ = conn.Write([]byte(status + "\r\n"))
	}
}

func (s *Server) handleSimpleMutation(conn net.Conn, command string, parts []string) {
	if s.maybeInjectFault(conn) {
		return
	}
	if len(parts) < 2 {
		_, _ = conn.Write([]byte("ERROR\r\n"))
		return
	}
	key := parts[1]
	noreply := len(parts) > 2 && strings.EqualFold(parts[len(parts)-1], "noreply")
	mock, ok := s.matchMock(command, key)
	status := map[string]string{"delete": "DELETED", "incr": "NOT_FOUND", "decr": "NOT_FOUND"}[command]
	if ok && mock.Response.Status != "" {
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}
		status = mock.Response.Status
	}
	if !noreply {
		_, _ = conn.Write([]byte(status + "\r\n"))
	}
}

func (s *Server) maybeInjectFault(conn net.Conn) bool {
	fault := s.scenarios.EffectiveMemcachedFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}
	if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
		errType := fault.ErrorType
		if errType == "" {
			errType = "SERVER_ERROR"
		}
		msg := fault.Message
		if msg == "" {
			msg = "fault injected"
		}
		_, _ = conn.Write([]byte(fmt.Sprintf("%s %s\r\n", errType, msg)))
		return true
	}
	return false
}

func (s *Server) matchMock(command, key string) (config.MemcachedMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if m.Command != "*" && !strings.EqualFold(m.Command, command) {
			continue
		}
		if m.Key != "" && !matchMemcachedKey(m.Key, key) {
			continue
		}
		return m, true
	}
	return config.MemcachedMock{}, false
}

func matchMemcachedKey(pattern, key string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(key)
	}
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		return strings.HasPrefix(key, parts[0]) && (parts[1] == "" || strings.HasSuffix(key, parts[1]))
	}
	return pattern == key
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "memcached", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks)}
}
