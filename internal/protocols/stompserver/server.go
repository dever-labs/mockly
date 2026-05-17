package stompserver

import (
	"bufio"
	"bytes"
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
	"github.com/dever-labs/mockly/internal/state"
)

type MessageStore struct {
	mu       sync.RWMutex
	messages []config.ReceivedSTOMPMessage
	maxSize  int
}

func newMessageStore(maxSize int) *MessageStore {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MessageStore{maxSize: maxSize}
}

func (m *MessageStore) Add(msg config.ReceivedSTOMPMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) >= m.maxSize {
		m.messages = m.messages[1:]
	}
	m.messages = append(m.messages, msg)
}

func (m *MessageStore) All() []config.ReceivedSTOMPMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]config.ReceivedSTOMPMessage(nil), m.messages...)
}

func (m *MessageStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

type clientConn struct {
	id   string
	conn net.Conn
	mu   sync.Mutex
	subs map[string]string
}

type subscription struct {
	id     string
	client *clientConn
}

type Server struct {
	cfg   *config.STOMPConfig
	store *state.Store
	log   *logger.Logger

	mu       sync.RWMutex
	mocks    []config.STOMPMock
	messages *MessageStore

	listener      net.Listener
	subscriptions map[string]map[string]*subscription
}

func New(cfg *config.STOMPConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, log: log, mocks: append([]config.STOMPMock(nil), cfg.Mocks...), messages: newMessageStore(1000), subscriptions: make(map[string]map[string]*subscription)}
}

func (s *Server) SetMocks(mocks []config.STOMPMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.STOMPMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.STOMPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.STOMPMock(nil), s.mocks...)
}

func (s *Server) GetMessageStore() *MessageStore { return s.messages }

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("stomp server listen :%d: %w", s.cfg.Port, err)
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
		client := &clientConn{id: fmt.Sprintf("%d", time.Now().UnixNano()), conn: conn, subs: map[string]string{}}
		go s.handleConn(client)
	}
}

func (s *Server) handleConn(client *clientConn) {
	defer func() {
		s.removeClient(client)
		_ = client.conn.Close()
	}()
	reader := bufio.NewReader(client.conn)
	for {
		cmd, headers, body, err := readSTOMPFrame(reader)
		if err != nil {
			return
		}
		if cmd == "" {
			continue
		}
		switch cmd {
		case "CONNECT", "STOMP":
			s.writeFrame(client, "CONNECTED", map[string]string{"version": "1.2", "heart-beat": "0,0"}, "")
		case "SUBSCRIBE":
			id := headers["id"]
			dest := headers["destination"]
			s.addSubscription(client, id, dest)
		case "UNSUBSCRIBE":
			s.removeSubscription(client, headers["id"])
		case "SEND":
			dest := headers["destination"]
			s.messages.Add(config.ReceivedSTOMPMessage{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Destination: dest, Body: body, Headers: headers, Timestamp: time.Now().UTC().Format(time.RFC3339)})
			mock, ok := s.matchMock(dest)
			if ok && mock.Response != nil {
				if mock.Delay.Duration > 0 {
					time.Sleep(mock.Delay.Duration)
				}
				responseDest := mock.Response.Destination
				if responseDest == "" {
					responseDest = dest
				}
				s.deliver(responseDest, mock.Response)
			}
		case "ACK", "NACK":
			if receipt := headers["receipt"]; receipt != "" {
				s.writeFrame(client, "RECEIPT", map[string]string{"receipt-id": receipt}, "")
			}
		case "DISCONNECT":
			if receipt := headers["receipt"]; receipt != "" {
				s.writeFrame(client, "RECEIPT", map[string]string{"receipt-id": receipt}, "")
			}
			return
		}
	}
}

func readSTOMPFrame(reader *bufio.Reader) (string, map[string]string, string, error) {
	var raw bytes.Buffer
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", nil, "", err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" && raw.Len() == 0 {
			continue
		}
		raw.WriteString(line)
		if trimmed == "" {
			break
		}
	}
	headersText := strings.ReplaceAll(raw.String(), "\r\n", "\n")
	lines := strings.Split(headersText, "\n")
	cmd := strings.TrimSpace(lines[0])
	headers := map[string]string{}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		headers[line[:idx]] = line[idx+1:]
	}
	const maxBody = 10 * 1024 * 1024
	if clStr := strings.TrimSpace(headers["content-length"]); clStr != "" {
		cl, cerr := strconv.Atoi(clStr)
		if cerr != nil || cl < 0 || cl > maxBody {
			return "", nil, "", fmt.Errorf("stomp: invalid content-length %q", clStr)
		}
		buf := make([]byte, cl+1)
		if _, rerr := io.ReadFull(reader, buf); rerr != nil {
			return "", nil, "", rerr
		}
		if buf[cl] != 0 {
			return "", nil, "", fmt.Errorf("stomp: missing null terminator")
		}
		return cmd, headers, string(buf[:cl]), nil
	}
	body, err := reader.ReadBytes(0)
	if err != nil {
		return "", nil, "", err
	}
	if len(body) > maxBody+1 {
		return "", nil, "", fmt.Errorf("stomp: body too large")
	}
	return cmd, headers, string(bytes.TrimSuffix(body, []byte{0})), nil
}

func (s *Server) writeFrame(client *clientConn, cmd string, headers map[string]string, body string) {
	var b strings.Builder
	b.WriteString(cmd)
	b.WriteByte('\n')
	for k, v := range headers {
		fmt.Fprintf(&b, "%s:%s\n", k, v)
	}
	b.WriteByte('\n')
	b.WriteString(body)
	b.WriteByte(0)
	client.mu.Lock()
	defer client.mu.Unlock()
	_, _ = client.conn.Write([]byte(b.String()))
}

func (s *Server) addSubscription(client *clientConn, id, dest string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	client.subs[id] = dest
	if s.subscriptions[dest] == nil {
		s.subscriptions[dest] = map[string]*subscription{}
	}
	s.subscriptions[dest][client.id+":"+id] = &subscription{id: id, client: client}
}

func (s *Server) removeSubscription(client *clientConn, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dest := client.subs[id]
	delete(client.subs, id)
	if s.subscriptions[dest] != nil {
		delete(s.subscriptions[dest], client.id+":"+id)
		if len(s.subscriptions[dest]) == 0 {
			delete(s.subscriptions, dest)
		}
	}
}

func (s *Server) removeClient(client *clientConn) {
	for id := range client.subs {
		s.removeSubscription(client, id)
	}
}

func (s *Server) deliver(dest string, resp *config.STOMPResponse) {
	s.mu.RLock()
	subs := make([]*subscription, 0, len(s.subscriptions[dest]))
	for _, sub := range s.subscriptions[dest] {
		subs = append(subs, sub)
	}
	s.mu.RUnlock()
	for _, sub := range subs {
		headers := map[string]string{"subscription": sub.id, "message-id": fmt.Sprintf("%d", time.Now().UnixNano()), "destination": dest}
		if resp.ContentType != "" {
			headers["content-type"] = resp.ContentType
		}
		for k, v := range resp.Headers {
			headers[k] = v
		}
		s.writeFrame(sub.client, "MESSAGE", headers, resp.Body)
	}
}

func (s *Server) matchMock(dest string) (config.STOMPMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if matchDestination(m.Destination, dest) {
			return m, true
		}
	}
	return config.STOMPMock{}, false
}

func matchDestination(pattern, dest string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(dest)
	}
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		return strings.HasPrefix(dest, parts[0]) && (parts[1] == "" || strings.HasSuffix(dest, parts[1]))
	}
	return pattern == dest
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "stomp", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks), "messages": len(s.messages.All())}
}
