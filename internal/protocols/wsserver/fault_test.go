package wsserver_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/wsserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func startFaultServer(t *testing.T, srv interface{ Start(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(100 * time.Millisecond)
}

func freeFaultPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	time.Sleep(10 * time.Millisecond)
	return port
}

func dialFaultWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))  //nolint:errcheck
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	return conn
}

func TestWSServer_GlobalFault(t *testing.T) {
	port := freeFaultPort(t)
	sc := scenarios.New(nil)
	srv := wsserver.New(&config.WebSocketConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.WebSocketMock{{
			ID:   "m",
			Path: "/ws",
			OnMessage: []config.WebSocketRule{{
				Match:   "hello",
				Respond: "world",
			}},
		}},
	}, state.New(), sc, logger.New(100))
	startFaultServer(t, srv)

	url := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
	sc.SetDirectFaults(config.ProtocolFaults{WebSocket: &config.WebSocketFault{ErrorRate: 0}})
	conn := dialFaultWS(t, url)
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write fault websocket message: %v", err)
	}
	_, _, err := conn.ReadMessage()
	_ = conn.Close()
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("fault read error = %v, want CloseError", err)
	}
	if closeErr.Code != websocket.CloseInternalServerErr {
		t.Fatalf("fault close code = %d, want %d", closeErr.Code, websocket.CloseInternalServerErr)
	}

	sc.ClearDirectFaults()
	conn = dialFaultWS(t, url)
	defer conn.Close() //nolint:errcheck
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write normal websocket message: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read normal websocket response: %v", err)
	}
	if string(msg) != "world" {
		t.Fatalf("normal response = %q, want %q", msg, "world")
	}
}

func TestWSServer_WebSocketFault_CustomCloseCode(t *testing.T) {
	port := freeFaultPort(t)
	sc := scenarios.New(nil)
	srv := wsserver.New(&config.WebSocketConfig{Enabled: true, Port: port, Mocks: []config.WebSocketMock{{ID: "m", Path: "/ws", OnMessage: []config.WebSocketRule{{Match: "hello", Respond: "world"}}}}}, state.New(), sc, logger.New(100))
	startFaultServer(t, srv)
	sc.SetDirectFaults(config.ProtocolFaults{WebSocket: &config.WebSocketFault{CloseCode: websocket.ClosePolicyViolation, Message: "blocked", ErrorRate: 0}})
	conn := dialFaultWS(t, fmt.Sprintf("ws://127.0.0.1:%d/ws", port))
	defer conn.Close() //nolint:errcheck
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := conn.ReadMessage()
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("fault read error = %v, want CloseError", err)
	}
	if closeErr.Code != websocket.ClosePolicyViolation {
		t.Fatalf("fault close code = %d, want %d", closeErr.Code, websocket.ClosePolicyViolation)
	}
}
