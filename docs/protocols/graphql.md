# GraphQL

HTTP-based GraphQL mock. Handles `POST /graphql` with `application/json` and `application/graphql` content types, plus `GET` requests with a `query` parameter. Introspection queries return an empty schema.

```yaml
protocols:
  graphql:
    enabled: true
    port: 8082
    path: /graphql
    mocks:
      - id: create-post
        operation_type: mutation
        operation_name: CreatePost
        response:
          createPost:
            id: "{{uuid}}"
            title: Hello
        errors: []
```

**Subscriptions** are served over a `graphql-transport-ws` WebSocket connection on the same path/port (connect with `Sec-WebSocket-Protocol: graphql-transport-ws`, e.g. via the `graphql-ws` npm client or Apollo Client's `GraphQLWsLink`). Give the mock a `stream` list instead of `response`: each entry is pushed as a `next` message in order (respecting its own optional `delay`), then the subscription ends with a `complete` message.

```yaml
      - id: order-updates-sub
        operation_type: subscription
        operation_name: onOrderUpdate
        stream:
          - data: '{"data":{"onOrderUpdate":{"status":"pending"}}}'
            delay: 200ms
          - data: '{"data":{"onOrderUpdate":{"status":"shipped"}}}'
            delay: 1s
```

Each `data` string is rendered as a Go template (same `{{ }}` syntax as `response`/`createPost.id` above) and must render to valid JSON — it's sent verbatim as the GraphQL execution result (`{"data": ...}`), not wrapped again. Requesting a `stream` mock over plain HTTP (no WebSocket upgrade) returns a GraphQL error telling the client to connect over WebSocket instead, since a single HTTP response can't carry multiple pushed messages. The legacy `subscriptions-transport-ws` protocol (`graphql-ws` subprotocol, different message shapes) isn't implemented.

Subscriptions stay open indefinitely once their `stream` list is
exhausted (they never auto-`complete`), so further ad-hoc `next` messages
can be delivered through the [stream management
API](#stream-management-api-push-close), which can also explicitly end a
subscription with `complete` or `error`.

## Stream management API (push / close)

HTTP streams with `stream.keep_open: true`, gRPC streaming calls with
`keep_open: true`, and GraphQL subscriptions (always) all stay open after
their configured `stream`/`events` list runs out, letting you push further
ad-hoc data into them — or close them, optionally as an error — through
three management API endpoints, in addition to whatever that mock's YAML
already sends. This is useful for driving dynamic/event-based test
scenarios (e.g. "fire this stock price update now") that don't fit a
purely static, pre-scheduled YAML event list.

```
GET  /api/streams                 list every currently open streaming connection
POST /api/streams/push            deliver one more ad-hoc event
POST /api/streams/close           end a connection, optionally as an error
```

`GET /api/streams` returns an array of `{id, protocol, mock_id, started_at}`.
`POST /api/streams/push` and `POST /api/streams/close` take a JSON body
targeting either one specific connection (`conn_id`, from the list above)
or every open connection currently serving a mock (`mock_id`, fanning out
to all of them):

```jsonc
// POST /api/streams/push
{"protocol": "http", "mock_id": "live-ticker", "data": "{\"price\":101}"}

// POST /api/streams/close
{"protocol": "graphql", "conn_id": "graphql-0xc0001...", "error": ""}
```

`protocol` is `http`, `grpc`, or `graphql`. `data`'s shape matches each
protocol's configured event payload: a raw template-free string for HTTP
(SSE-framed automatically when the response is `text/event-stream`), valid
JSON for GraphQL (the full `{"data": ...}` execution result, sent
verbatim), or a JSON object for gRPC (matching `stream[].response`'s
shape). `error` on close is optional; when non-empty the connection ends
as an error (an SSE `error` event for HTTP, a `codes.Aborted` status for
gRPC, a graphql-transport-ws `error` message for GraphQL) instead of
ending normally.
