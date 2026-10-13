# Kafka

Kafka wire-protocol mock covering ApiVersions, Metadata, Produce, and Fetch flows. Published messages are stored for later inspection.

```yaml
protocols:
  kafka:
    enabled: true
    port: 9092
    mocks:
      - id: orders-topic
        topic: orders
        records:
          - key: "order-1"
            value: '{"id":1,"status":"pending"}'
```

## See also

- [Configuration](../getting-started/configuration.md) — matching, templated responses, state conditions, and scenarios shared by every protocol
- [Fault Injection](../testing/fault-injection.md#fault-fields-per-protocol) — this protocol's fault fields: `error_code`, `delay`, `error_rate`
- [Management API](../reference/management-api.md#message-stores-amqp-kafka-stomp) — list/clear captured Kafka messages
