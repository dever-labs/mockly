package amqpserver_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/amqpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// amqpHandshake performs just the Connection.{Start,Tune,Open} handshake
// (no channel/queue/consumer setup), leaving the caller free to open
// multiple channels/queues on the same connection — needed to exercise
// fanout delivery to several queues from a single publisher connection.
func amqpHandshake(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial AMQP: %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("AMQP\x00\x00\x09\x01")); err != nil {
		t.Fatalf("write AMQP protocol header: %v", err)
	}
	readFrame(t, conn)
	writeMethodFrame(t, conn, 0, 10, 11, nil)
	readFrame(t, conn)
	writeMethodFrame(t, conn, 0, 10, 31, make([]byte, 8))
	writeMethodFrame(t, conn, 0, 10, 40, shortStr(""))
	readFrame(t, conn)
	return conn
}

func openChannelAndConsume(t *testing.T, conn net.Conn, channel uint16, queue, consumerTag string) {
	t.Helper()
	writeMethodFrame(t, conn, channel, 20, 10, nil)
	readFrame(t, conn)
	queueArgs := append([]byte{0, 0}, shortStr(queue)...)
	writeMethodFrame(t, conn, channel, 50, 10, queueArgs)
	readFrame(t, conn)
	consumeArgs := append([]byte{0, 0}, shortStr(queue)...)
	consumeArgs = append(consumeArgs, shortStr(consumerTag)...)
	writeMethodFrame(t, conn, channel, 60, 20, consumeArgs)
	readFrame(t, conn)
}

func publishTo(t *testing.T, conn net.Conn, channel uint16, exchange, routingKey, body string) {
	t.Helper()
	args := append([]byte{0, 0}, shortStr(exchange)...)
	args = append(args, shortStr(routingKey)...)
	writeMethodFrame(t, conn, channel, 60, 40, args)
	writeHeaderFrame(t, conn, channel, 60, []byte(body))
	writeFrame(t, conn, 3, channel, []byte(body))
}

func expectNoDelivery(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("unexpectedly received a delivery")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("read error = %v, want timeout", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
}

func TestAMQPServer_Topology_DirectBinding(t *testing.T) {
	port := freePort(t)
	srv := amqpserver.New(&config.AMQPConfig{
		Enabled: true,
		Port:    port,
		Bindings: []config.AMQPBinding{
			{Exchange: "orders", RoutingKeyPattern: "orders.created", Queue: "orders-created-queue"},
		},
		Mocks: []config.AMQPMock{
			{ID: "created", Queue: "orders-created-queue", Response: &config.AMQPResponse{Body: "created-handled"}},
		},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)

	conn := amqpHandshake(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck
	openChannelAndConsume(t, conn, 1, "orders-created-queue", "ctag-1")

	publishTo(t, conn, 1, "orders", "orders.created", "payload")
	if body := readDeliveryBody(t, conn); body != "created-handled" {
		t.Fatalf("direct-binding delivery = %q, want %q", body, "created-handled")
	}

	publishTo(t, conn, 1, "orders", "orders.deleted", "payload")
	expectNoDelivery(t, conn)
}

func TestAMQPServer_Topology_FanoutBinding_DeliversToAllQueues(t *testing.T) {
	port := freePort(t)
	srv := amqpserver.New(&config.AMQPConfig{
		Enabled:   true,
		Port:      port,
		Exchanges: []config.AMQPExchange{{Name: "broadcast", Type: "fanout"}},
		Bindings: []config.AMQPBinding{
			{Exchange: "broadcast", Queue: "q1"},
			{Exchange: "broadcast", Queue: "q2"},
		},
		Mocks: []config.AMQPMock{
			{ID: "m1", Queue: "q1", Response: &config.AMQPResponse{Body: "q1-handled"}},
			{ID: "m2", Queue: "q2", Response: &config.AMQPResponse{Body: "q2-handled"}},
		},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)

	conn := amqpHandshake(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck
	openChannelAndConsume(t, conn, 1, "q1", "ctag-1")
	openChannelAndConsume(t, conn, 2, "q2", "ctag-2")

	publishTo(t, conn, 1, "broadcast", "irrelevant-key", "payload")

	got := map[string]bool{}
	for i := 0; i < 2; i++ {
		got[readDeliveryBody(t, conn)] = true
	}
	if !got["q1-handled"] || !got["q2-handled"] {
		t.Fatalf("fanout deliveries = %v, want both q1-handled and q2-handled", got)
	}
}

func TestAMQPServer_Topology_TopicBinding(t *testing.T) {
	port := freePort(t)
	srv := amqpserver.New(&config.AMQPConfig{
		Enabled:   true,
		Port:      port,
		Exchanges: []config.AMQPExchange{{Name: "orders", Type: "topic"}},
		Bindings: []config.AMQPBinding{
			{Exchange: "orders", RoutingKeyPattern: "orders.*.created", Queue: "created-queue"},
		},
		Mocks: []config.AMQPMock{
			{ID: "created", Queue: "created-queue", Response: &config.AMQPResponse{Body: "topic-handled"}},
		},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)

	conn := amqpHandshake(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck
	openChannelAndConsume(t, conn, 1, "created-queue", "ctag-1")

	publishTo(t, conn, 1, "orders", "orders.us.created", "payload")
	if body := readDeliveryBody(t, conn); body != "topic-handled" {
		t.Fatalf("topic-binding delivery = %q, want %q", body, "topic-handled")
	}

	publishTo(t, conn, 1, "orders", "orders.us.updated", "payload")
	expectNoDelivery(t, conn)
}

func TestAMQPServer_Topology_FlatModeUnaffectedWhenNoBindings(t *testing.T) {
	// No Bindings configured: behavior must be identical to pre-topology
	// flat Exchange+RoutingKey matching.
	port := freePort(t)
	srv := amqpserver.New(&config.AMQPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.AMQPMock{
			{ID: "flat", Exchange: "test", RoutingKey: "rk", Response: &config.AMQPResponse{Body: "flat-handled"}},
		},
	}, state.New(), scenarios.New(nil), logger.New(100))
	startServer(t, srv)

	conn := openAMQPConnection(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck
	publishAMQP(t, conn)
	if body := readDeliveryBody(t, conn); body != "flat-handled" {
		t.Fatalf("flat-mode delivery = %q, want %q", body, "flat-handled")
	}
}
