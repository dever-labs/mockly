// Package redisserver implements a Redis-compatible mock server using the RESP protocol.
package redisserver

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/redcon"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

// Server is the Redis mock server.
type Server struct {
	cfg   *config.RedisConfig
	store *state.Store
	log   *logger.Logger
	mocks []config.RedisMock
}

// New creates a Server.
func New(cfg *config.RedisConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:   cfg,
		store: store,
		log:   log,
		mocks: append([]config.RedisMock(nil), cfg.Mocks...),
	}
}

func (s *Server) SetMocks(mocks []config.RedisMock) {
	s.mocks = append([]config.RedisMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.RedisMock {
	return append([]config.RedisMock(nil), s.mocks...)
}

// Start begins listening. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)

	srv := redcon.NewServer(addr,
		s.handleCommand,
		func(conn redcon.Conn) bool { return true },
		func(conn redcon.Conn, err error) {},
	)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		return srv.Close()
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleCommand(conn redcon.Conn, cmd redcon.Command) {
	start := time.Now()
	command := strings.ToUpper(string(cmd.Args[0]))

	// Always-handled built-ins.
	switch command {
	case "PING":
		if len(cmd.Args) > 1 {
			conn.WriteBulk(cmd.Args[1])
		} else {
			conn.WriteString("PONG")
		}
		return
	case "QUIT":
		conn.WriteString("OK")
		conn.Close()
		return
	case "SELECT", "FLUSHDB", "FLUSHALL":
		conn.WriteString("OK")
		return
	case "COMMAND":
		conn.WriteString("OK")
		return
	}

	key := ""
	if len(cmd.Args) > 1 {
		key = string(cmd.Args[1])
	}

	mock, matched := s.matchMock(command, key)
	if !matched {
		conn.WriteError(fmt.Sprintf("ERR no mock matched for %s %s", command, key))
		s.log.Log(logger.Entry{
			Protocol: "redis",
			Method:   command,
			Path:     key,
			Status:   0,
			Duration: time.Since(start).Milliseconds(),
		})
		return
	}

	if mock.Delay.Duration > 0 {
		time.Sleep(mock.Delay.Duration)
	}

	writeRedisResponse(conn, mock.Response)

	s.log.Log(logger.Entry{
		Protocol:  "redis",
		Method:    command,
		Path:      key,
		Status:    0,
		Duration:  time.Since(start).Milliseconds(),
		MatchedID: mock.ID,
	})
}

func (s *Server) matchMock(command, key string) (config.RedisMock, bool) {
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if m.Command != "*" && !strings.EqualFold(m.Command, command) {
			continue
		}
		if m.Key != "" && !matchRedisKey(m.Key, key) {
			continue
		}
		return m, true
	}
	return config.RedisMock{}, false
}

// matchRedisKey supports exact, "re:…" regex, and glob wildcards (*).
func matchRedisKey(pattern, key string) bool {
	if pattern == "*" {
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
		if !strings.HasPrefix(key, parts[0]) {
			return false
		}
		if parts[1] != "" && !strings.HasSuffix(key, parts[1]) {
			return false
		}
		return true
	}
	return pattern == key
}

// writeRedisResponse writes the RESP-typed response to the connection.
func writeRedisResponse(conn redcon.Conn, r config.RedisResponse) {
	switch strings.ToLower(r.Type) {
	case "string", "bulk":
		if r.Value == nil {
			conn.WriteNull()
			return
		}
		conn.WriteBulkString(fmt.Sprint(r.Value))
	case "integer", "int":
		switch v := r.Value.(type) {
		case int:
			conn.WriteInt(v)
		case int64:
			conn.WriteInt64(v)
		case float64:
			conn.WriteInt64(int64(v))
		default:
			conn.WriteInt(0)
		}
	case "error", "err":
		msg := "ERR"
		if r.Value != nil {
			msg = fmt.Sprint(r.Value)
		}
		conn.WriteError(msg)
	case "nil", "null":
		conn.WriteNull()
	case "array":
		items := toStringSlice(r.Value)
		conn.WriteArray(len(items))
		for _, item := range items {
			conn.WriteBulkString(item)
		}
	default:
		if r.Value == nil {
			conn.WriteNull()
		} else {
			conn.WriteBulkString(fmt.Sprint(r.Value))
		}
	}
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch arr := v.(type) {
	case []interface{}:
		out := make([]string, len(arr))
		for i, item := range arr {
			out[i] = fmt.Sprint(item)
		}
		return out
	case []string:
		return arr
	}
	return nil
}

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	return map[string]interface{}{
		"protocol": "redis",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
	}
}
