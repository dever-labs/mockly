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
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
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

// MessageStore holds captured MQTT messages using a ring buffer (O(1) add).
// When full, the oldest entry is overwritten.
type MessageStore struct {
	mu      sync.RWMutex
	buf     []ReceivedMessage
	head    int
	count   int
	maxSize int
}

func newMessageStore(maxSize int) *MessageStore {
	if maxSize <= 0 {
		maxSize = config.DefaultMessageStoreSize
	}
	return &MessageStore{buf: make([]ReceivedMessage, maxSize), maxSize: maxSize}
}

// NewMessageStore creates a new MessageStore with the given capacity. Exported for testing.
func NewMessageStore(maxSize int) *MessageStore { return newMessageStore(maxSize) }

func (m *MessageStore) Add(msg ReceivedMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tail := (m.head + m.count) % m.maxSize
	m.buf[tail] = msg
	if m.count == m.maxSize {
		m.head = (m.head + 1) % m.maxSize
	} else {
		m.count++
	}
}

func (m *MessageStore) All() []ReceivedMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ReceivedMessage, m.count)
	for i := 0; i < m.count; i++ {
		out[i] = m.buf[(m.head+i)%m.maxSize]
	}
	return out
}

func (m *MessageStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.head = 0
	m.count = 0
}

// Server is the MQTT broker mock server.
type Server struct {
	cfg       *config.MQTTConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger
	mocks     []config.MQTTMock
	messages  *MessageStore
	broker    *mqtt.Server
	mu        sync.RWMutex
}

// New creates a Server.
func New(cfg *config.MQTTConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		mocks:     append([]config.MQTTMock(nil), cfg.Mocks...),
		messages:  newMessageStore(1000),
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
	_ = broker.Close()
	return nil
}

// matchMock returns the first mock whose topic pattern matches the incoming topic.
// Supports MQTT wildcards: + (single level), # (multi-level), and {name} captures.
func (s *Server) matchMock(topic string) (config.MQTTMock, bool, map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if ok, params := matchMQTTTopic(m.Topic, topic); ok {
			return m, true, params
		}
	}
	return config.MQTTMock{}, false, nil
}

// matchMQTTTopic returns whether filter matches topic and any named captures.
// Supports MQTT standard wildcards (+ single-level, # multi-level) and {name}
// captures which behave like + but also capture the segment value.
func matchMQTTTopic(filter, topic string) (bool, map[string]string) {
	if filter == "#" {
		return true, nil
	}
	if filter == topic {
		return true, nil
	}
	filterParts := strings.Split(filter, "/")
	topicParts := strings.Split(topic, "/")

	var params map[string]string
	for i, f := range filterParts {
		if f == "#" {
			return true, params
		}
		if i >= len(topicParts) {
			return false, nil
		}
		if f == "+" {
			continue
		}
		if strings.HasPrefix(f, "{") && strings.HasSuffix(f, "}") {
			if name := f[1 : len(f)-1]; name != "" {
				if params == nil {
					params = make(map[string]string)
				}
				params[name] = topicParts[i]
			}
			continue
		}
		if f != topicParts[i] {
			return false, nil
		}
	}
	return len(filterParts) == len(topicParts), params
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

	fault := h.srv.scenarios.EffectiveMQTTFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

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

	mock, matched, topicParams := h.srv.matchMock(topic)
	if !matched || mock.Response == nil {
		return pk, nil
	}
	if fault != nil && h.srv.scenarios.RollFault(fault.ErrorRate) {
		return pk, nil
	}

	// Publish the response asynchronously to avoid re-entrancy issues.
	resp := mock.Response
	broker := h.srv.broker
	reqCtx := engine.RequestContext{
		Path:       topic,
		Body:       payload,
		PathParams: topicParams,
	}
	go func() {
		if resp.Delay.Duration > 0 {
			time.Sleep(resp.Delay.Duration)
		}
		responseTopic := engine.Render(resp.Topic, reqCtx)
		if responseTopic == "" {
			responseTopic = topic + "/response"
		}
		responsePayload := engine.Render(resp.Payload, reqCtx)
		if err := broker.Publish(responseTopic, []byte(responsePayload), resp.Retain, resp.QoS); err != nil {
			// Non-fatal: log and continue — this is a mock server best-effort publish.
			h.srv.log.Log(logger.Entry{
				Protocol: "mqtt",
				Method:   "PUBLISH_ERR",
				Path:     responseTopic,
				Status:   0,
				Body:     fmt.Sprintf("publish failed: %v", err),
			})
			return
		}
		h.srv.log.Log(logger.Entry{
			Protocol:   "mqtt",
			Method:     "PUBLISH_RESP",
			Path:       responseTopic,
			Status:     0,
			Body:       responsePayload,
			MatchedID:  mock.ID,
			PathParams: topicParams,
		})
	}()

	return pk, nil
}
