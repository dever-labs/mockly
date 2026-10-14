# STOMP

STOMP 1.2 broker mock handling CONNECT, SEND, SUBSCRIBE, UNSUBSCRIBE, and DISCONNECT. Matching destinations can publish configured MESSAGE frames and captured inbound messages are stored for inspection.

```yaml
protocols:
  stomp:
    enabled: true
    port: 61613
    mocks:
      - id: process-order
        destination: "/queue/orders"
        response:
          body: '{"status":"queued"}'
          content_type: application/json
```

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `message`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#message-stores-amqp-kafka-stomp) — list/clear captured STOMP messages
