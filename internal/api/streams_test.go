package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/api"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/natsserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/streamregistry"
)

// startStreamAPI starts an API server with real stubHTTP/stubGRPC/stubGraphQL
// instances (so their embedded stubStream can be preloaded/inspected by
// tests) and returns the base URL plus the three stubs.
func startStreamAPI(t *testing.T) (base string, httpStub *stubHTTP, grpcStub *stubGRPC, gqlStub *stubGraphQL) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{}
	cfg.Mockly.API.Port = port

	httpStub = &stubHTTP{}
	grpcStub = &stubGRPC{}
	gqlStub = &stubGraphQL{}

	srv := api.New(
		cfg, state.New(), scenarios.New(nil), logger.New(100), nil,
		httpStub,
		&stubWS{},
		grpcStub,
		gqlStub,
		&stubTCP{},
		&stubRedis{},
		&stubSMTP{inbox: smtpserver.NewInbox(50)},
		&stubMQTT{ms: mqttserver.NewMessageStore(50)},
		&stubNATS{ms: natsserver.NewMessageStore(50)},
		&stubSNMP{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base = fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base+"/api/protocols", 2*time.Second)
	return base, httpStub, grpcStub, gqlStub
}

func TestAPI_ListStreams_AggregatesAcrossProtocols(t *testing.T) {
	base, httpStub, grpcStub, gqlStub := startStreamAPI(t)
	httpStub.infos = []streamregistry.Info{{ID: "http-1", Protocol: "http", MockID: "m1"}}
	grpcStub.infos = []streamregistry.Info{{ID: "grpc-1", Protocol: "grpc", MockID: "m2"}}
	gqlStub.infos = []streamregistry.Info{{ID: "gql-1", Protocol: "graphql", MockID: "m3"}}

	resp, err := http.Get(base + "/api/streams")
	if err != nil {
		t.Fatalf("GET /api/streams: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got []streamregistry.Info
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 streams across protocols, got %d: %+v", len(got), got)
	}
}

func TestAPI_PushStream_DispatchesByProtocol(t *testing.T) {
	base, httpStub, _, _ := startStreamAPI(t)

	body, _ := json.Marshal(map[string]string{
		"protocol": "http",
		"mock_id":  "stream-mock",
		"data":     `{"n":1}`,
	})
	resp, err := http.Post(base+"/api/streams/push", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/streams/push: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if len(httpStub.pushes) != 1 || httpStub.pushes[0].mockID != "stream-mock" || httpStub.pushes[0].data != `{"n":1}` {
		t.Errorf("want push routed to stubHTTP, got %+v", httpStub.pushes)
	}
}

func TestAPI_CloseStream_DispatchesByProtocol(t *testing.T) {
	base, _, grpcStub, _ := startStreamAPI(t)

	body, _ := json.Marshal(map[string]string{
		"protocol": "grpc",
		"conn_id":  "grpc-conn-1",
		"error":    "boom",
	})
	resp, err := http.Post(base+"/api/streams/close", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/streams/close: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if len(grpcStub.closes) != 1 || grpcStub.closes[0].connID != "grpc-conn-1" || grpcStub.closes[0].errMsg != "boom" {
		t.Errorf("want close routed to stubGRPC, got %+v", grpcStub.closes)
	}
}

func TestAPI_PushStream_UnknownProtocolReturns400(t *testing.T) {
	base, _, _, _ := startStreamAPI(t)

	body, _ := json.Marshal(map[string]string{"protocol": "mqtt", "mock_id": "m1", "data": "x"})
	resp, err := http.Post(base+"/api/streams/push", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/streams/push: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAPI_PushStream_MissingTargetReturns400(t *testing.T) {
	base, _, _, _ := startStreamAPI(t)

	body, _ := json.Marshal(map[string]string{"protocol": "http", "data": "x"})
	resp, err := http.Post(base+"/api/streams/push", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/streams/push: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAPI_PushStream_PropagatesUnderlyingError(t *testing.T) {
	base, httpStub, _, _ := startStreamAPI(t)
	httpStub.err = fmt.Errorf("no active stream connections for mock %q", "ghost")

	body, _ := json.Marshal(map[string]string{"protocol": "http", "mock_id": "ghost", "data": "x"})
	resp, err := http.Post(base+"/api/streams/push", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/streams/push: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
