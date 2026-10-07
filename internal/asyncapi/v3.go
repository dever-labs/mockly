package asyncapi

import (
	"fmt"
	"strings"
)

// parseV3 extracts operations from an AsyncAPI 3.x document: "channels" is
// a map of channel definitions (address + messages), and "operations" is a
// separate map of operation objects that each reference a channel via
// {"$ref": "#/channels/<name>"} and declare action: "send" (the
// application emits a message) or "receive" (the application receives
// one). 3.x also lets a channel restrict itself to specific servers, which
// resolves the protocol unambiguously even in multi-server documents.
func parseV3(doc map[string]any) ([]operation, []string) {
	var ops []operation
	var warnings []string

	servers := parseServers(doc)
	defaultProtocol := defaultProtocolFrom(servers)

	channels, _ := asMap(doc["channels"])
	operations, _ := asMap(doc["operations"])

	for _, opName := range sortedKeys(operations) {
		opObj, ok := asMap(operations[opName])
		if !ok {
			continue
		}
		action, _ := opObj["action"].(string)

		chObj, chName, ok := resolveChannelRef(doc, opObj["channel"], channels)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("operation %q: could not resolve its channel, skipped", opName))
			continue
		}

		address, _ := chObj["address"].(string)
		if address == "" {
			address = chName
		}

		protocol := resolveProtocolV3(chObj, servers, defaultProtocol)
		if protocol == "" {
			warnings = append(warnings, fmt.Sprintf(
				"operation %q (channel %q): could not determine its protocol (document declares %d servers and the channel doesn't restrict itself to one); skipped",
				opName, address, len(servers)))
			continue
		}

		msgRaw := firstOperationMessage(opObj, chObj)
		var payload any
		if msgRaw != nil {
			payload, _ = extractMessage(doc, msgRaw, &warnings)
		}

		op := operation{
			ID:          opName,
			Protocol:    protocol,
			Address:     address,
			AppReceives: action == "receive",
			Payload:     payload,
		}

		if replyRaw, ok := asMap(opObj["reply"]); ok {
			if replyMsgRaw := firstReplyMessage(doc, replyRaw, channels); replyMsgRaw != nil {
				op.ReplyPayload, _ = extractMessage(doc, replyMsgRaw, &warnings)
			}
		}

		ops = append(ops, op)
	}

	return ops, warnings
}

// resolveChannelRef follows an operation's {"$ref": "#/channels/name"}
// channel field to the referenced channel object, returning its name too
// (the last path segment of the ref) for use as a fallback address/ID.
func resolveChannelRef(root map[string]any, chField any, channels map[string]any) (map[string]any, string, bool) {
	m, ok := asMap(chField)
	if !ok {
		return nil, "", false
	}
	refStr, ok := m["$ref"].(string)
	if !ok {
		return nil, "", false
	}
	target, err := resolvePointer(root, refStr)
	if err != nil {
		return nil, "", false
	}
	tm, ok := asMap(target)
	if !ok {
		return nil, "", false
	}
	parts := strings.Split(refStr, "/")
	name := parts[len(parts)-1]
	if _, declared := channels[name]; !declared {
		// Ref pointed somewhere other than the top-level channels map
		// (unusual, but still a valid pointer) — use the ref itself as a
		// best-effort name.
		name = refStr
	}
	return tm, name, true
}

// resolveProtocolV3 resolves a channel's protocol via its (optional)
// "servers" restriction, falling back to the document-wide default when
// the channel doesn't restrict itself.
func resolveProtocolV3(chObj map[string]any, servers map[string]serverInfo, defaultProtocol string) string {
	serversField, ok := chObj["servers"].([]any)
	if !ok || len(serversField) == 0 {
		return defaultProtocol
	}
	for _, s := range serversField {
		m, ok := asMap(s)
		if !ok {
			continue
		}
		refStr, ok := m["$ref"].(string)
		if !ok {
			continue
		}
		parts := strings.Split(refStr, "/")
		sname := parts[len(parts)-1]
		if info, ok := servers[sname]; ok && info.protocol != "" {
			return info.protocol
		}
	}
	return defaultProtocol
}

// firstOperationMessage picks the message to derive an example from: the
// operation's own "messages" list (each a $ref into the channel's
// messages) when present, otherwise the channel's first declared message.
func firstOperationMessage(opObj, chObj map[string]any) any {
	if arr, ok := opObj["messages"].([]any); ok && len(arr) > 0 {
		return arr[0]
	}
	if chMsgs, ok := asMap(chObj["messages"]); ok && len(chMsgs) > 0 {
		keys := sortedKeys(chMsgs)
		return chMsgs[keys[0]]
	}
	return nil
}

// firstReplyMessage picks the message to derive a reply example from: the
// reply's own "messages" list when present, otherwise the first message of
// the reply's referenced channel (if it names one).
func firstReplyMessage(root map[string]any, replyObj map[string]any, channels map[string]any) any {
	if arr, ok := replyObj["messages"].([]any); ok && len(arr) > 0 {
		return arr[0]
	}
	if chObj, _, ok := resolveChannelRef(root, replyObj["channel"], channels); ok {
		if chMsgs, ok := asMap(chObj["messages"]); ok && len(chMsgs) > 0 {
			keys := sortedKeys(chMsgs)
			return chMsgs[keys[0]]
		}
	}
	return nil
}
