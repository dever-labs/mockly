## Kafka

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
