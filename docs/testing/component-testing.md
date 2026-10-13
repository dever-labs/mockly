# Component Testing

Mockly is designed for **component testing** — testing how your application behaves when a dependency returns errors, timeouts, unexpected data, or edge-case responses. The config file is owned by the dependency team; consuming teams just load it and toggle scenarios.

## Typical workflow

```
dependency-service/
└── mockly/
    ├── mockly.yaml          # base happy-path mocks
    └── scenarios/           # optional split-out scenario files
        ├── auth-down.yaml
        └── payment-timeout.yaml
```

Your test:

```go
// start mockly with the dependency's config
// activate a scenario to simulate a failure
// make requests to your app and assert it handles the error correctly
// call the verification API to confirm your app called the right endpoints
```

## Generating mocks from an OpenAPI, AsyncAPI, or Protobuf spec

If the dependency already publishes an OpenAPI 3.x document, an AsyncAPI
2.x/3.x document, or a Protobuf (`.proto`) service definition, skip
hand-writing the happy-path mocks and generate them instead. `mockly
generate` auto-detects which kind of spec it's looking at (from the file's
top-level `openapi:`/`asyncapi:` field, or a `.proto` extension / `syntax =
"proto2|3";` declaration), so the same command works for all three:

```bash
mockly generate api.yaml -o mockly.yaml
mockly start -c mockly.yaml
```

**OpenAPI** → for every operation (path + method) in the spec, this picks a
representative response (preferring `200`/`201`/`202`/`204`, then any other
`2xx`) and builds its body from:

1. the response's `example` or `examples`, if the spec defines one, or
2. a value synthesised from the response's JSON schema — objects and arrays
   are built recursively, `enum`s use their first value, and strings use a
   format-aware placeholder (`date-time`, `date`, `email`, `uuid`, `uri`,
   `byte` all get a sensible fake value instead of the literal `"string"`).

Path templates carry over as-is: an operation on `/pets/{petId}` becomes a
mock with `path: /pets/{petId}`, which Mockly already matches and captures
into `{{.request.params.petId}}` for response templates.

Operations Mockly couldn't derive a usable response for (no response
defined in the spec, or a binary content-type with no example) are skipped
with a warning printed to stderr — generation still succeeds for the rest.

**AsyncAPI** → channels/operations become Kafka, MQTT, AMQP, NATS and
WebSocket mocks, depending on which protocol(s) the spec's `servers` use.
Not every AsyncAPI direction has a Mockly equivalent:

- **Kafka**: an operation where the application *emits* messages
  (2.x `publish` / 3.x `action: send`) becomes a pre-seeded topic (data a
  consumer can read immediately). The opposite direction (the app
  *consuming* messages) has no static Mockly equivalent and is skipped.
- **MQTT/AMQP/NATS**: an operation where the application *receives*
  messages (2.x `subscribe` / 3.x `action: receive`) becomes a reactive
  listener on that topic/routing-key/subject, optionally replying if the
  operation declares a `reply` message (3.x only). The opposite direction
  (the app spontaneously publishing) has no scheduled/spontaneous publish
  mechanism in Mockly yet and is skipped — use the management API to
  publish a message on demand instead.
- **WebSocket**: a channel's `address` becomes the mock's `path`; a
  `receive` operation becomes an `on_message` rule (replying if a `reply`
  is declared), a `send` operation becomes an `on_connect` push.

Skipped operations print a warning to stderr but don't fail generation, as
long as at least one mock could be produced.

**Protobuf** → for every RPC method in the spec's service(s), this builds a
gRPC mock whose response body is synthesised from the method's output
message type: scalar fields get a type-appropriate placeholder (64-bit
integer kinds become JSON strings, per the standard proto3 JSON mapping),
`repeated`/`map` fields get a single representative entry, nested messages
recurse, and the handful of `google.protobuf.*` well-known types
(`Timestamp`, `Duration`, `Empty`, the wrapper types, ...) get their
JSON-mapped placeholder directly rather than being expanded field-by-field.
Only one member of each real `oneof` is included (proto3 `optional` fields
use a synthetic oneof under the hood and are unaffected).

`import`s are resolved only from the spec file's own directory (no absolute
paths, no `../` escaping it) plus the standard `google/protobuf/*.proto`
well-known types — anything else is a hard error, not a skip, since an
unresolvable type can't be faithfully represented at all. Streaming methods
(client-streaming, server-streaming, bidirectional) are generated as
minimal single-message `stream`/`client_stream` stubs (see
[gRPC](../protocols/grpc.md)) rather than skipped, with a warning noting it's a starting
point meant to be hand-edited into a fuller sequence. Any RPC method whose
name collides with one already seen in a different service in the same
file is skipped with a warning instead (Mockly matches gRPC calls by method
name alone, not service, so a later duplicate would just shadow the first
and never be reachable).

The generated file is a complete, runnable config (management API/UI ports
included), not a fragment — review it, add scenarios/faults/state as
needed, and commit it like any other Mockly config.

### Alternative: reference a spec directly from `mockly.yaml`

`mockly generate` is a one-time snapshot: it turns a spec into a static file
you then own — regenerating overwrites it, so any fault/scenario/mock you
hand-added is lost unless you re-apply it yourself. If you'd rather the spec
stay a live input — e.g. it's still evolving, or you want hand-written
faults/scenarios/extra mocks to automatically keep working against a spec
you don't want to manually resync — reference it inline instead, using the
same `openapi`/`asyncapi`/`proto` key next to `mocks:` in the relevant
protocol block. Every time the config is loaded (`mockly start`,
`mockly apply`, `mockly config validate`), Mockly regenerates mocks from the
spec and layers `mocks:` on top: an entry whose `id` matches a generated
mock overrides it (e.g. to attach a `fault`, pin a different response, or
add `state`); any other `id` is simply appended alongside it. Relative spec
paths are resolved against the config file's own directory, not the current
working directory.

A generated mock's `id` is derived automatically (from the OpenAPI
operation's `operationId`, or the AsyncAPI operation's key, or, for
Protobuf, `<service>-<method>` — always lowercased/slugified, e.g.
`operationId: getUser` → `id: getuser`, `rpc GetUser` on service `Users` →
`id: users-getuser`). Run `mockly generate <spec> -o /tmp/preview.yaml` once
to see the exact IDs a given spec produces before writing overrides for it —
guessing the id wrong just appends a harmless extra mock instead of
overriding anything, so it fails quietly rather than loudly.

```yaml
protocols:
  http:
    enabled: true
    port: 8080
    openapi: ./api.yaml      # generates one HTTP mock per operation
    mocks:
      - id: getuser           # matches operationId "getUser", slugified -> overrides it
        request: { method: GET, path: "/users/{id}" }
        response: { status: 200, body: '{"id":"{{.request.params.id}}","name":"Alice"}' }
        fault: { delay: 200ms, error_rate: 0.1 }
      - id: admin-only-route  # no spec counterpart -> just an extra mock
        request: { method: GET, path: "/admin/ping" }
        response: { status: 200, body: "pong" }

  kafka:
    enabled: true
    asyncapi: ./events.yaml  # same spec can be referenced by more than one protocol block
    mocks: []

  grpc:
    enabled: true
    port: 50051
    services:
      - proto: ./protos/users.proto
        mocks:
          - id: users-getuser  # "<service>-<method>", slugified -> overrides it
            method: GetUser
            response: { id: "1", name: Alice }
```

Both approaches use the exact same generators under the hood and can be
mixed across protocols in the same file — use whichever fits each spec:
`generate` once for a spec you want to fully own and hand-edit from that
point on, an inline reference for one you want to keep regenerating from on
every start.

## Record mode (bootstrap mocks from a real backend)

Hand-writing every mock is the biggest upfront cost of adopting a mock server. Record mode removes it: point Mockly at a real upstream, and any request that doesn't match an existing mock is transparently proxied there — the real response is both returned to the caller and saved as a new mock, so every subsequent identical request is replayed locally without hitting the upstream again.

```yaml
mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8080
    record:
      enabled: true
      target: https://api.example.com   # real upstream to proxy unmatched requests to
      save_to: recorded-mocks.yaml      # optional: persist captured mocks for review
    mocks: []                           # hand-written mocks still take precedence
```

```sh
mockly start -c mockly.yaml

# Run your test suite (or click around manually) against the real backend,
# through Mockly, once — every response it returns gets captured.
curl http://localhost:8080/v1/users/42

# Recorded mocks are immediately visible like any other mock...
curl http://localhost:9091/api/mocks/http

# ...and, if save_to was set, written to disk so you can review and fold
# them into your hand-written config:
cat recorded-mocks.yaml
```

A few things worth knowing:
- Hand-written mocks always take precedence — record mode only fires for requests that don't match anything already configured, so it's safe to mix recorded and hand-written mocks in the same file.
- If the upstream is unreachable, Mockly falls back to the normal 404 "no mock matched" response rather than failing the request outright.
- `save_to` is rewritten (not appended) after every new capture, and only ever contains recorded mocks — it won't clobber your hand-written config file.

## Call verification

Check how many times your app hit a mock — without log scraping:

```sh
# How many times was POST /token called?
curl http://localhost:9091/api/calls/http/token-endpoint

# Block until the mock has been called at least 3 times (timeout 5s)
curl -X POST http://localhost:9091/api/calls/http/token-endpoint/wait \
  -H 'Content-Type: application/json' \
  -d '{"count":3,"timeout":"5s"}'

# Reset call counters for one mock
curl -X DELETE http://localhost:9091/api/calls/http/token-endpoint

# Reset all call counters
curl -X DELETE http://localhost:9091/api/calls/http
```

Response from `GET /api/calls/http/{mockId}`:

```json
{
  "mock_id": "token-endpoint",
  "count": 3,
  "calls": [
    {"id": "...", "mock_id": "token-endpoint", "method": "POST", "path": "/token", "timestamp": "..."}
  ]
}
```

## Full component-test example

```yaml
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      # Happy path
      - id: get-token
        request:
          method: POST
          path: /oauth/token
          body_json:
            grant_type: client_credentials
        response:
          status: 200
          body: '{"access_token":"{{uuid}}","expires_in":3600}'

      # Sequence: simulate token refresh after expiry
      - id: get-token-expiry-flow
        request:
          method: POST
          path: /oauth/token
          body_json:
            grant_type: refresh_token
        sequence:
          - status: 401
            body: '{"error":"token_expired"}'
          - status: 200
            body: '{"access_token":"new-token","expires_in":3600}'
        sequence_exhausted: hold_last
        response:
          status: 200
          body: '{"access_token":"new-token","expires_in":3600}'

scenarios:
  - id: auth-service-down
    name: Auth Service Down
    patches:
      - mock_id: get-token
        status: 503
        body: '{"error":"service unavailable"}'

  - id: rate-limited
    name: Rate Limited
    patches:
      - mock_id: get-token
        status: 429
        body: '{"error":"too_many_requests"}'
```

---
