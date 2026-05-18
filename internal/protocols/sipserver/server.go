package sipserver

import (
	"bytes"
	"context"
	"fmt"
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
	cfg       *config.SIPConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger

	mu    sync.RWMutex
	mocks []config.SIPMock
	conn  net.PacketConn
}

func New(cfg *config.SIPConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, scenarios: sc, log: log, mocks: append([]config.SIPMock(nil), cfg.Mocks...)}
}

func (s *Server) SetMocks(mocks []config.SIPMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.SIPMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.SIPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.SIPMock(nil), s.mocks...)
}

func (s *Server) Start(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("sip server listen :%d: %w", s.cfg.Port, err)
	}
	s.conn = conn
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	buf := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		s.handlePacket(conn, addr, append([]byte(nil), buf[:n]...))
	}
}

func (s *Server) handlePacket(conn net.PacketConn, addr net.Addr, data []byte) {
	method, uri, headers, body := parseSIPMessage(data)
	fault := s.scenarios.EffectiveSIPFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}
	if method == "" {
		return
	}
	s.log.Log(logger.Entry{Protocol: "sip", Method: method, Path: uri, Status: 0, Body: body})
	if method == "ACK" {
		return
	}
	if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
		status := fault.Status
		if status == 0 {
			status = 503
		}
		reason := fault.Reason
		if reason == "" {
			reason = defaultSIPReason(status)
		}
		resp := config.SIPResponse{Status: status, Reason: reason}
		message := buildSIPResponse(resp, headers)
		_, _ = conn.WriteTo(message, addr)
		return
	}
	mock, ok := s.matchMock(method, uri)
	resp := config.SIPResponse{Status: 200, Reason: defaultSIPReason(200)}
	matchedID := ""
	if ok {
		resp = mock.Response
		if resp.Status == 0 {
			resp.Status = 200
		}
		if resp.Reason == "" {
			resp.Reason = defaultSIPReason(resp.Status)
		}
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}
		matchedID = mock.ID
	}
	message := buildSIPResponse(resp, headers)
	_, _ = conn.WriteTo(message, addr)
	s.log.Log(logger.Entry{Protocol: "sip", Method: method, Path: uri, Status: resp.Status, Body: body, MatchedID: matchedID})
}

func parseSIPMessage(data []byte) (string, string, map[string]string, string) {
	parts := bytes.SplitN(data, []byte("\r\n\r\n"), 2)
	headersPart := string(parts[0])
	body := ""
	if len(parts) > 1 {
		body = string(parts[1])
	}
	lines := strings.Split(headersPart, "\r\n")
	if len(lines) == 0 {
		return "", "", nil, ""
	}
	first := strings.Fields(lines[0])
	if len(first) < 2 {
		return "", "", nil, ""
	}
	headers := map[string]string{}
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		headers[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	if cl, err := strconv.Atoi(headers["Content-Length"]); err == nil && cl >= 0 && cl <= len(body) {
		body = body[:cl]
	}
	return strings.ToUpper(first[0]), first[1], headers, body
}

func buildSIPResponse(resp config.SIPResponse, reqHeaders map[string]string) []byte {
	body := resp.Body
	var b strings.Builder
	fmt.Fprintf(&b, "SIP/2.0 %d %s\r\n", resp.Status, resp.Reason)
	for _, key := range []string{"Via", "From", "To", "Call-ID", "CSeq"} {
		if value := reqHeaders[key]; value != "" {
			fmt.Fprintf(&b, "%s: %s\r\n", key, value)
		}
	}
	for k, v := range resp.Headers {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return []byte(b.String())
}

func (s *Server) matchMock(method, uri string) (config.SIPMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if m.Method != "*" && !strings.EqualFold(m.Method, method) {
			continue
		}
		if m.URI != "" && !matchSIPURI(m.URI, uri) {
			continue
		}
		return m, true
	}
	return config.SIPMock{}, false
}

func matchSIPURI(pattern, uri string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(uri)
	}
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		return strings.HasPrefix(uri, parts[0]) && (parts[1] == "" || strings.HasSuffix(uri, parts[1]))
	}
	return pattern == uri
}

func defaultSIPReason(status int) string {
	switch status {
	case 100:
		return "Trying"
	case 180:
		return "Ringing"
	case 200:
		return "OK"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 486:
		return "Busy Here"
	case 487:
		return "Request Terminated"
	case 500:
		return "Server Internal Error"
	case 503:
		return "Service Unavailable"
	default:
		return "OK"
	}
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "sip", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks)}
}
