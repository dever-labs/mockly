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
