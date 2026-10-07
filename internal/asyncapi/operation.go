package asyncapi

import (
	"encoding/json"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/dever-labs/mockly/internal/schemaexample"
)

// operation is a protocol-agnostic view of a single AsyncAPI publish/send or
// subscribe/receive operation, normalised from either AsyncAPI 2.x
// (channel.publish/channel.subscribe) or 3.x (operations[].action
// "send"/"receive") documents.
type operation struct {
	// ID is a human-readable label (operationId, or channel/operation name)
	// used to derive the generated mock's ID and in warning messages.
	ID string
	// Protocol is one of "kafka", "mqtt", "amqp", "nats", "websocket", or ""
	// if it couldn't be determined (the operation is then skipped).
	Protocol string
	// Address is the channel's topic/subject/routing-key/path.
	Address string
	// AppReceives is true for a 2.x "subscribe" / 3.x action:"receive"
	// operation: the application described by the spec receives messages
	// that external clients send it (so Mockly should react to incoming
	// messages on Address). It is false for 2.x "publish" / 3.x
	// action:"send": the application emits messages for external
	// consumers to read (so Mockly should make data available on Address
	// without waiting for input).
	AppReceives bool
	// Payload is the synthesised example value for the operation's primary
	// message, or nil if none could be derived.
	Payload any
	// ReplyPayload is the synthesised example value for the operation's
	// reply message (3.0 operation.reply; 2.x has no equivalent), or nil.
	ReplyPayload any
}

// normalizeProtocol maps an AsyncAPI server "protocol" value to one of the
// Mockly protocols the generator can target, or "" if unrecognised/
// unsupported.
func normalizeProtocol(p string) string {
	switch strings.ToLower(p) {
	case "kafka", "kafka-secure":
		return "kafka"
	case "mqtt", "mqtts", "secure-mqtt":
		return "mqtt"
	case "amqp", "amqp1", "amqps":
		return "amqp"
	case "nats":
		return "nats"
	case "ws", "wss", "websocket", "websockets":
		return "websocket"
	default:
		return ""
	}
}

// serverInfo is the subset of an AsyncAPI Server Object this generator
// cares about.
type serverInfo struct {
	protocol string
}

// parseServers extracts the top-level "servers" map (name -> protocol),
// shared verbatim between AsyncAPI 2.x and 3.x documents.
func parseServers(doc map[string]any) map[string]serverInfo {
	out := map[string]serverInfo{}
	servers, _ := asMap(doc["servers"])
	for name, raw := range servers {
		if m, ok := asMap(raw); ok {
			if p, ok := m["protocol"].(string); ok {
				out[name] = serverInfo{protocol: normalizeProtocol(p)}
			}
		}
	}
	return out
}

// defaultProtocolFrom returns the lone server's protocol when the document
// declares exactly one server (the common case), or "" when zero or
// multiple servers are declared — in the ambiguous multi-server case,
// per-channel resolution (bindings hints, or 3.0 channel.servers) decides
// instead.
func defaultProtocolFrom(servers map[string]serverInfo) string {
	if len(servers) == 1 {
		for _, s := range servers {
			return s.protocol
		}
	}
	return ""
}

// extractMessage derives an example payload value from an AsyncAPI Message
// Object (which may itself be a bare {"$ref": "..."}), preferring an
// explicit example over one synthesised from the payload JSON Schema.
func extractMessage(root map[string]any, raw any, warnings *[]string) (any, bool) {
	msgObj, ok := asMap(raw)
	if !ok {
		return nil, false
	}
	if refStr, ok := msgObj["$ref"].(string); ok {
		target, err := resolvePointer(root, refStr)
		if err != nil {
			*warnings = append(*warnings, err.Error())
			return nil, false
		}
		return extractMessage(root, target, warnings)
	}
	if oneOf, ok := msgObj["oneOf"].([]any); ok && len(oneOf) > 0 {
		return extractMessage(root, oneOf[0], warnings)
	}
	if examples, ok := msgObj["examples"].([]any); ok && len(examples) > 0 {
		if ex, ok := asMap(examples[0]); ok {
			if p, ok := ex["payload"]; ok {
				return inlineRefs(root, p, 0, warnings), true
			}
		}
	}
	if payloadRaw, ok := msgObj["payload"]; ok {
		resolved := inlineRefs(root, payloadRaw, 0, warnings)
		schema := toSchemaRef(resolved)
		return schemaexample.Generate(schema, 0), true
	}
	return nil, false
}

// toSchemaRef converts a plain YAML/JSON-decoded schema (map[string]any,
// already fully $ref-inlined) into the openapi3.Schema shape so the shared
// schemaexample.Generate logic can be reused verbatim: AsyncAPI's payload
// schemas are JSON-Schema-compatible (2.x: a documented superset of Draft
// 7, closely mirroring OpenAPI 3.0's Schema Object; 3.x: plain JSON
// Schema), and openapi3.Schema's own JSON unmarshalling already
// understands both dialects' overlap well enough for example synthesis.
func toSchemaRef(schema any) *openapi3.SchemaRef {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var s openapi3.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &openapi3.SchemaRef{Value: &s}
}
