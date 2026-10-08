// Package configgen resolves the inline schema references a Mockly config
// can declare (HTTPConfig.OpenAPI, the AsyncAPI field on each of
// Kafka/MQTT/AMQP/NATS/WebSocketConfig, and GRPCService.Proto) by running
// the same generators `mockly generate` uses, and layering the result
// underneath whatever mocks are already hand-written in that section.
//
// This gives two ways to consume a spec: `mockly generate` turns it into a
// static, one-time config snapshot you then own and edit freely; an inline
// schema reference instead keeps the spec and the config file as two live
// inputs that are merged every time the config is loaded, so faults,
// scenarios, state and extra hand-written mocks can be layered on top of a
// spec that keeps evolving, without ever hand-copying its output.
package configgen

import (
	"fmt"
	"path/filepath"

	"github.com/dever-labs/mockly/internal/asyncapi"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/openapi"
	"github.com/dever-labs/mockly/internal/protoidl"
)

// Resolve expands every inline schema reference in cfg in place. baseDir is
// the directory a relative spec path is resolved against — normally the
// directory containing the config file itself, so a config and the specs it
// references can be moved together without hardcoding an absolute path or
// depending on the process's current working directory.
//
// It returns every warning emitted by a generator (e.g. an operation it
// couldn't derive a mock for), which is not fatal — generation still
// succeeds for everything else. A spec that fails to parse at all, or an
// unresolvable import, is a hard error: an explicit, broken schema
// reference in the config shouldn't fail silently.
func Resolve(cfg *config.Config, baseDir string) ([]string, error) {
	var warnings []string
	resolvePath := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(baseDir, p)
	}

	if cfg.Protocols.HTTP != nil && cfg.Protocols.HTTP.OpenAPI != "" {
		ref := cfg.Protocols.HTTP.OpenAPI
		res, err := openapi.Generate(resolvePath(ref))
		if err != nil {
			return warnings, fmt.Errorf("resolving protocols.http.openapi %q: %w", ref, err)
		}
		warnings = append(warnings, res.Warnings...)
		cfg.Protocols.HTTP.Mocks = mergeByID(res.Mocks, cfg.Protocols.HTTP.Mocks, httpMockID)
	}

	// The same AsyncAPI spec can be referenced from more than one protocol
	// block (e.g. it describes both a Kafka and a WebSocket channel), so
	// cache parsed results by resolved path to avoid re-parsing it once per
	// block that references it.
	asyncCache := map[string]*asyncapi.Result{}
	getAsync := func(ref string) (*asyncapi.Result, error) {
		specPath := resolvePath(ref)
		if res, ok := asyncCache[specPath]; ok {
			return res, nil
		}
		res, err := asyncapi.Generate(specPath)
		if err != nil {
			return nil, err
		}
		asyncCache[specPath] = res
		warnings = append(warnings, res.Warnings...)
		return res, nil
	}

	if cfg.Protocols.Kafka != nil && cfg.Protocols.Kafka.AsyncAPI != "" {
		ref := cfg.Protocols.Kafka.AsyncAPI
		res, err := getAsync(ref)
		if err != nil {
			return warnings, fmt.Errorf("resolving protocols.kafka.asyncapi %q: %w", ref, err)
		}
		cfg.Protocols.Kafka.Mocks = mergeByID(res.Kafka, cfg.Protocols.Kafka.Mocks, kafkaMockID)
	}
	if cfg.Protocols.MQTT != nil && cfg.Protocols.MQTT.AsyncAPI != "" {
		ref := cfg.Protocols.MQTT.AsyncAPI
		res, err := getAsync(ref)
		if err != nil {
			return warnings, fmt.Errorf("resolving protocols.mqtt.asyncapi %q: %w", ref, err)
		}
		cfg.Protocols.MQTT.Mocks = mergeByID(res.MQTT, cfg.Protocols.MQTT.Mocks, mqttMockID)
	}
	if cfg.Protocols.AMQP != nil && cfg.Protocols.AMQP.AsyncAPI != "" {
		ref := cfg.Protocols.AMQP.AsyncAPI
		res, err := getAsync(ref)
		if err != nil {
			return warnings, fmt.Errorf("resolving protocols.amqp.asyncapi %q: %w", ref, err)
		}
		cfg.Protocols.AMQP.Mocks = mergeByID(res.AMQP, cfg.Protocols.AMQP.Mocks, amqpMockID)
	}
	if cfg.Protocols.NATS != nil && cfg.Protocols.NATS.AsyncAPI != "" {
		ref := cfg.Protocols.NATS.AsyncAPI
		res, err := getAsync(ref)
		if err != nil {
			return warnings, fmt.Errorf("resolving protocols.nats.asyncapi %q: %w", ref, err)
		}
		cfg.Protocols.NATS.Mocks = mergeByID(res.NATS, cfg.Protocols.NATS.Mocks, natsMockID)
	}
	if cfg.Protocols.WebSocket != nil && cfg.Protocols.WebSocket.AsyncAPI != "" {
		ref := cfg.Protocols.WebSocket.AsyncAPI
		res, err := getAsync(ref)
		if err != nil {
			return warnings, fmt.Errorf("resolving protocols.websocket.asyncapi %q: %w", ref, err)
		}
		cfg.Protocols.WebSocket.Mocks = mergeByID(res.WebSocket, cfg.Protocols.WebSocket.Mocks, websocketMockID)
	}

	if cfg.Protocols.GRPC != nil {
		for i := range cfg.Protocols.GRPC.Services {
			svc := &cfg.Protocols.GRPC.Services[i]
			if svc.Proto == "" {
				continue
			}
			res, err := protoidl.Generate(resolvePath(svc.Proto))
			if err != nil {
				return warnings, fmt.Errorf("resolving protocols.grpc.services[%d].proto %q: %w", i, svc.Proto, err)
			}
			warnings = append(warnings, res.Warnings...)
			svc.Mocks = mergeByID(res.Mocks, svc.Mocks, grpcMockID)
		}
	}

	return warnings, nil
}

func httpMockID(m config.HTTPMock) string           { return m.ID }
func kafkaMockID(m config.KafkaMock) string         { return m.ID }
func mqttMockID(m config.MQTTMock) string           { return m.ID }
func amqpMockID(m config.AMQPMock) string           { return m.ID }
func natsMockID(m config.NATSMock) string           { return m.ID }
func websocketMockID(m config.WebSocketMock) string { return m.ID }
func grpcMockID(m config.GRPCMock) string           { return m.ID }

// mergeByID layers handWritten entries over generated ones: an entry whose
// ID matches a generated one overrides it in place (keeping the generated
// ordering), and any other entry is appended after. This is what lets a
// config combine mocks derived from an inline schema reference with mocks
// written by hand, without losing either: edit the generated mock's
// counterpart under the same "id" to override it (e.g. to add a fault or
// pin a specific response), or add a new "id" for something the spec
// doesn't describe at all (e.g. a scenario-only mock).
func mergeByID[T any](generated, handWritten []T, idOf func(T) string) []T {
	if len(handWritten) == 0 {
		return generated
	}
	index := make(map[string]int, len(generated))
	result := make([]T, len(generated))
	copy(result, generated)
	for i, m := range result {
		index[idOf(m)] = i
	}
	for _, m := range handWritten {
		id := idOf(m)
		if idx, ok := index[id]; ok {
			result[idx] = m
		} else {
			index[id] = len(result)
			result = append(result, m)
		}
	}
	return result
}
