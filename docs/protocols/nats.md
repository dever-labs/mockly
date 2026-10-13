# NATS

Mockly embeds the real [nats-server](https://github.com/nats-io/nats-server) in-process (the same way MQTT embeds mochi-mqtt), so any real `nats.go` / JetStream / KV client can connect and use the full protocol: core pub/sub, subject wildcards, request/reply, and queue groups. On top of that, Mockly offers an optional declarative `mocks` auto-responder layer for the no-client-code fake-service use case.

```yaml
protocols:
  nats:
    enabled: true
    port: 4222
    mocks:
      - id: order-ack
        subject: "orders.created"
        response:
          subject: "orders.ack"
          payload: '{"received":"{{ .body }}"}'
```

Subject wildcards: `*` matches a single token, `>` matches the remainder (must be last), and `{name}` captures a single token for use in response templates and logs — e.g. `orders.{id}.created`.

For request/reply (`nats.Request(...)`), omit `response.subject`: Mockly replies directly on the caller's reply-to subject.

```yaml
      - id: ping
        subject: "svc.ping"
        response:
          payload: "pong"
```

Set `queue_group` to have the mock join a real NATS queue group, so only one member of the group (Mockly or one of your app's own instances) answers each matching message:

```yaml
      - id: worker
        subject: "work.task"
        queue_group: workers
        response:
          payload: '{"status":"done"}'
```

## JetStream

Enable JetStream to get real streams, consumers, and KV/Object Store, backed by an on-disk (or ephemeral temp-dir) store:

```yaml
protocols:
  nats:
    enabled: true
    jetstream:
      enabled: true
      # store_dir: /var/lib/mockly/jetstream   # omit for an ephemeral temp dir
      streams:
        - name: EVENTS
          subjects: ["events.>"]
      kv_buckets:
        - bucket: config
```

`streams`/`kv_buckets` are an optional convenience for a ready-to-use demo config — any real client can also create streams, consumers, and KV buckets itself via the standard JetStream management API.

Captured messages are visible at `GET /api/nats/messages`; mocks can be managed live at `GET/POST /api/mocks/nats` and `PUT/DELETE /api/mocks/nats/{id}`.

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `delay`, `error_rate`
- [Management API](../reference/management-api.md#nats-messages) — list/clear captured NATS messages
