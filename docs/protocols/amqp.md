## AMQP

AMQP 0.9.1 mock broker. Handles connection, channel, and basic frames, supports publish/consume flows, and captures published messages for inspection via the management API.

```yaml
protocols:
  amqp:
    enabled: true
    port: 5672
    # Optional: only "0.9.1" (the default) is currently implemented.
    # AMQP 1.0 is a structurally different protocol and is not yet
    # supported — setting "1.0" fails config validation.
    protocol_version: "0.9.1"
    # Flat mode (default): mocks are matched directly against a published
    # message's exchange+routing_key and delivered to whichever consumer
    # happens to be active on the publishing channel.
    mocks:
      - id: order-created
        exchange: orders
        routing_key: "order.created"
        response:
          body: '{"status":"accepted"}'
```

Setting `bindings` switches to real exchange/queue topology instead: a published message is routed (per that exchange's declared `type` — `direct`/`topic`/`fanout`) to every bound queue, and mocks match by `queue` rather than `exchange`+`routing_key`. This lets one published message fan out to multiple queues, mirroring how a real broker like RabbitMQ routes messages.

```yaml
protocols:
  amqp:
    enabled: true
    port: 5672
    exchanges:
      - name: orders-topic-exchange
        type: topic # "direct" (default), "topic", or "fanout"
    bindings:
      - exchange: orders-topic-exchange
        # "." word-separated; "*" = exactly one word, "#" = zero or more words.
        routing_key_pattern: "orders.*.created"
        queue: orders-created-queue
      - exchange: orders-topic-exchange
        routing_key_pattern: "orders.#"
        queue: orders-audit-queue
    mocks:
      - id: order-created-handler
        queue: orders-created-queue
        response:
          body: '{"status":"accepted"}'
      - id: order-audit-handler
        queue: orders-audit-queue
        response:
          body: '{"status":"logged"}'
```
