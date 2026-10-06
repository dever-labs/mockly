// Integration tests using a real nats.go client against the embedded server
// — exactly the kind of client a real application under test would use.
package natsserver_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	nats "github.com/nats-io/nats.go"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/natsserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// startServer boots a natsserver.Server on a free port and waits until it
// accepts connections, returning its URL and a cleanup-bound context.
func startServer(t *testing.T, cfg *config.NATSConfig) string {
	t.Helper()
	cfg.Port = freePort(t)

	srv := natsserver.New(cfg, state.New(), scenarios.New(nil), logger.New(100))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()

	url := fmt.Sprintf("nats://127.0.0.1:%d", cfg.Port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nc, err := nats.Connect(url, nats.Timeout(200*time.Millisecond))
		if err == nil {
			nc.Close()
			return url
		}
		select {
		case err := <-errCh:
			t.Fatalf("server exited early: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("nats server on %s never became ready", url)
	return ""
}

func TestNATS_CorePubSub(t *testing.T) {
	url := startServer(t, &config.NATSConfig{Enabled: true})

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	msgs := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe("greetings.hello", func(m *nats.Msg) { msgs <- m })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe() //nolint:errcheck

	if err := nc.Publish("greetings.hello", []byte("hi there")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case m := <-msgs:
		if string(m.Data) != "hi there" {
			t.Errorf("unexpected payload: %s", m.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestNATS_MockPubSubReaction(t *testing.T) {
	url := startServer(t, &config.NATSConfig{
		Enabled: true,
		Mocks: []config.NATSMock{{
			ID:      "echo",
			Subject: "orders.created",
			Response: &config.NATSResponse{
				Subject: "orders.ack",
				Payload: `{"received":"{{ .body }}"}`,
			},
		}},
	})

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	ackCh := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe("orders.ack", func(m *nats.Msg) { ackCh <- m })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe() //nolint:errcheck

	if err := nc.Publish("orders.created", []byte(`order-1`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case m := <-ackCh:
		want := `{"received":"order-1"}`
		if string(m.Data) != want {
			t.Errorf("expected %q, got %q", want, m.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mock reaction")
	}
}

func TestNATS_MockRequestReply(t *testing.T) {
	url := startServer(t, &config.NATSConfig{
		Enabled: true,
		Mocks: []config.NATSMock{{
			ID:      "ping",
			Subject: "svc.ping",
			Response: &config.NATSResponse{
				Payload: "pong",
			},
		}},
	})

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	reply, err := nc.Request("svc.ping", []byte("ping"), 2*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if string(reply.Data) != "pong" {
		t.Errorf("expected pong, got %q", reply.Data)
	}
}

func TestNATS_QueueGroup_LoadBalanced(t *testing.T) {
	url := startServer(t, &config.NATSConfig{Enabled: true})

	nc1, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect 1: %v", err)
	}
	defer nc1.Close()
	nc2, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect 2: %v", err)
	}
	defer nc2.Close()

	var count1, count2 int
	wait := make(chan struct{}, 10)
	sub1, err := nc1.QueueSubscribe("work.task", "workers", func(m *nats.Msg) {
		count1++
		wait <- struct{}{}
	})
	if err != nil {
		t.Fatalf("queue subscribe 1: %v", err)
	}
	defer sub1.Unsubscribe() //nolint:errcheck
	sub2, err := nc2.QueueSubscribe("work.task", "workers", func(m *nats.Msg) {
		count2++
		wait <- struct{}{}
	})
	if err != nil {
		t.Fatalf("queue subscribe 2: %v", err)
	}
	defer sub2.Unsubscribe() //nolint:errcheck

	// QueueSubscribe registers interest with the server asynchronously; flush
	// both connections so the server has both queue members registered before
	// publishing, otherwise all messages can race ahead to whichever member
	// subscribed first.
	if err := nc1.Flush(); err != nil {
		t.Fatalf("flush 1: %v", err)
	}
	if err := nc2.Flush(); err != nil {
		t.Fatalf("flush 2: %v", err)
	}

	const n = 10
	for i := 0; i < n; i++ {
		if err := nc1.Publish("work.task", []byte("job")); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	for i := 0; i < n; i++ {
		select {
		case <-wait:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for queue-grouped deliveries")
		}
	}

	if count1+count2 != n {
		t.Fatalf("expected %d total deliveries, got %d", n, count1+count2)
	}
	if count1 == 0 || count2 == 0 {
		t.Errorf("expected delivery split across both queue members, got %d/%d", count1, count2)
	}
}

func TestNATS_JetStream_StreamPublishAndFetch(t *testing.T) {
	url := startServer(t, &config.NATSConfig{
		Enabled: true,
		JetStream: &config.NATSJetStreamConfig{
			Enabled: true,
			Streams: []config.NATSStreamConfig{{
				Name:     "EVENTS",
				Subjects: []string{"events.>"},
			}},
		},
	})

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	if _, err := js.Publish("events.signup", []byte("alice")); err != nil {
		t.Fatalf("js publish: %v", err)
	}

	sub, err := js.PullSubscribe("events.>", "fetcher")
	if err != nil {
		t.Fatalf("pull subscribe: %v", err)
	}
	defer sub.Unsubscribe() //nolint:errcheck

	msgs, err := sub.Fetch(1, nats.MaxWait(2*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(msgs) != 1 || string(msgs[0].Data) != "alice" {
		t.Fatalf("unexpected fetched messages: %+v", msgs)
	}
	_ = msgs[0].Ack()
}

func TestNATS_JetStream_KVPutGet(t *testing.T) {
	url := startServer(t, &config.NATSConfig{
		Enabled: true,
		JetStream: &config.NATSJetStreamConfig{
			Enabled: true,
			KVBuckets: []config.NATSKVBucketConfig{{
				Bucket: "config",
			}},
		},
	})

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	kv, err := js.KeyValue("config")
	if err != nil {
		t.Fatalf("kv lookup: %v", err)
	}

	if _, err := kv.PutString("feature.flag", "enabled"); err != nil {
		t.Fatalf("kv put: %v", err)
	}

	entry, err := kv.Get("feature.flag")
	if err != nil {
		t.Fatalf("kv get: %v", err)
	}
	if string(entry.Value()) != "enabled" {
		t.Errorf("expected enabled, got %q", entry.Value())
	}
}
