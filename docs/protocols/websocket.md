# WebSocket

`on_connect.send` and each `on_message[*].respond` value are rendered as Go templates. The incoming message text is available as `{{.request.body}}`.

```yaml
protocols:
  websocket:
    enabled: true
    port: 8081
    mocks:
      - id: ticker
        path: /ws/ticker
        on_connect:
          send: '{"event":"connected","path":"{{.request.path}}"}'
        on_message:
          - match: ping
            respond: '{"event":"pong","echo":"{{.request.body}}"}'
```

## Binary frames

`match_binary`/`respond_binary` (and `on_connect.send_binary`) hold base64-encoded raw bytes and operate on binary (opcode `0x2`) WebSocket frames — useful for Protobuf/MessagePack-framed APIs or other binary protocols. A rule with `match_binary` only matches binary frames and compares the decoded bytes exactly (no wildcard/regex); plain `match`/`respond` rules are unaffected and keep working exactly as before (including against binary frames, matched as text, for backward compatibility).

```yaml
      - id: binary-echo
        path: /ws/binary
        on_connect:
          send_binary: "AQIDBA=="   # base64 for 0x01 0x02 0x03 0x04
        on_message:
          - match_binary: "3q2+7w==" # base64 for 0xDE 0xAD 0xBE 0xEF
            respond_binary: "yv4="  # base64 for 0xCA 0xFE
```

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `close_code`, `message`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#http-mocks) — mocks for this protocol follow the same CRUD pattern as HTTP mocks (`/api/mocks/{protocol}`)
