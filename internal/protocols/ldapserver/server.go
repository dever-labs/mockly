package ldapserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

type Server struct {
	cfg   *config.LDAPConfig
	store *state.Store
	log   *logger.Logger

	mu       sync.RWMutex
	mocks    []config.LDAPMock
	listener net.Listener
}

func New(cfg *config.LDAPConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, log: log, mocks: append([]config.LDAPMock(nil), cfg.Mocks...)}
}

func (s *Server) SetMocks(mocks []config.LDAPMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.LDAPMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.LDAPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.LDAPMock(nil), s.mocks...)
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("ldap server listen :%d: %w", s.cfg.Port, err)
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
	for {
		packet, err := readBERPacket(conn)
		if err != nil {
			return
		}
		msgID, opTag, opContent := parseLDAPEnvelope(packet)
		switch opTag {
		case 0x60:
			_, _ = conn.Write(buildLDAPMessage(msgID, 0x61, bindSuccessContent()))
		case 0x42:
			return
		case 0x63:
			baseDN, filterRaw := parseSearchRequest(opContent)
			for _, mock := range s.matchMocks(baseDN, filterRaw) {
				if mock.Delay.Duration > 0 {
					time.Sleep(mock.Delay.Duration)
				}
				_, _ = conn.Write(buildLDAPMessage(msgID, 0x64, searchEntryContent(baseDN, mock.Attributes)))
				s.log.Log(logger.Entry{Protocol: "ldap", Method: "SEARCH", Path: baseDN, Status: 0, Body: filterRaw, MatchedID: mock.ID})
			}
			_, _ = conn.Write(buildLDAPMessage(msgID, 0x65, searchDoneContent()))
		case 0x70:
			continue
		}
	}
}

func readBERPacket(conn net.Conn) ([]byte, error) {
	head := make([]byte, 2)
	if _, err := conn.Read(head); err != nil {
		return nil, err
	}
	extra := 0
	length := int(head[1])
	if head[1]&0x80 != 0 {
		extra = int(head[1] & 0x7f)
		buf := make([]byte, extra)
		if _, err := conn.Read(buf); err != nil {
			return nil, err
		}
		length = 0
		for _, b := range buf {
			length = (length << 8) | int(b)
		}
		head = append(head, buf...)
	}
	if length > 10*1024*1024 {
		return nil, fmt.Errorf("ldap: packet too large: %d bytes", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	return append(head, body...), nil
}

func parseLDAPEnvelope(packet []byte) (int, byte, []byte) {
	_, hdr := tlvLength(packet[1:])
	content := packet[1+hdr:]
	msgIDTag, msgIDContent, next := readTLV(content)
	if msgIDTag != 0x02 {
		return 0, 0, nil
	}
	msgID := 0
	for _, b := range msgIDContent {
		msgID = (msgID << 8) | int(b)
	}
	opTag, opContent, _ := readTLV(content[next:])
	return msgID, opTag, opContent
}

func parseSearchRequest(content []byte) (string, string) {
	_, baseContent, next := readTLV(content)
	baseDN := string(baseContent)
	rest := content[next:]
	for i := 0; i < 5; i++ {
		_, _, consumed := readTLV(rest)
		rest = rest[consumed:]
	}
	filterTag, filterContent, _ := readTLV(rest)
	encoded := append([]byte{filterTag}, berLen(len(filterContent))...)
	encoded = append(encoded, filterContent...)
	return baseDN, string(encoded)
}

func (s *Server) matchMocks(baseDN, filterRaw string) []config.LDAPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := make([]config.LDAPMock, 0)
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if !strings.Contains(strings.ToLower(baseDN), strings.ToLower(m.BaseDN)) {
			continue
		}
		if m.Filter != "" && m.Filter != filterRaw {
			continue
		}
		matched = append(matched, m)
	}
	return matched
}

func buildLDAPMessage(msgID int, tag byte, content []byte) []byte {
	body := append(berTLV(0x02, encodeInt(msgID)), berTLV(tag, content)...)
	return berTLV(0x30, body)
}

func bindSuccessContent() []byte {
	return append(append(berTLV(0x0a, []byte{0x00}), berTLV(0x04, nil)...), berTLV(0x04, nil)...)
}

func searchDoneContent() []byte {
	return bindSuccessContent()
}

func searchEntryContent(baseDN string, attrs map[string][]string) []byte {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	attrSeq := make([]byte, 0)
	for _, key := range keys {
		vals := make([]byte, 0)
		for _, v := range attrs[key] {
			vals = append(vals, berTLV(0x04, []byte(v))...)
		}
		attr := append(berTLV(0x04, []byte(key)), berTLV(0x31, vals)...)
		attrSeq = append(attrSeq, berTLV(0x30, attr)...)
	}
	return append(berTLV(0x04, []byte(baseDN)), berTLV(0x30, attrSeq)...)
}

func berTLV(tag byte, content []byte) []byte {
	out := []byte{tag}
	out = append(out, berLen(len(content))...)
	out = append(out, content...)
	return out
}

func berLen(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n < 0x100 {
		return []byte{0x81, byte(n)}
	}
	return []byte{0x82, byte(n >> 8), byte(n)}
}

func tlvLength(b []byte) (int, int) {
	if len(b) == 0 {
		return 0, 0
	}
	if b[0]&0x80 == 0 {
		return int(b[0]), 1
	}
	n := int(b[0] & 0x7f)
	length := 0
	for i := 0; i < n && i+1 < len(b); i++ {
		length = (length << 8) | int(b[i+1])
	}
	return length, 1 + n
}

func readTLV(b []byte) (byte, []byte, int) {
	if len(b) < 2 {
		return 0, nil, len(b)
	}
	length, hdr := tlvLength(b[1:])
	start := 1 + hdr
	end := start + length
	if end > len(b) {
		end = len(b)
	}
	return b[0], b[start:end], end
}

func encodeInt(v int) []byte {
	if v == 0 {
		return []byte{0}
	}
	out := make([]byte, 0)
	for v > 0 {
		out = append([]byte{byte(v)}, out...)
		v >>= 8
	}
	return out
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "ldap", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks)}
}
