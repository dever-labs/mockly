package amqpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

type MessageStore struct {
	mu       sync.RWMutex
	messages []config.ReceivedAMQPMessage
	maxSize  int
}

func newMessageStore(maxSize int) *MessageStore {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MessageStore{maxSize: maxSize}
}

func (m *MessageStore) Add(msg config.ReceivedAMQPMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) >= m.maxSize {
		m.messages = m.messages[1:]
	}
	m.messages = append(m.messages, msg)
}

func (m *MessageStore) All() []config.ReceivedAMQPMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]config.ReceivedAMQPMessage(nil), m.messages...)
}

func (m *MessageStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

type publishState struct {
	exchange   string
	routingKey string
	bodySize   uint64
	body       bytes.Buffer
}

type consumer struct {
	channel uint16
	tag     string
	queue   string
}

type connState struct {
	conn      net.Conn
	consumers map[uint16]*consumer
	publishes map[uint16]*publishState
}

type Server struct {
	cfg   *config.AMQPConfig
	store *state.Store
	log   *logger.Logger

	mu       sync.RWMutex
	mocks    []config.AMQPMock
	messages *MessageStore
	listener net.Listener
	delivery uint64
}

func New(cfg *config.AMQPConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, log: log, mocks: append([]config.AMQPMock(nil), cfg.Mocks...), messages: newMessageStore(1000)}
}

func (s *Server) SetMocks(mocks []config.AMQPMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.AMQPMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.AMQPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.AMQPMock(nil), s.mocks...)
}

func (s *Server) GetMessageStore() *MessageStore { return s.messages }

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("amqp server listen :%d: %w", s.cfg.Port, err)
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
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil || string(header) != "AMQP\x00\x00\x09\x01" {
		return
	}
	_ = writeMethodFrame(conn, 0, 10, 10, buildConnectionStart())
	state := &connState{conn: conn, consumers: map[uint16]*consumer{}, publishes: map[uint16]*publishState{}}
	for {
		ft, ch, payload, err := readFrame(reader)
		if err != nil {
			return
		}
		switch ft {
		case 1:
			classID := binary.BigEndian.Uint16(payload[0:2])
			methodID := binary.BigEndian.Uint16(payload[2:4])
			args := payload[4:]
			if !s.handleMethod(state, ch, classID, methodID, args) {
				return
			}
		case 2:
			if pub := state.publishes[ch]; pub != nil && len(payload) >= 12 {
				pub.bodySize = binary.BigEndian.Uint64(payload[4:12])
			}
		case 3:
			if pub := state.publishes[ch]; pub != nil {
				pub.body.Write(payload)
				if uint64(pub.body.Len()) >= pub.bodySize {
					s.processPublish(state, ch, pub)
					delete(state.publishes, ch)
				}
			}
		case 8:
			continue
		}
	}
}

func (s *Server) handleMethod(state *connState, channel, classID, methodID uint16, args []byte) bool {
	switch {
	case classID == 10 && methodID == 11:
		_ = writeMethodFrame(state.conn, 0, 10, 30, buildConnectionTune())
	case classID == 10 && methodID == 31:
		return true
	case classID == 10 && methodID == 40:
		_ = writeMethodFrame(state.conn, 0, 10, 41, nil)
	case classID == 20 && methodID == 10:
		_ = writeMethodFrame(state.conn, channel, 20, 11, []byte{0x00, 0x00, 0x00, 0x00})
	case classID == 40 && methodID == 10:
		_ = writeMethodFrame(state.conn, channel, 40, 11, nil)
	case classID == 50 && methodID == 10:
		_ = writeMethodFrame(state.conn, channel, 50, 11, buildQueueDeclareOK(args))
	case classID == 50 && methodID == 20:
		_ = writeMethodFrame(state.conn, channel, 50, 21, nil)
	case classID == 60 && methodID == 20:
		queue, consumerTag := parseConsume(args)
		if consumerTag == "" {
			consumerTag = fmt.Sprintf("ctag-%d", time.Now().UnixNano())
		}
		state.consumers[channel] = &consumer{channel: channel, queue: queue, tag: consumerTag}
		_ = writeMethodFrame(state.conn, channel, 60, 21, encodeShortStr(consumerTag))
	case classID == 60 && methodID == 40:
		exchange, routingKey := parsePublish(args)
		state.publishes[channel] = &publishState{exchange: exchange, routingKey: routingKey}
	case classID == 20 && methodID == 40:
		_ = writeMethodFrame(state.conn, channel, 20, 41, nil)
	case classID == 10 && methodID == 50:
		_ = writeMethodFrame(state.conn, 0, 10, 51, nil)
		return false
	}
	return true
}

func (s *Server) processPublish(state *connState, channel uint16, pub *publishState) {
	body := pub.body.String()
	s.messages.Add(config.ReceivedAMQPMessage{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Exchange: pub.exchange, RoutingKey: pub.routingKey, Body: body, Timestamp: time.Now().UTC().Format(time.RFC3339)})
	mock, ok := s.matchMock(pub.exchange, pub.routingKey)
	matchedID := ""
	if ok {
		matchedID = mock.ID
	}
	s.log.Log(logger.Entry{Protocol: "amqp", Method: "PUBLISH", Path: pub.routingKey, Status: 0, Body: body, MatchedID: matchedID})
	if !ok || mock.Response == nil {
		return
	}
	if mock.Delay.Duration > 0 {
		time.Sleep(mock.Delay.Duration)
	}
	cons := state.consumers[channel]
	if cons == nil {
		for _, c := range state.consumers {
			cons = c
			break
		}
	}
	if cons == nil {
		return
	}
	respExchange := mock.Response.Exchange
	respRoutingKey := mock.Response.RoutingKey
	if respRoutingKey == "" {
		respRoutingKey = pub.routingKey
	}
	deliveryTag := atomic.AddUint64(&s.delivery, 1)
	args := append(encodeShortStr(cons.tag), encodeLongLong(deliveryTag)...)
	args = append(args, 0)
	args = append(args, encodeShortStr(respExchange)...)
	args = append(args, encodeShortStr(respRoutingKey)...)
	_ = writeMethodFrame(state.conn, cons.channel, 60, 60, args)
	_ = writeHeaderFrame(state.conn, cons.channel, 60, uint64(len(mock.Response.Body)))
	_ = writeBodyFrame(state.conn, cons.channel, []byte(mock.Response.Body))
}

func (s *Server) matchMock(exchange, routingKey string) (config.AMQPMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if m.Exchange != exchange {
			continue
		}
		if !matchAMQPRoutingKey(m.RoutingKey, routingKey) {
			continue
		}
		return m, true
	}
	return config.AMQPMock{}, false
}

func matchAMQPRoutingKey(pattern, key string) bool {
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

func readFrame(r io.Reader) (byte, uint16, []byte, error) {
	head := make([]byte, 7)
	if _, err := io.ReadFull(r, head); err != nil {
		return 0, 0, nil, err
	}
	frameType := head[0]
	channel := binary.BigEndian.Uint16(head[1:3])
	size := binary.BigEndian.Uint32(head[3:7])
	if size > 10*1024*1024 {
		return 0, 0, nil, fmt.Errorf("amqp: frame too large: %d bytes", size)
	}
	payload := make([]byte, size+1)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, 0, nil, err
	}
	if payload[len(payload)-1] != 0xce {
		return 0, 0, nil, fmt.Errorf("invalid frame end")
	}
	return frameType, channel, payload[:len(payload)-1], nil
}

func writeFrame(w io.Writer, frameType byte, channel uint16, payload []byte) error {
	head := []byte{frameType, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(head[1:3], channel)
	binary.BigEndian.PutUint32(head[3:7], uint32(len(payload)))
	if _, err := w.Write(head); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	_, err := w.Write([]byte{0xce})
	return err
}

func writeMethodFrame(w io.Writer, channel, classID, methodID uint16, args []byte) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], classID)
	binary.BigEndian.PutUint16(payload[2:4], methodID)
	payload = append(payload, args...)
	return writeFrame(w, 1, channel, payload)
}

func writeHeaderFrame(w io.Writer, channel, classID uint16, bodySize uint64) error {
	payload := make([]byte, 14)
	binary.BigEndian.PutUint16(payload[0:2], classID)
	binary.BigEndian.PutUint16(payload[2:4], 0)
	binary.BigEndian.PutUint64(payload[4:12], bodySize)
	binary.BigEndian.PutUint16(payload[12:14], 0)
	return writeFrame(w, 2, channel, payload)
}

func writeBodyFrame(w io.Writer, channel uint16, body []byte) error {
	return writeFrame(w, 3, channel, body)
}

func buildConnectionStart() []byte {
	payload := []byte{0, 9}
	payload = append(payload, 0, 0, 0, 0)
	payload = append(payload, encodeLongStr("PLAIN")...)
	payload = append(payload, encodeLongStr("en_US")...)
	return payload
}

func buildConnectionTune() []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint16(payload[0:2], 0)
	binary.BigEndian.PutUint32(payload[2:6], 131072)
	binary.BigEndian.PutUint16(payload[6:8], 0)
	return payload
}

func buildQueueDeclareOK(args []byte) []byte {
	buf := bytes.NewBuffer(args)
	_ = readShort(buf)
	queue := readShortStr(buf)
	payload := encodeShortStr(queue)
	payload = append(payload, 0, 0, 0, 0, 0, 0, 0, 0)
	return payload
}

func parseConsume(args []byte) (string, string) {
	buf := bytes.NewBuffer(args)
	_ = readShort(buf)
	queue := readShortStr(buf)
	tag := readShortStr(buf)
	return queue, tag
}

func parsePublish(args []byte) (string, string) {
	buf := bytes.NewBuffer(args)
	_ = readShort(buf)
	exchange := readShortStr(buf)
	routingKey := readShortStr(buf)
	return exchange, routingKey
}

func readShort(r *bytes.Buffer) uint16 {
	var v uint16
	_ = binary.Read(r, binary.BigEndian, &v)
	return v
}

func readShortStr(r *bytes.Buffer) string {
	if r.Len() == 0 {
		return ""
	}
	n, _ := r.ReadByte()
	buf := make([]byte, int(n))
	_, _ = io.ReadFull(r, buf)
	return string(buf)
}

func encodeShortStr(s string) []byte {
	if len(s) > 255 {
		s = s[:255]
	}
	return append([]byte{byte(len(s))}, []byte(s)...)
}

func encodeLongStr(s string) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(s)))
	return append(buf, []byte(s)...)
}

func encodeLongLong(v uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	return buf
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "amqp", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks), "messages": len(s.messages.All())}
}
