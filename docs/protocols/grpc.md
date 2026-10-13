# gRPC

Dynamic gRPC mocking — no compiled `.proto` files needed. Uses a raw codec to intercept any service/method call. Supports unary RPCs as well as server-streaming, client-streaming, and bidirectional-streaming.

```yaml
protocols:
  grpc:
    enabled: true
    port: 50051
    services:
      - mocks:
          # Unary
          - id: charge
            method: Charge
            response:
              success: true
              charge_id: ch_123

          # Server-streaming: reads the client's one request, then sends
          # each `stream` entry in order (each with its own optional delay).
          - id: list-charges
            method: ListCharges
            stream:
              - response: {charge_id: ch_1}
              - response: {charge_id: ch_2}
                delay: 100ms

          # Client-streaming: drains every message the client sends, then
          # replies once with `response` after the client half-closes.
          - id: sum-charges
            method: SumCharges
            client_stream: true
            response:
              total: 42

          # Bidirectional streaming: client_stream + stream together reply
          # to each client message with the correspondingly-indexed stream
          # entry; once `stream` is exhausted, further client messages are
          # drained silently until it half-closes.
          - id: echo-charges
            method: EchoCharges
            client_stream: true
            stream:
              - response: {echo: 1}
              - response: {echo: 2}
```

Any `stream` entry may set `error` instead of `response` to end the call
early with a gRPC status, same shape as the top-level `error` field.

By default, a streaming RPC (server- or bidirectional-streaming) returns
once its `stream` list is exhausted. Set `keep_open: true` on the mock to
leave it open indefinitely instead — including across a bidirectional
call's client half-close — so further ad-hoc messages can be pushed
through the [stream management API](graphql.md#stream-management-api-push-close).

Optionally reference a real `.proto` file (`proto: ./protos/payments.proto`)
to generate mocks from it instead of hand-writing them — see
[Generating mocks from an OpenAPI, AsyncAPI, or Protobuf
spec](../testing/component-testing.md#generating-mocks-from-an-openapi-asyncapi-or-protobuf-spec).
