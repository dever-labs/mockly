// Package natsserver embeds the real NATS server (core pub/sub,
// request/reply, queue groups, and optional JetStream) in-process, exactly
// like mqttserver embeds mochi-mqtt. Because it is a genuine nats-server,
// any real nats.go / JetStream / KV client can connect to it. On top of
// that, Mockly offers an optional declarative "mocks" auto-responder layer
// for the no-client-code fake-service use case.
package natsserver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	natslib "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ReceivedMessage is a captured inbound NATS message.
type ReceivedMessage struct {
	ID        string `json:"id"`
	Subject   string `json:"subject"`
	Reply     string `json:"reply,omitempty"`
	Payload   string `json:"payload"`
	Timestamp string `json:"timestamp"`
}

// MessageStore holds captured NATS messages using a ring buffer (O(1) add).
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

// Server embeds a real NATS server (and optional JetStream) and layers an
// optional declarative mock auto-responder on top via an internal, in-process
// nats.go client.
type Server struct {
	cfg       *config.NATSConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger

	mu    sync.RWMutex
	mocks []config.NATSMock

	messages *MessageStore

	ns *natslib.Server
	nc *nats.Conn

	// catchAll subscribes to ">" for plain (non queue-grouped) mocks, which
	// are matched in-process so that subject patterns may use "{name}"
	// captures in addition to standard NATS "*"/">" wildcards.
	catchAll *nats.Subscription
	// queueSubs holds one dedicated nats.Subscription per queue-grouped
	// mock (queue groups only work on literal subject subscriptions).
	queueSubs []*nats.Subscription

	tmpStoreDir string
}

// New creates a Server.
func New(cfg *config.NATSConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		mocks:     append([]config.NATSMock(nil), cfg.Mocks...),
		messages:  newMessageStore(1000),
	}
}

// SetMocks replaces the active set of mocks and re-establishes subscriptions.
func (s *Server) SetMocks(mocks []config.NATSMock) {
	s.mu.Lock()
	s.mocks = append([]config.NATSMock(nil), mocks...)
	nc := s.nc
	s.mu.Unlock()

	if nc != nil {
		if err := s.resubscribe(); err != nil {
			s.log.Log(logger.Entry{
				Protocol: "nats",
				Method:   "SUBSCRIBE_ERR",
				Status:   0,
				Body:     fmt.Sprintf("resubscribe failed: %v", err),
			})
		}
	}
}

func (s *Server) GetMocks() []config.NATSMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.NATSMock(nil), s.mocks...)
}

func (s *Server) GetMessageStore() *MessageStore {
	return s.messages
}

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info := map[string]interface{}{
		"protocol": "nats",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
		"messages": len(s.messages.All()),
	}
	if s.cfg.JetStream != nil {
		info["jetstream"] = s.cfg.JetStream.Enabled
	}
	return info
}

// Start runs the embedded NATS server. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	opts := &natslib.Options{
		Host:   "0.0.0.0",
		Port:   s.cfg.Port,
		NoLog:  true,
		NoSigs: true,
	}

	if s.cfg.JetStream != nil && s.cfg.JetStream.Enabled {
		opts.JetStream = true
		storeDir := s.cfg.JetStream.StoreDir
		if storeDir == "" {
			dir, err := os.MkdirTemp("", "mockly-nats-js-")
			if err != nil {
				return fmt.Errorf("creating jetstream temp store dir: %w", err)
			}
			storeDir = dir
			s.tmpStoreDir = dir
		}
		opts.StoreDir = storeDir
	}

	ns, err := natslib.NewServer(opts)
	if err != nil {
		return fmt.Errorf("creating nats server: %w", err)
	}
	s.ns = ns

	ns.Start()
	defer func() {
		ns.Shutdown()
		ns.WaitForShutdown()
		if s.tmpStoreDir != "" {
			_ = os.RemoveAll(s.tmpStoreDir)
		}
	}()

	if !ns.ReadyForConnections(10 * time.Second) {
		return fmt.Errorf("nats server did not become ready in time")
	}

	nc, err := nats.Connect("", nats.InProcessServer(ns))
	if err != nil {
		return fmt.Errorf("connecting internal mock client: %w", err)
	}
	s.nc = nc
	defer nc.Close()

	if s.cfg.JetStream != nil && s.cfg.JetStream.Enabled {
		if err := s.provisionJetStream(nc); err != nil {
			s.log.Log(logger.Entry{
				Protocol: "nats",
				Method:   "JETSTREAM_PROVISION_ERR",
				Status:   0,
				Body:     fmt.Sprintf("jetstream provisioning failed: %v", err),
			})
		}
	}

	if err := s.resubscribe(); err != nil {
		return fmt.Errorf("subscribing mocks: %w", err)
	}

	<-ctx.Done()
	return nil
}

// provisionJetStream pre-creates any streams/KV buckets declared in config.
func (s *Server) provisionJetStream(nc *nats.Conn) error {
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	for _, st := range s.cfg.JetStream.Streams {
		if _, err := js.AddStream(&nats.StreamConfig{Name: st.Name, Subjects: st.Subjects}); err != nil {
			return fmt.Errorf("creating stream %q: %w", st.Name, err)
		}
	}
	for _, kv := range s.cfg.JetStream.KVBuckets {
		if _, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: kv.Bucket}); err != nil {
			return fmt.Errorf("creating kv bucket %q: %w", kv.Bucket, err)
		}
	}
	return nil
}

// resubscribe tears down existing mock subscriptions and re-creates them
// from the current mock set. Safe to call repeatedly (e.g. after SetMocks).
func (s *Server) resubscribe() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.catchAll != nil {
		_ = s.catchAll.Unsubscribe()
		s.catchAll = nil
	}
	for _, sub := range s.queueSubs {
		_ = sub.Unsubscribe()
	}
	s.queueSubs = nil

	hasPlain := false
	for _, m := range s.mocks {
		if m.QueueGroup == "" {
			hasPlain = true
			continue
		}
		mock := m
		sub, err := s.nc.QueueSubscribe(mock.Subject, mock.QueueGroup, func(msg *nats.Msg) {
			s.handleMessage(msg, mock, nil)
		})
		if err != nil {
			return fmt.Errorf("queue subscribing %q (group %q): %w", mock.Subject, mock.QueueGroup, err)
		}
		s.queueSubs = append(s.queueSubs, sub)
	}

	if hasPlain {
		sub, err := s.nc.Subscribe(">", func(msg *nats.Msg) {
			mock, matched, params := s.matchPlainMock(msg.Subject)
			if !matched {
				// Still capture the message even if no mock matches.
				s.captureOnly(msg)
				return
			}
			s.handleMessage(msg, mock, params)
		})
		if err != nil {
			return fmt.Errorf("subscribing catch-all: %w", err)
		}
		s.catchAll = sub
	}

	return nil
}

// matchPlainMock returns the first non-queue-grouped mock whose subject
// pattern matches, along with any "{name}" captures.
func (s *Server) matchPlainMock(subject string) (config.NATSMock, bool, map[string]string) {
	for _, m := range s.mocks {
		if m.QueueGroup != "" {
			continue
		}
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if ok, params := matchNATSSubject(m.Subject, subject); ok {
			return m, true, params
		}
	}
	return config.NATSMock{}, false, nil
}

// matchNATSSubject reports whether pattern matches subject, honouring the
// standard NATS wildcards ("*" single token, ">" remainder, must be last)
// plus "{name}" captures (behaves like "*" but also captures the token).
func matchNATSSubject(pattern, subject string) (bool, map[string]string) {
	if pattern == subject {
		return true, nil
	}
	patternParts := strings.Split(pattern, ".")
	subjectParts := strings.Split(subject, ".")

	var params map[string]string
	for i, p := range patternParts {
		if p == ">" {
			return true, params
		}
		if i >= len(subjectParts) {
			return false, nil
		}
		if p == "*" {
			continue
		}
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			if name := p[1 : len(p)-1]; name != "" {
				if params == nil {
					params = make(map[string]string)
				}
				params[name] = subjectParts[i]
			}
			continue
		}
		if p != subjectParts[i] {
			return false, nil
		}
	}
	return len(patternParts) == len(subjectParts), params
}

func (s *Server) captureOnly(msg *nats.Msg) {
	s.messages.Add(ReceivedMessage{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Subject:   msg.Subject,
		Reply:     msg.Reply,
		Payload:   string(msg.Data),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	s.log.Log(logger.Entry{
		Protocol: "nats",
		Method:   "PUB",
		Path:     msg.Subject,
		Status:   0,
		Body:     string(msg.Data),
	})
}

// handleMessage captures the message, applies fault injection, and —if a
// response is configured— replies (request/reply) or publishes (pub/sub
// reaction) a templated payload.
func (s *Server) handleMessage(msg *nats.Msg, mock config.NATSMock, params map[string]string) {
	payload := string(msg.Data)

	fault := s.scenarios.EffectiveNATSFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	s.messages.Add(ReceivedMessage{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Subject:   msg.Subject,
		Reply:     msg.Reply,
		Payload:   payload,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	s.log.Log(logger.Entry{
		Protocol:   "nats",
		Method:     "PUB",
		Path:       msg.Subject,
		Status:     0,
		Body:       payload,
		MatchedID:  mock.ID,
		PathParams: params,
	})

	if mock.Response == nil {
		return
	}
	if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
		return
	}

	resp := mock.Response
	reqCtx := engine.RequestContext{
		Path:       msg.Subject,
		Body:       payload,
		PathParams: params,
	}

	go func() {
		if resp.Delay.Duration > 0 {
			time.Sleep(resp.Delay.Duration)
		}
		responsePayload := engine.Render(resp.Payload, reqCtx)

		var err error
		destSubject := ""
		if msg.Reply != "" {
			destSubject = msg.Reply
			err = msg.Respond([]byte(responsePayload))
		} else if resp.Subject != "" {
			destSubject = engine.Render(resp.Subject, reqCtx)
			err = s.nc.Publish(destSubject, []byte(responsePayload))
		} else {
			return
		}

		if err != nil {
			s.log.Log(logger.Entry{
				Protocol: "nats",
				Method:   "PUB_ERR",
				Path:     destSubject,
				Status:   0,
				Body:     fmt.Sprintf("publish failed: %v", err),
			})
			return
		}
		s.log.Log(logger.Entry{
			Protocol:   "nats",
			Method:     "PUB_RESP",
			Path:       destSubject,
			Status:     0,
			Body:       responsePayload,
			MatchedID:  mock.ID,
			PathParams: params,
		})
	}()
}
