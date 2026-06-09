package kafkaserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

type MessageStore struct {
	mu       sync.RWMutex
	messages []config.ProducedKafkaMessage
	maxSize  int
}

func newMessageStore(maxSize int) *MessageStore {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MessageStore{maxSize: maxSize}
}

// NewMessageStore creates a new MessageStore with the given capacity. Exported for testing.
func NewMessageStore(maxSize int) *MessageStore { return newMessageStore(maxSize) }

func (m *MessageStore) Add(msg config.ProducedKafkaMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) >= m.maxSize {
		m.messages = m.messages[1:]
	}
	m.messages = append(m.messages, msg)
}

func (m *MessageStore) All() []config.ProducedKafkaMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]config.ProducedKafkaMessage(nil), m.messages...)
}

func (m *MessageStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

type Server struct {
	cfg       *config.KafkaConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger

	mu       sync.RWMutex
	mocks    []config.KafkaMock
	messages *MessageStore
	listener net.Listener
}

func New(cfg *config.KafkaConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, scenarios: sc, log: log, mocks: append([]config.KafkaMock(nil), cfg.Mocks...), messages: newMessageStore(1000)}
}

func (s *Server) SetMocks(mocks []config.KafkaMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.KafkaMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.KafkaMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.KafkaMock(nil), s.mocks...)
}

func (s *Server) GetMessageStore() *MessageStore { return s.messages }

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("kafka server listen :%d: %w", s.cfg.Port, err)
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
		var size int32
		if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
			return
		}
		if size < 0 || size > 10*1024*1024 {
			return
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return
		}
		resp, err := s.handleRequest(buf)
		if err != nil {
			return
		}
		_, _ = conn.Write(resp)
	}
}

func (s *Server) handleRequest(buf []byte) ([]byte, error) {
	fault := s.scenarios.EffectiveKafkaFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	r := bytes.NewReader(buf)
	var apiKey, apiVersion int16
	var correlationID int32
	_ = binary.Read(r, binary.BigEndian, &apiKey)
	_ = binary.Read(r, binary.BigEndian, &apiVersion)
	_ = binary.Read(r, binary.BigEndian, &correlationID)
	_ = apiVersion
	clientID := readKafkaString(r)
	_ = clientID
	var body []byte
	switch apiKey {
	case 18:
		body = buildApiVersionsResponse()
	case 3:
		body = s.buildMetadataResponse(r)
	case 0:
		body = s.handleProduce(r, fault)
	case 1:
		body = s.handleFetch(r, fault)
	default:
		body = []byte{}
	}
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int32(4+len(body)))
	_ = binary.Write(out, binary.BigEndian, correlationID)
	out.Write(body)
	return out.Bytes(), nil
}

func buildApiVersionsResponse() []byte {
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int16(0))
	_ = binary.Write(out, binary.BigEndian, int32(4))
	for _, key := range []int16{0, 1, 3, 18} {
		_ = binary.Write(out, binary.BigEndian, key)
		_ = binary.Write(out, binary.BigEndian, int16(0))
		_ = binary.Write(out, binary.BigEndian, int16(0))
	}
	_ = binary.Write(out, binary.BigEndian, int32(0))
	return out.Bytes()
}

func (s *Server) buildMetadataResponse(r *bytes.Reader) []byte {
	topics := readKafkaTopics(r)
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int32(1))
	_ = binary.Write(out, binary.BigEndian, int32(1))
	writeKafkaString(out, "localhost")
	_ = binary.Write(out, binary.BigEndian, int32(s.cfg.Port))
	_ = binary.Write(out, binary.BigEndian, int32(len(topics)))
	for _, topic := range topics {
		_ = binary.Write(out, binary.BigEndian, int16(0))
		writeKafkaString(out, topic)
		out.WriteByte(0)
		_ = binary.Write(out, binary.BigEndian, int32(1))
		_ = binary.Write(out, binary.BigEndian, int16(0))
		_ = binary.Write(out, binary.BigEndian, int32(0))
		_ = binary.Write(out, binary.BigEndian, int32(1))
		_ = binary.Write(out, binary.BigEndian, int32(1))
		_ = binary.Write(out, binary.BigEndian, int32(1))
		_ = binary.Write(out, binary.BigEndian, int32(1))
		_ = binary.Write(out, binary.BigEndian, int32(1))
	}
	return out.Bytes()
}

func (s *Server) handleProduce(r *bytes.Reader, fault *config.KafkaFault) []byte {
	var requiredAcks int16
	var timeout int32
	_ = binary.Read(r, binary.BigEndian, &requiredAcks)
	_ = binary.Read(r, binary.BigEndian, &timeout)
	var numTopics int32
	_ = binary.Read(r, binary.BigEndian, &numTopics)
	responses := make([]struct {
		topic     string
		partition int32
	}, 0)
	for i := int32(0); i < numTopics; i++ {
		topic := readKafkaString(r)
		var numPartitions int32
		_ = binary.Read(r, binary.BigEndian, &numPartitions)
		for j := int32(0); j < numPartitions; j++ {
			var partition int32
			var recordSetLen int32
			_ = binary.Read(r, binary.BigEndian, &partition)
			_ = binary.Read(r, binary.BigEndian, &recordSetLen)
			if recordSetLen < 0 || recordSetLen > 10*1024*1024 {
				return nil
			}
			recordSet := make([]byte, recordSetLen)
			_, _ = io.ReadFull(r, recordSet)
			key, value := parseMessageSet(recordSet)
			s.messages.Add(config.ProducedKafkaMessage{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Topic: topic, Partition: partition, Key: key, Value: value, Timestamp: time.Now().UTC().Format(time.RFC3339)})
			s.log.Log(logger.Entry{Protocol: "kafka", Method: "PRODUCE", Path: topic, Status: 0, Body: value})
			responses = append(responses, struct {
				topic     string
				partition int32
			}{topic: topic, partition: partition})
		}
	}
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int32(len(responses)))
	for _, resp := range responses {
		writeKafkaString(out, resp.topic)
		_ = binary.Write(out, binary.BigEndian, int32(1))
		_ = binary.Write(out, binary.BigEndian, resp.partition)
		errorCode := int16(0)
		if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
			errorCode = fault.ErrorCode
			if errorCode == 0 {
				errorCode = 5
			}
		}
		_ = binary.Write(out, binary.BigEndian, errorCode)
		_ = binary.Write(out, binary.BigEndian, int64(0))
	}
	return out.Bytes()
}

func (s *Server) handleFetch(r *bytes.Reader, fault *config.KafkaFault) []byte {
	var replicaID, maxWait, minBytes int32
	_ = binary.Read(r, binary.BigEndian, &replicaID)
	_ = binary.Read(r, binary.BigEndian, &maxWait)
	_ = binary.Read(r, binary.BigEndian, &minBytes)
	var numTopics int32
	_ = binary.Read(r, binary.BigEndian, &numTopics)
	topics := make([]string, 0, numTopics)
	for i := int32(0); i < numTopics; i++ {
		topic := readKafkaString(r)
		topics = append(topics, topic)
		var numPartitions int32
		_ = binary.Read(r, binary.BigEndian, &numPartitions)
		for j := int32(0); j < numPartitions; j++ {
			var partition int32
			var fetchOffset int64
			var maxBytes int32
			_ = binary.Read(r, binary.BigEndian, &partition)
			_ = binary.Read(r, binary.BigEndian, &fetchOffset)
			_ = binary.Read(r, binary.BigEndian, &maxBytes)
			_, _, _ = partition, fetchOffset, maxBytes
		}
	}
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, int32(len(topics)))
	for _, topic := range topics {
		writeKafkaString(out, topic)
		_ = binary.Write(out, binary.BigEndian, int32(1))
		_ = binary.Write(out, binary.BigEndian, int32(0))
		errorCode := int16(0)
		messageSet := s.buildMessageSet(topic)
		if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
			errorCode = fault.ErrorCode
			if errorCode == 0 {
				errorCode = 5
			}
			messageSet = nil
		}
		_ = binary.Write(out, binary.BigEndian, errorCode)
		_ = binary.Write(out, binary.BigEndian, int64(0))
		_ = binary.Write(out, binary.BigEndian, int32(len(messageSet)))
		out.Write(messageSet)
	}
	return out.Bytes()
}

func (s *Server) buildMessageSet(topic string) []byte {
	mock, ok := s.matchMock(topic)
	if !ok {
		return nil
	}
	if mock.Delay.Duration > 0 {
		time.Sleep(mock.Delay.Duration)
	}
	out := new(bytes.Buffer)
	for _, record := range mock.Records {
		msg := new(bytes.Buffer)
		msg.WriteByte(0)
		msg.WriteByte(0)
		writeKafkaBytes(msg, nullableBytes(record.Key))
		writeKafkaBytes(msg, []byte(record.Value))
		payload := msg.Bytes()
		crc := crc32.ChecksumIEEE(payload)
		entry := new(bytes.Buffer)
		_ = binary.Write(entry, binary.BigEndian, int64(0))
		_ = binary.Write(entry, binary.BigEndian, int32(len(payload)+4))
		_ = binary.Write(entry, binary.BigEndian, crc)
		entry.Write(payload)
		out.Write(entry.Bytes())
	}
	return out.Bytes()
}

func nullableBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return []byte(s)
}

func parseMessageSet(recordSet []byte) (string, string) {
	if len(recordSet) < 26 {
		return "", string(recordSet)
	}
	r := bytes.NewReader(recordSet)
	var offset int64
	var msgSize int32
	var crc uint32
	var magic, attrs byte
	_ = binary.Read(r, binary.BigEndian, &offset)
	_ = binary.Read(r, binary.BigEndian, &msgSize)
	_ = binary.Read(r, binary.BigEndian, &crc)
	_ = binary.Read(r, binary.BigEndian, &magic)
	_ = binary.Read(r, binary.BigEndian, &attrs)
	_, _, _, _, _ = offset, msgSize, crc, magic, attrs
	key := readKafkaBytes(r)
	value := readKafkaBytes(r)
	return string(key), string(value)
}

func (s *Server) matchMock(topic string) (config.KafkaMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if matchKafkaTopic(m.Topic, topic) {
			return m, true
		}
	}
	return config.KafkaMock{}, false
}

func matchKafkaTopic(pattern, topic string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(topic)
	}
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		return strings.HasPrefix(topic, parts[0]) && (parts[1] == "" || strings.HasSuffix(topic, parts[1]))
	}
	return pattern == topic
}

func readKafkaString(r *bytes.Reader) string {
	var n int16
	_ = binary.Read(r, binary.BigEndian, &n)
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	_, _ = io.ReadFull(r, buf)
	return string(buf)
}

func writeKafkaString(w io.Writer, s string) {
	_ = binary.Write(w, binary.BigEndian, int16(len(s)))
	_, _ = w.Write([]byte(s))
}

func readKafkaBytes(r *bytes.Reader) []byte {
	var n int32
	_ = binary.Read(r, binary.BigEndian, &n)
	if n < 0 {
		return nil
	}
	buf := make([]byte, n)
	_, _ = io.ReadFull(r, buf)
	return buf
}

func writeKafkaBytes(w io.Writer, b []byte) {
	if b == nil {
		_ = binary.Write(w, binary.BigEndian, int32(-1))
		return
	}
	_ = binary.Write(w, binary.BigEndian, int32(len(b)))
	_, _ = w.Write(b)
}

func readKafkaTopics(r *bytes.Reader) []string {
	var n int32
	_ = binary.Read(r, binary.BigEndian, &n)
	out := make([]string, 0, n)
	for i := int32(0); i < n; i++ {
		out = append(out, readKafkaString(r))
	}
	return out
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "kafka", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks), "messages": len(s.messages.All())}
}
