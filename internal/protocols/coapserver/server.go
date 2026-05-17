package coapserver

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

type Server struct {
	cfg   *config.CoAPConfig
	store *state.Store
	log   *logger.Logger

	mu    sync.RWMutex
	mocks []config.CoAPMock
	conn  net.PacketConn
}

func New(cfg *config.CoAPConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, log: log, mocks: append([]config.CoAPMock(nil), cfg.Mocks...)}
}

func (s *Server) SetMocks(mocks []config.CoAPMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.CoAPMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.CoAPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.CoAPMock(nil), s.mocks...)
}

func (s *Server) Start(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("coap server listen :%d: %w", s.cfg.Port, err)
	}
	s.conn = conn
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	buf := make([]byte, 1500)
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
	method, path, msgID, token, _ := parseCoAPPacket(data)
	if method == "" {
		return
	}
	mock, ok := s.matchMock(method, path)
	resp := config.CoAPResponse{Code: "4.04"}
	matchedID := ""
	if ok {
		resp = mock.Response
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}
		matchedID = mock.ID
	}
	packet := buildCoAPResponse(msgID, token, resp)
	_, _ = conn.WriteTo(packet, addr)
	s.log.Log(logger.Entry{Protocol: "coap", Method: method, Path: path, Status: 0, Body: resp.Payload, MatchedID: matchedID})
}

func parseCoAPPacket(data []byte) (string, string, uint16, []byte, []byte) {
	if len(data) < 4 {
		return "", "", 0, nil, nil
	}
	if data[0]>>6 != 1 {
		return "", "", 0, nil, nil
	}
	tkl := int(data[0] & 0x0f)
	if len(data) < 4+tkl {
		return "", "", 0, nil, nil
	}
	code := data[1]
	msgID := binary.BigEndian.Uint16(data[2:4])
	token := append([]byte(nil), data[4:4+tkl]...)
	idx := 4 + tkl
	lastOpt := 0
	var pathParts []string
	for idx < len(data) {
		if data[idx] == 0xff {
			idx++
			break
		}
		head := data[idx]
		idx++
		delta := int(head >> 4)
		length := int(head & 0x0f)
		if idx+length > len(data) {
			return coapMethod(code), strings.Join(pathParts, "/"), msgID, token, nil
		}
		optNum := lastOpt + delta
		val := data[idx : idx+length]
		idx += length
		lastOpt = optNum
		if optNum == 11 {
			pathParts = append(pathParts, string(val))
		}
	}
	payload := []byte(nil)
	if idx <= len(data) {
		payload = append(payload, data[idx:]...)
	}
	return coapMethod(code), "/" + strings.Join(pathParts, "/"), msgID, token, payload
}

func coapMethod(code byte) string {
	switch code {
	case 0x01:
		return "GET"
	case 0x02:
		return "POST"
	case 0x03:
		return "PUT"
	case 0x04:
		return "DELETE"
	default:
		return ""
	}
}

func buildCoAPResponse(msgID uint16, token []byte, resp config.CoAPResponse) []byte {
	out := []byte{0x60 | byte(len(token)), coapResponseCode(resp.Code), 0, 0}
	binary.BigEndian.PutUint16(out[2:4], msgID)
	out = append(out, token...)
	if resp.ContentFormat != 0 {
		out = append(out, byte((12<<4)|1), byte(resp.ContentFormat))
	}
	if resp.Payload != "" {
		out = append(out, 0xff)
		out = append(out, []byte(resp.Payload)...)
	}
	return out
}

func coapResponseCode(code string) byte {
	switch code {
	case "2.01":
		return 0x41
	case "2.04":
		return 0x44
	case "2.05":
		return 0x45
	case "4.00":
		return 0x80
	case "4.04":
		return 0x84
	case "5.00":
		return 0xA0
	default:
		return 0x84
	}
}

func (s *Server) matchMock(method, path string) (config.CoAPMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if !strings.EqualFold(m.Method, method) || !matchCoAPPath(m.Path, path) {
			continue
		}
		return m, true
	}
	return config.CoAPMock{}, false
}

func matchCoAPPath(pattern, path string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(path)
	}
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		return strings.HasPrefix(path, parts[0]) && (parts[1] == "" || strings.HasSuffix(path, parts[1]))
	}
	return pattern == path
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "coap", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks)}
}
