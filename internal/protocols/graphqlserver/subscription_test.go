// Integration tests for the graphql-transport-ws subscription transport.
package graphqlserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// startSubTestServer starts a real GraphQL server (needed for a genuine
// WebSocket upgrade — httptest.ResponseRecorder can't hijack) and returns a
// dialed, protocol-negotiated graphql-transport-ws connection to it.
func startSubTestServer(t *testing.T, mocks []config.GraphQLMock) (*Server, *websocket.Conn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.GraphQLConfig{Enabled: true, Port: port, Path: "/graphql"}
	srv := New(cfg, state.New(), scenarios.New(nil), logger.New(10))
	srv.SetMocks(mocks)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	addr := net.JoinHostPort("127.0.0.1", itoa(port))
	waitForHTTP(t, "http://"+addr+"/graphql", 2*time.Second)

	dialer := websocket.Dialer{Subprotocols: []string{"graphql-transport-ws"}}
	conn, _, err := dialer.Dial("ws://"+addr+"/graphql", nil)
	if err != nil {
		t.Fatalf("dial subscription websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return srv, conn
}

func waitForHTTP(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec,noctx
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready within %v", url, timeout)
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{digits[n%10]}, buf...)
		n /= 10
	}
	return string(buf)
}

func readWSMessage(t *testing.T, conn *websocket.Conn) graphqlWSMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var msg graphqlWSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	return msg
}

func writeTestWSMessage(t *testing.T, conn *websocket.Conn, msg graphqlWSMessage) {
	t.Helper()
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
}

func TestSubscription_ConnectionInitAck(t *testing.T) {
	_, conn := startSubTestServer(t, nil)

	writeTestWSMessage(t, conn, graphqlWSMessage{Type: "connection_init"})
	msg := readWSMessage(t, conn)
	if msg.Type != "connection_ack" {
		t.Errorf("Type = %q, want connection_ack", msg.Type)
	}
}

func TestSubscription_PingPong(t *testing.T) {
	_, conn := startSubTestServer(t, nil)

	writeTestWSMessage(t, conn, graphqlWSMessage{Type: "ping"})
	msg := readWSMessage(t, conn)
	if msg.Type != "pong" {
		t.Errorf("Type = %q, want pong", msg.Type)
	}
}

func TestSubscription_NextMessagesThenComplete(t *testing.T) {
	mocks := []config.GraphQLMock{{
		ID:            "order-updates-sub",
		OperationType: "subscription",
		OperationName: "onOrderUpdate",
		Stream: []config.GraphQLStreamEvent{
			{Data: `{"data":{"onOrderUpdate":{"status":"pending"}}}`},
			{Data: `{"data":{"onOrderUpdate":{"status":"shipped"}}}`},
		},
	}}
	_, conn := startSubTestServer(t, mocks)

	writeTestWSMessage(t, conn, graphqlWSMessage{Type: "connection_init"})
	if msg := readWSMessage(t, conn); msg.Type != "connection_ack" {
		t.Fatalf("Type = %q, want connection_ack", msg.Type)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"query":         "subscription onOrderUpdate { onOrderUpdate { status } }",
		"operationName": "onOrderUpdate",
	})
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "1", Type: "subscribe", Payload: payload})

	msg1 := readWSMessage(t, conn)
	if msg1.ID != "1" || msg1.Type != "next" {
		t.Fatalf("message 1 = %+v, want next/id=1", msg1)
	}
	var body1 map[string]interface{}
	_ = json.Unmarshal(msg1.Payload, &body1)
	status1 := body1["data"].(map[string]interface{})["onOrderUpdate"].(map[string]interface{})["status"]
	if status1 != "pending" {
		t.Errorf("status1 = %v, want pending", status1)
	}

	msg2 := readWSMessage(t, conn)
	if msg2.ID != "1" || msg2.Type != "next" {
		t.Fatalf("message 2 = %+v, want next/id=1", msg2)
	}
	var body2 map[string]interface{}
	_ = json.Unmarshal(msg2.Payload, &body2)
	status2 := body2["data"].(map[string]interface{})["onOrderUpdate"].(map[string]interface{})["status"]
	if status2 != "shipped" {
		t.Errorf("status2 = %v, want shipped", status2)
	}

	msg3 := readWSMessage(t, conn)
	if msg3.ID != "1" || msg3.Type != "complete" {
		t.Fatalf("message 3 = %+v, want complete/id=1", msg3)
	}
}

func TestSubscription_RespectsDelay(t *testing.T) {
	d := config.Duration{}
	d.Duration = 100 * time.Millisecond
	mocks := []config.GraphQLMock{{
		ID:            "slow-sub",
		OperationName: "SlowSub",
		Stream: []config.GraphQLStreamEvent{
			{Data: `{"data":{"n":1}}`, Delay: d},
		},
	}}
	_, conn := startSubTestServer(t, mocks)

	payload, _ := json.Marshal(map[string]interface{}{
		"query":         "subscription SlowSub { n }",
		"operationName": "SlowSub",
	})
	start := time.Now()
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "1", Type: "subscribe", Payload: payload})

	msg := readWSMessage(t, conn)
	if msg.Type != "next" {
		t.Fatalf("Type = %q, want next", msg.Type)
	}
	if elapsed := time.Since(start); elapsed < 80*time.Millisecond {
		t.Errorf("expected delay >=80ms, got %v", elapsed)
	}
}

func TestSubscription_UnmatchedOperationReturnsError(t *testing.T) {
	_, conn := startSubTestServer(t, nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"query":         "subscription Missing { missing }",
		"operationName": "Missing",
	})
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "1", Type: "subscribe", Payload: payload})

	msg := readWSMessage(t, conn)
	if msg.ID != "1" || msg.Type != "error" {
		t.Fatalf("message = %+v, want error/id=1", msg)
	}
}

func TestSubscription_QueryOperationWithoutStreamErrors(t *testing.T) {
	mocks := []config.GraphQLMock{{
		ID:            "plain-query",
		OperationName: "GetUser",
		Response:      map[string]interface{}{"id": "1"},
	}}
	_, conn := startSubTestServer(t, mocks)

	payload, _ := json.Marshal(map[string]interface{}{
		"query":         "query GetUser { id }",
		"operationName": "GetUser",
	})
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "1", Type: "subscribe", Payload: payload})

	// The mock matched but has no Stream, so the subscription transport
	// can't serve it either — it should still error rather than hang.
	msg := readWSMessage(t, conn)
	if msg.ID != "1" || msg.Type != "error" {
		t.Fatalf("message = %+v, want error/id=1", msg)
	}
}

func TestSubscription_CompleteCancelsRemainingEvents(t *testing.T) {
	d := config.Duration{}
	d.Duration = 200 * time.Millisecond
	mocks := []config.GraphQLMock{{
		ID:            "cancel-sub",
		OperationName: "CancelSub",
		Stream: []config.GraphQLStreamEvent{
			{Data: `{"data":{"n":1}}`},
			{Data: `{"data":{"n":2}}`, Delay: d},
		},
	}}
	_, conn := startSubTestServer(t, mocks)

	payload, _ := json.Marshal(map[string]interface{}{
		"query":         "subscription CancelSub { n }",
		"operationName": "CancelSub",
	})
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "1", Type: "subscribe", Payload: payload})

	msg1 := readWSMessage(t, conn)
	if msg1.Type != "next" {
		t.Fatalf("message 1 = %+v, want next", msg1)
	}

	// Cancel before the second (delayed) event would have been sent.
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "1", Type: "complete"})

	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("expected no further messages after complete, but got one")
	}
}

func TestSubscription_InvalidTemplatedJSONReturnsError(t *testing.T) {
	mocks := []config.GraphQLMock{{
		ID:            "bad-json-sub",
		OperationName: "BadJSON",
		Stream: []config.GraphQLStreamEvent{
			{Data: `not valid json`},
		},
	}}
	_, conn := startSubTestServer(t, mocks)

	payload, _ := json.Marshal(map[string]interface{}{
		"query":         "subscription BadJSON { n }",
		"operationName": "BadJSON",
	})
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "1", Type: "subscribe", Payload: payload})

	msg := readWSMessage(t, conn)
	if msg.ID != "1" || msg.Type != "error" {
		t.Fatalf("message = %+v, want error/id=1", msg)
	}
}

func TestSubscription_MultiplexedByID(t *testing.T) {
	mocks := []config.GraphQLMock{
		{ID: "sub-a", OperationName: "SubA", Stream: []config.GraphQLStreamEvent{{Data: `{"data":{"a":1}}`}}},
		{ID: "sub-b", OperationName: "SubB", Stream: []config.GraphQLStreamEvent{{Data: `{"data":{"b":1}}`}}},
	}
	_, conn := startSubTestServer(t, mocks)

	payloadA, _ := json.Marshal(map[string]interface{}{"query": "subscription SubA { a }", "operationName": "SubA"})
	payloadB, _ := json.Marshal(map[string]interface{}{"query": "subscription SubB { b }", "operationName": "SubB"})
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "a", Type: "subscribe", Payload: payloadA})
	writeTestWSMessage(t, conn, graphqlWSMessage{ID: "b", Type: "subscribe", Payload: payloadB})

	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		msg := readWSMessage(t, conn)
		if msg.Type == "next" {
			seen[msg.ID] = true
		}
	}
	if !seen["a"] || !seen["b"] {
		t.Errorf("seen = %v, want both subscription ids to have produced a next message", seen)
	}
}
