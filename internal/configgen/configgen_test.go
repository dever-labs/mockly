package configgen

import (
	"strings"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestResolve_HTTPOpenAPI_MergesHandWrittenOverride(t *testing.T) {
	cfg := &config.Config{
		Protocols: config.ProtocolsConfig{
			HTTP: &config.HTTPConfig{
				Enabled: true,
				OpenAPI: "orders.yaml",
				Mocks: []config.HTTPMock{
					// Overrides the generated "listorders" mock's response.
					{
						ID:       "listorders",
						Request:  config.HTTPRequest{Method: "GET", Path: "/orders"},
						Response: config.HTTPResponse{Status: 200, Body: `{"count":42}`},
					},
					// A hand-written mock with no spec counterpart at all.
					{
						ID:       "create-order",
						Request:  config.HTTPRequest{Method: "POST", Path: "/orders"},
						Response: config.HTTPResponse{Status: 201},
					},
				},
			},
		},
	}

	warnings, err := Resolve(cfg, "testdata")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	mocks := cfg.Protocols.HTTP.Mocks
	if len(mocks) != 2 {
		t.Fatalf("got %d mocks, want 2: %+v", len(mocks), mocks)
	}
	if mocks[0].ID != "listorders" || mocks[0].Response.Body != `{"count":42}` {
		t.Errorf("generated mock wasn't overridden by hand-written entry: %+v", mocks[0])
	}
	if mocks[1].ID != "create-order" {
		t.Errorf("hand-written-only mock wasn't appended: %+v", mocks[1])
	}
}

func TestResolve_AsyncAPI_SharedSpecAcrossProtocols(t *testing.T) {
	cfg := &config.Config{
		Protocols: config.ProtocolsConfig{
			Kafka: &config.KafkaConfig{
				Enabled:  true,
				AsyncAPI: "events.yaml",
			},
			WebSocket: &config.WebSocketConfig{
				Enabled:  true,
				AsyncAPI: "events.yaml",
				Mocks: []config.WebSocketMock{
					// Overrides the generated on_connect push payload.
					{ID: "pushliveupdate", Path: "/ws/updates", OnConnect: &config.WebSocketAction{Send: `{"status":"override"}`}},
				},
			},
		},
	}

	warnings, err := Resolve(cfg, "testdata")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	if len(cfg.Protocols.Kafka.Mocks) != 1 || cfg.Protocols.Kafka.Mocks[0].ID != "publishordercreated" {
		t.Fatalf("Kafka mocks = %+v", cfg.Protocols.Kafka.Mocks)
	}
	if len(cfg.Protocols.WebSocket.Mocks) != 1 {
		t.Fatalf("WebSocket mocks = %+v", cfg.Protocols.WebSocket.Mocks)
	}
	if got := cfg.Protocols.WebSocket.Mocks[0].OnConnect.Send; got != `{"status":"override"}` {
		t.Errorf("WebSocket on_connect override didn't apply, got %q", got)
	}
}

func TestResolve_GRPCProto(t *testing.T) {
	cfg := &config.Config{
		Protocols: config.ProtocolsConfig{
			GRPC: &config.GRPCConfig{
				Enabled: true,
				Services: []config.GRPCService{
					{
						Proto: "users.proto",
						Mocks: []config.GRPCMock{
							{ID: "extra-method", Method: "Ping"},
						},
					},
				},
			},
		},
	}

	warnings, err := Resolve(cfg, "testdata")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	mocks := cfg.Protocols.GRPC.Services[0].Mocks
	if len(mocks) != 2 {
		t.Fatalf("got %d mocks, want 2 (1 generated + 1 hand-written): %+v", len(mocks), mocks)
	}
	var methods []string
	for _, m := range mocks {
		methods = append(methods, m.Method)
	}
	if methods[0] != "GetUser" || methods[1] != "Ping" {
		t.Errorf("unexpected methods %v", methods)
	}
}

func TestResolve_NoReferences_NoOp(t *testing.T) {
	cfg := &config.Config{
		Protocols: config.ProtocolsConfig{
			HTTP: &config.HTTPConfig{Enabled: true, Mocks: []config.HTTPMock{{ID: "a"}}},
		},
	}
	warnings, err := Resolve(cfg, "testdata")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(cfg.Protocols.HTTP.Mocks) != 1 || cfg.Protocols.HTTP.Mocks[0].ID != "a" {
		t.Errorf("HTTP mocks unexpectedly changed: %+v", cfg.Protocols.HTTP.Mocks)
	}
}

func TestResolve_BadSpecPath_IsHardError(t *testing.T) {
	cfg := &config.Config{
		Protocols: config.ProtocolsConfig{
			HTTP: &config.HTTPConfig{Enabled: true, OpenAPI: "does-not-exist.yaml"},
		},
	}
	_, err := Resolve(cfg, "testdata")
	if err == nil {
		t.Fatal("expected an error for a missing spec file, got nil")
	}
	if !strings.Contains(err.Error(), "protocols.http.openapi") {
		t.Errorf("error should mention the offending field, got: %v", err)
	}
}
