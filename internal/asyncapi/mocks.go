package asyncapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dever-labs/mockly/internal/config"
)

// buildMocks turns the normalised, protocol-agnostic operations into
// concrete per-protocol Mockly mocks.
//
// Not every AsyncAPI operation has a Mockly equivalent: Kafka mocks model
// pre-seeded topic data for consumers (so only "the app emits" operations
// map to one), while MQTT/AMQP/NATS mocks model a reactive
// listen-and-optionally-reply (so only "the app receives" operations map
// to one) since Mockly has no scheduled/spontaneous publish mechanism for
// those three protocols. Operations with no Mockly equivalent are skipped
// with a warning rather than silently dropped.
func buildMocks(ops []operation) *Result {
	res := &Result{}
	usedIDs := map[string]int{}

	wsByPath := map[string]*config.WebSocketMock{}
	var wsOrder []string

	for _, op := range ops {
		switch op.Protocol {
		case "kafka":
			if op.AppReceives {
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"%s: Kafka 'subscribe'/'receive' operations have no Mockly equivalent (Kafka mocks pre-seed topic records for consumers to read); skipped", op.ID))
				continue
			}
			res.Kafka = append(res.Kafka, config.KafkaMock{
				ID:      uniqueID(usedIDs, slugify(op.ID)),
				Topic:   op.Address,
				Records: []config.KafkaRecord{{Value: encodeJSON(op.Payload)}},
			})

		case "mqtt":
			if !op.AppReceives {
				res.Warnings = append(res.Warnings, skipPublishWarning(op.ID, "mqtt"))
				continue
			}
			mock := config.MQTTMock{ID: uniqueID(usedIDs, slugify(op.ID)), Topic: op.Address}
			if op.ReplyPayload != nil {
				mock.Response = &config.MQTTResponse{Payload: encodeJSON(op.ReplyPayload)}
			}
			res.MQTT = append(res.MQTT, mock)

		case "amqp":
			if !op.AppReceives {
				res.Warnings = append(res.Warnings, skipPublishWarning(op.ID, "amqp"))
				continue
			}
			mock := config.AMQPMock{ID: uniqueID(usedIDs, slugify(op.ID)), RoutingKey: op.Address}
			if op.ReplyPayload != nil {
				mock.Response = &config.AMQPResponse{Body: encodeJSON(op.ReplyPayload)}
			}
			res.AMQP = append(res.AMQP, mock)

		case "nats":
			if !op.AppReceives {
				res.Warnings = append(res.Warnings, skipPublishWarning(op.ID, "nats"))
				continue
			}
			mock := config.NATSMock{ID: uniqueID(usedIDs, slugify(op.ID)), Subject: op.Address}
			if op.ReplyPayload != nil {
				mock.Response = &config.NATSResponse{Payload: encodeJSON(op.ReplyPayload)}
			}
			res.NATS = append(res.NATS, mock)

		case "websocket":
			path := op.Address
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			mock, exists := wsByPath[path]
			if !exists {
				mock = &config.WebSocketMock{ID: uniqueID(usedIDs, slugify(op.ID)), Path: path}
				wsByPath[path] = mock
				wsOrder = append(wsOrder, path)
			}
			if op.AppReceives {
				rule := config.WebSocketRule{Match: "*"}
				if op.ReplyPayload != nil {
					rule.Respond = encodeJSON(op.ReplyPayload)
				}
				mock.OnMessage = append(mock.OnMessage, rule)
			} else {
				if mock.OnConnect != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf(
						"%s: another 'send'/'publish' operation already targets WebSocket path %q; only one on_connect push per path is supported, this operation's payload is discarded",
						op.ID, path))
				}
				mock.OnConnect = &config.WebSocketAction{Send: encodeJSON(op.Payload)}
			}

		default:
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: could not determine a supported protocol, skipped", op.ID))
		}
	}

	for _, p := range wsOrder {
		res.WebSocket = append(res.WebSocket, *wsByPath[p])
	}
	return res
}

func skipPublishWarning(id, protocol string) string {
	return fmt.Sprintf(
		"%s: %s 'publish'/'send' operations have no Mockly equivalent yet (no scheduled/spontaneous publish mechanism for %s); skipped. Use the management API to publish a message on demand instead.",
		id, protocol, protocol)
}

// encodeJSON renders v as indented JSON, or "" for a nil/unencodable value
// (an empty response body is a perfectly valid default, rather than
// failing the whole operation over a single bad example).
func encodeJSON(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

// uniqueID appends a numeric suffix to id if it was already used, so
// generated mocks never collide (mirrors internal/openapi's uniqueID).
func uniqueID(used map[string]int, id string) string {
	n := used[id]
	used[id]++
	if n == 0 {
		return id
	}
	return fmt.Sprintf("%s-%d", id, n+1)
}

// slugify sanitises an arbitrary operation/channel name into a mock ID
// (mirrors internal/openapi's slugify).
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "mock"
	}
	return out
}
