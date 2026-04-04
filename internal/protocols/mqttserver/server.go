// Package mqttserver implements an MQTT broker with configurable mock responses.
package mqttserver

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

// ReceivedMessage is a captured inbound MQTT message.
type ReceivedMessage struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	Payload   string `json:"payload"`
	QoS       byte   `json:"qos"`
	Retain    bool   `json:"retain"`
	Timestamp string `json:"timestamp"`
}

// MessageStore holds captured MQTT messages.
type MessageStore struct {
	mu       sync.RWMutex
	messages []ReceivedMessage
	maxSize  int
}

func newMessageStore(maxSize int) *MessageStore {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MessageStore{maxSize: maxSize}
}

func (m *MessageStore) Add(msg ReceivedMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) >= m.maxSize {
		m.messages = m.messages[1:]
	}
	m.messages = append(m.messages, msg)
}

func (m *MessageStore) All() []ReceivedMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ReceivedMessage, len(m.messages))
	copy(out, m.messages)
	return out
}

func (m *MessageStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

// Server is the MQTT broker mock server.
type Server struct {
	cfg      *config.MQTTConfig
	store    *state.Store
	log      *logger.Logger
	mocks    []config.MQTTMock
	messages *MessageStore
	broker   *mqtt.Server
	mu       sync.RWMutex
}

// New creates a Server.
func New(cfg *config.MQTTConfig, store *state.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:      cfg,
		store:    store,
		log:      log,
		mocks:    append([]config.MQTTMock(nil), cfg.Mocks...),
		messages: newMessageStore(1000),
	}
}

func (s *Server) SetMocks(mocks []config.MQTTMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.MQTTMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.MQTTMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.MQTTMock(nil), s.mocks...)
}

func (s *Server) GetMessageStore() *MessageStore {
	return s.messages
}

// Start runs the MQTT broker. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	broker := mqtt.New(&mqtt.Options{})

	if err := broker.AddHook(new(auth.AllowHook), nil); err != nil {
		return fmt.Errorf("mqtt add auth hook: %w", err)
	}

	if err := broker.AddHook(&mockHook{srv: s}, nil); err != nil {
		return fmt.Errorf("mqtt add mock hook: %w", err)
	}

	ln := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: fmt.Sprintf(":%d", s.cfg.Port),
	})
	if err := broker.AddListener(ln); err != nil {
		return fmt.Errorf("mqtt add listener: %w", err)
	}

	s.broker = broker

	// broker.Serve() is non-blocking in mochi-mqtt v2 — it starts background goroutines.
	if err := broker.Serve(); err != nil {
		return fmt.Errorf("mqtt serve: %w", err)
	}

	// Block until context is cancelled, then cleanly shut down.
	<-ctx.Done()
	broker.Close()
	return nil
}

// matchMock returns the first mock whose topic pattern matches the incoming topic.
// Supports MQTT wildcards: + (single level), # (multi-level).
func (s *Server) matchMock(topic string) (config.MQTTMock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if matchMQTTTopic(m.Topic, topic) {
			return m, true
		}
	}
	return config.MQTTMock{}, false
}

// matchMQTTTopic returns true if the filter (possibly with wildcards) matches the topic.
func matchMQTTTopic(filter, topic string) bool {
	if filter == "#" {
		return true
	}
	if filter == topic {
		return true
	}
	filterParts := strings.Split(filter, "/")
	topicParts := strings.Split(topic, "/")

	for i, f := range filterParts {
		if f == "#" {
			return true
		}
		if i >= len(topicParts) {
			return false
		}
		if f != "+" && f != topicParts[i] {
			return false
		}
	}
	return len(filterParts) == len(topicParts)
}

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"protocol": "mqtt",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
		"messages": len(s.messages.All()),
	}
}

// ---------------------------------------------------------------------------
// mochi-mqtt hook
// ---------------------------------------------------------------------------

type mockHook struct {
	mqtt.HookBase
	srv *Server
}

func (h *mockHook) ID() string { return "mockly-mock-hook" }

func (h *mockHook) Provides(b byte) bool {
	return bytes.Contains([]byte{mqtt.OnPublish}, []byte{b})
}

func (h *mockHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// Skip messages published by the broker itself (responses).
	if cl == nil {
		return pk, nil
	}

	topic := pk.TopicName
	payload := string(pk.Payload)

	// Capture the message.
	h.srv.messages.Add(ReceivedMessage{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Topic:     topic,
		Payload:   payload,
		QoS:       pk.FixedHeader.Qos,
		Retain:    pk.FixedHeader.Retain,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})

	h.srv.log.Log(logger.Entry{
		Protocol: "mqtt",
		Method:   "PUBLISH",
		Path:     topic,
		Status:   0,
		Body:     payload,
	})

	mock, matched := h.srv.matchMock(topic)
	if !matched || mock.Response == nil {
		return pk, nil
	}

	// Publish the response asynchronously to avoid re-entrancy issues.
	resp := mock.Response
	broker := h.srv.broker
	go func() {
		if resp.Delay.Duration > 0 {
			time.Sleep(resp.Delay.Duration)
		}
		responseTopic := resp.Topic
		if responseTopic == "" {
			responseTopic = topic + "/response"
		}
		broker.Publish(responseTopic, []byte(resp.Payload), resp.Retain, resp.QoS) //nolint:errcheck
		h.srv.log.Log(logger.Entry{
			Protocol:  "mqtt",
			Method:    "PUBLISH_RESP",
			Path:      responseTopic,
			Status:    0,
			Body:      resp.Payload,
			MatchedID: mock.ID,
		})
	}()

	return pk, nil
}
