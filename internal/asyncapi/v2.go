package asyncapi

import "fmt"

// parseV2 extracts operations from an AsyncAPI 2.x document: channels are a
// top-level map keyed by channel name, each optionally holding "publish"
// (the application emits messages here) and/or "subscribe" (the
// application receives messages here) operation objects.
//
// 2.x has no per-channel "servers" restriction (that was added in 3.x), so
// when the document declares more than one server the protocol can only be
// resolved per-channel via a channel.bindings hint (e.g. a "kafka:" or
// "mqtt:" key); channels with no such hint and an ambiguous (multi-server)
// document are skipped with a warning.
func parseV2(doc map[string]any) ([]operation, []string) {
	var ops []operation
	var warnings []string

	servers := parseServers(doc)
	defaultProtocol := defaultProtocolFrom(servers)

	channels, _ := asMap(doc["channels"])
	for _, name := range sortedKeys(channels) {
		ch, ok := asMap(channels[name])
		if !ok {
			continue
		}

		protocol := defaultProtocol
		if bindings, ok := asMap(ch["bindings"]); ok {
			for _, p := range []string{"kafka", "mqtt", "amqp", "amqp1", "nats", "ws"} {
				if _, exists := bindings[p]; exists {
					protocol = normalizeProtocol(p)
					break
				}
			}
		}
		if protocol == "" {
			warnings = append(warnings, fmt.Sprintf(
				"channel %q: could not determine its protocol (document declares %d servers and the channel has no protocol-specific bindings); skipped",
				name, len(servers)))
			continue
		}

		if pubRaw, ok := ch["publish"]; ok {
			if op := buildV2Operation(doc, name, protocol, false, pubRaw, &warnings); op != nil {
				ops = append(ops, *op)
			}
		}
		if subRaw, ok := ch["subscribe"]; ok {
			if op := buildV2Operation(doc, name, protocol, true, subRaw, &warnings); op != nil {
				ops = append(ops, *op)
			}
		}
	}

	return ops, warnings
}

// buildV2Operation turns a single 2.x publish/subscribe operation object
// into a normalised operation.
func buildV2Operation(root map[string]any, address, protocol string, appReceives bool, opRaw any, warnings *[]string) *operation {
	opObj, ok := asMap(opRaw)
	if !ok {
		return nil
	}
	msgRaw, ok := opObj["message"]
	if !ok {
		*warnings = append(*warnings, fmt.Sprintf("channel %q: operation has no message defined, skipped", address))
		return nil
	}

	payload, _ := extractMessage(root, msgRaw, warnings)

	id := address
	if opID, ok := opObj["operationId"].(string); ok && opID != "" {
		id = opID
	}

	return &operation{
		ID:          id,
		Protocol:    protocol,
		Address:     address,
		AppReceives: appReceives,
		Payload:     payload,
	}
}
