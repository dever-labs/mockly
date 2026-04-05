# Mockly

**Cross-platform, multi-protocol mock server** — HTTP, WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, and MQTT in a single binary with a built-in web UI, REST management API, scenario system, and fault injection.

[![CI](https://github.com/dever-labs/mockly/actions/workflows/ci.yml/badge.svg)](https://github.com/dever-labs/mockly/actions/workflows/ci.yml)

---

## Features

| Feature | Details |
|---|---|
| **Protocols** | HTTP, WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, MQTT |
| **Request matching** | Exact path/key, prefix wildcard (`/api/*`), regex (`re:^/users/\d+$`) |
| **Response control** | Status code, headers, body, artificial delay |
| **Template responses** | Go template syntax in response bodies (`{{now}}`, `{{.body}}`, etc.) |
| **State conditions** | Fire a mock only when a runtime state variable matches |
| **Scenarios** | Named sets of mock patches — activate/deactivate atomically via API or CLI |
| **Fault injection** | Global delay, status override, and probabilistic error rate |
| **PATCH mocks** | Change only specific response fields at runtime without replacing the whole mock |
| **Preset configs** | Drop-in YAML configs for Keycloak, Authelia, OAuth2, GitHub, Stripe, OpenAI, Slack, Twilio, SendGrid |
| **Web UI** | Served from the binary itself — no separate install |
| **Management API** | 40+ REST endpoints covering all protocols, scenarios, fault, state, and logs |
| **Live request log** | SSE-streamed in real time to the UI |
| **CI-friendly** | Zero dependencies, single binary, YAML config |

---

## Quickstart

### Download

Grab the binary for your platform from the [releases page](https://github.com/dever-labs/mockly/releases), or build from source:

```sh
git clone https://github.com/dever-labs/mockly
cd mockly
make build        # builds UI + Go binary
```

### Run with a config file

```sh
mockly start --config mockly.yaml
```

Open `http://localhost:9091` for the web UI, or call the management API at the same port.

### Run a preset

```sh
mockly preset use keycloak    # starts Mockly pre-loaded with Keycloak endpoints
mockly preset list            # list all available presets
mockly preset show stripe     # print the preset YAML
```

---

## Configuration

Mockly is driven by a YAML config file. Every section is optional.

```yaml
mockly:
  api:
    port: 9091   # Management API + Web UI port (default: 9091)

protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: list-users
        request:
          method: GET
          path: /api/users
        response:
          status: 200
          headers:
            Content-Type: application/json
          body: '[{"id":1,"name":"Alice"}]'
          delay: 50ms

  websocket:
    enabled: true
    port: 8081
    mocks:
      - id: echo
        path: /ws/echo
        on_message:
          match: ping
          respond: pong

  grpc:
    enabled: true
    port: 50051
    services:
      - name: users
        mocks:
          - id: get-user
            method: GetUser
            response:
              body: '{"id":"1","name":"Alice"}'

  graphql:
    enabled: true
    port: 8082
    path: /graphql
    mocks:
      - id: get-user
        operation_type: query
        operation_name: GetUser
        response:
          user:
            id: "1"
            name: Alice

  tcp:
    enabled: true
    port: 8083
    mocks:
      - id: hello
        pattern: "HELLO"
        response: "WORLD\n"

  redis:
    enabled: true
    port: 6379
    mocks:
      - id: get-session
        command: GET
        key: "session:*"
        response:
          type: bulk
          value: '{"userId":"abc"}'

  smtp:
    enabled: true
    port: 2525
    domain: mockly.local
    rules:
      - id: accept-all
        action: accept

  mqtt:
    enabled: true
    port: 1883
    mocks:
      - id: sensor-ack
        topic: "sensors/+"
        response:
          topic: "sensors/ack"
          payload: '{"ok":true}'

scenarios:
  - id: auth-down
    name: Auth Service Down
    description: Simulate auth outage — all token endpoints return 503
    patches:
      - mock_id: list-users
        status: 503
        body: '{"error":"auth unavailable"}'
```

### Path matching

| Pattern | Matches |
|---|---|
| `/api/users` | Exact match |
| `/api/*` | Any path starting with `/api/` |
| `re:^/users/\d+$` | Regex — any `/users/<number>` |

### Template responses

Response bodies are rendered as Go templates. Built-in functions:

| Function | Description |
|---|---|
| `{{now}}` | Current UTC time in RFC3339 |
| `{{uuid}}` | Random UUID |
| `{{.body}}` | Incoming request body |
| `{{state "key"}}` | Value from runtime state store |

---

## Protocols

### HTTP

Full HTTP mock server. Matching on method + path (exact/wildcard/regex), optional header and body match, optional state condition.

```yaml
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: create-user
        request:
          method: POST
          path: /users
          headers:
            Authorization: "Bearer *"
        response:
          status: 201
          body: '{"id":"{{uuid}}"}'
          headers:
            Content-Type: application/json
          delay: 20ms
```

### WebSocket

```yaml
protocols:
  websocket:
    enabled: true
    port: 8081
    mocks:
      - id: ticker
        path: /ws/ticker
        on_connect:
          send: '{"type":"connected"}'
        on_message:
          match: subscribe
          respond: '{"type":"tick","price":42.0}'
```

### gRPC

Dynamic gRPC mocking — no compiled `.proto` files needed. Uses a raw codec to intercept any service/method call.

```yaml
protocols:
  grpc:
    enabled: true
    port: 50051
    services:
      - name: payments
        mocks:
          - id: charge
            method: Charge
            response:
              body: '{"success":true,"charge_id":"ch_123"}'
```

### GraphQL

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

### TCP

Raw TCP mock server. Matches incoming data as exact string, prefix wildcard, or regex. Supports hex encoding for binary protocols.

```yaml
protocols:
  tcp:
    enabled: true
    port: 8083
    mocks:
      - id: ping
        pattern: "PING\r\n"
        response: "+PONG\r\n"
      - id: hex-response
        pattern: "re:^\\x02.*\\x03$"
        response_hex: "060000"
```

### Redis

RESP-protocol Redis mock. Intercepts any Redis command and returns a configurable response.

```yaml
protocols:
  redis:
    enabled: true
    port: 6379
    mocks:
      - id: auth
        command: AUTH
        response:
          type: string     # string | bulk | integer | null | error | array
          value: "OK"
      - id: get-token
        command: GET
        key: "token:*"
        response:
          type: bulk
          value: "abc123"
          delay: 5ms
```

### SMTP

SMTP server that captures emails and applies accept/reject rules.

```yaml
protocols:
  smtp:
    enabled: true
    port: 2525
    domain: mockly.local
    rules:
      - id: reject-spam
        from: "*@spam.example.com"
        action: reject
        message: "550 spam not accepted"
      - id: accept-all
        action: accept
```

Captured emails are visible at `GET /api/emails`.

### MQTT

Full MQTT v3/v4/v5 broker (powered by mochi-mqtt). Configurable topic pattern matching with automatic response publishing.

```yaml
protocols:
  mqtt:
    enabled: true
    port: 1883
    mocks:
      - id: command-ack
        topic: "devices/+/command"
        response:
          topic: "devices/+/ack"
          payload: '{"status":"ok"}'
          qos: 1
```

Topic wildcards: `+` matches a single segment, `#` matches everything below.

---

## Scenarios

Scenarios let you pre-define named mock overrides and activate/deactivate them at any time — great for toggling between happy path and failure modes during testing.

### Define in config

```yaml
scenarios:
  - id: payment-timeout
    name: Payment Gateway Timeout
    patches:
      - mock_id: charge
        status: 504
        body: '{"error":"timeout"}'
        delay: 5s
      - mock_id: refund
        disabled: true   # Removes this endpoint entirely
```

### Control via CLI

```sh
mockly scenario list
mockly scenario activate payment-timeout
mockly scenario deactivate payment-timeout
```

### Control via API

```sh
# Activate
curl -X POST http://localhost:9091/api/scenarios/payment-timeout/activate

# Deactivate
curl -X DELETE http://localhost:9091/api/scenarios/payment-timeout/activate

# List active
curl http://localhost:9091/api/scenarios/active
```

---

## Fault Injection

Inject global faults to test your application's resilience — without touching individual mocks.

```sh
# Inject 100ms latency on all requests
mockly fault set --delay 100ms

# Make 30% of requests return 503
mockly fault set --status 503 --rate 0.3

# Always return 429 (rate limit)
mockly fault set --status 429

# Remove the fault
mockly fault clear
```

Or via API:

```sh
curl -X POST http://localhost:9091/api/fault \
  -H 'Content-Type: application/json' \
  -d '{"enabled":true,"delay":"100ms","status_override":503,"error_rate":0.3}'

curl -X DELETE http://localhost:9091/api/fault
```

Fault fields:

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Master switch |
| `delay` | duration | Delay added to every request before matching |
| `status_override` | int | Replace response status after matching |
| `error_rate` | float | Probability (0–1) that the override fires; 0 = always |

---

## PATCH Mocks

Change individual fields of an existing mock without replacing it entirely:

```sh
curl -X PATCH http://localhost:9091/api/mocks/http/charge \
  -H 'Content-Type: application/json' \
  -d '{"response":{"status":500,"body":"{\"error\":\"internal\"}"}}'
```

---

## Preset Configs

Mockly ships with pre-built YAML configs for common services:

| Preset | Description |
|---|---|
| `keycloak` | Token endpoint, JWKS, userinfo, introspection |
| `authelia` | Auth verify, session endpoints |
| `oauth2` | Generic OAuth2 flows (authorize, token, revoke) |
| `github` | REST API: repos, issues, pull requests |
| `stripe` | Charges, refunds, customers, payment intents |
| `openai` | Chat completions, embeddings, models |
| `slack` | Messages, channels, users, reactions |
| `twilio` | SMS, calls, lookup |
| `sendgrid` | Email send, templates, contacts |

Each preset also includes built-in scenarios for common failure modes (e.g. `keycloak-unauthorized`, `stripe-card-declined`).

### Use a preset

```sh
mockly preset use keycloak
```

### Import a preset into your own config

```sh
mockly preset show keycloak > keycloak.yaml
# Edit keycloak.yaml, then:
mockly start --config keycloak.yaml
```

---

## CLI Reference

```
mockly start       [--config <file>] [--http-port <n>] [--api-port <n>]
mockly apply       --config <file>
mockly list
mockly add http    --method GET --path /foo --status 200 --body '{"ok":true}'
mockly delete      <mock-id>
mockly status
mockly reset
mockly preset      list | show <name> | use <name>
mockly scenario    list | show <id> | activate <id> | deactivate <id>
mockly fault       set [--delay <d>] [--status <n>] [--rate <f>] | clear | show
```

---

## Management API Reference

Base URL: `http://localhost:9091`

### Protocols

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/protocols` | List all protocol statuses |
| `GET` | `/api/health` | Health check |

### HTTP Mocks

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/mocks/http` | List HTTP mocks |
| `POST` | `/api/mocks/http` | Create HTTP mock |
| `PUT` | `/api/mocks/http/{id}` | Replace HTTP mock |
| `PATCH` | `/api/mocks/http/{id}` | Partial update HTTP mock |
| `DELETE` | `/api/mocks/http/{id}` | Delete HTTP mock |

Similarly for WebSocket (`/api/mocks/websocket`), gRPC (`/api/mocks/grpc`), GraphQL (`/api/mocks/graphql`), TCP (`/api/mocks/tcp`), Redis (`/api/mocks/redis`), SMTP (`/api/mocks/smtp`), MQTT (`/api/mocks/mqtt`).

### Email inbox (SMTP)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/emails` | List captured emails |
| `DELETE` | `/api/emails` | Clear inbox |

### MQTT messages

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/mqtt/messages` | List captured MQTT messages |
| `DELETE` | `/api/mqtt/messages` | Clear message store |

### Scenarios

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/scenarios` | List all scenarios |
| `POST` | `/api/scenarios` | Create scenario |
| `GET` | `/api/scenarios/active` | List active scenarios |
| `GET` | `/api/scenarios/{id}` | Get scenario |
| `PUT` | `/api/scenarios/{id}` | Replace scenario |
| `DELETE` | `/api/scenarios/{id}` | Delete scenario |
| `POST` | `/api/scenarios/{id}/activate` | Activate scenario |
| `DELETE` | `/api/scenarios/{id}/activate` | Deactivate scenario |

### Fault Injection

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/fault` | Get current fault config |
| `POST` | `/api/fault` | Set fault config |
| `DELETE` | `/api/fault` | Clear fault |

### State Store

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/state` | Get all state keys |
| `POST` | `/api/state` | Set state keys (JSON object) |
| `DELETE` | `/api/state/{key}` | Delete a state key |

### Logs

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/logs` | Get recent log entries |
| `DELETE` | `/api/logs` | Clear logs |
| `GET` | `/api/logs/stream` | SSE stream of live log entries |

### Reset

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/reset` | Reset all mocks/state/logs/fault/scenarios to config defaults |

---

## CI Integration

Mockly is a single static binary with no runtime dependencies — ideal for CI.

### GitHub Actions (composite action)

```yaml
steps:
  - uses: actions/checkout@v5

  - name: Start Mockly
    uses: dever-labs/mockly/.github/actions/setup-mockly@v0.1.0
    with:
      version: v0.1.0          # pin to a specific version
      config: mockly.yaml      # path to your config
      api-port: 9090           # management API port (default)

  - name: Run tests
    run: npm test
```

The action automatically:
- Downloads the right binary for the runner OS/arch
- Starts mockly in the background
- Waits up to 30 s for the server to be ready
- Kills the process after the job completes

---

### GitLab CI

Include the template and extend the `.mockly-start` job:

```yaml
include:
  - remote: 'https://raw.githubusercontent.com/dever-labs/mockly/main/.gitlab/mockly.yml'

integration-tests:
  extends: .mockly-start
  variables:
    MOCKLY_VERSION: "v0.1.0"
    MOCKLY_CONFIG: "mockly.yaml"
  script:
    - ./run-tests.sh
```

Or run it as a Docker service (no binary install needed):

```yaml
integration-tests:
  image: alpine:3.21
  services:
    - name: ghcr.io/dever-labs/mockly:latest
      alias: mockly
      variables:
        # mount config via CI artifacts or inline
  variables:
    MOCKLY_URL: http://mockly:9090
  script:
    - apk add --no-cache curl
    - curl "$MOCKLY_URL/api/protocols"
    - ./run-tests.sh
```

---

### Any CI (install script)

```sh
# Install latest release
curl -sSfL https://raw.githubusercontent.com/dever-labs/mockly/main/install.sh | bash

# Or pin to a version
MOCKLY_VERSION=v0.1.0 \
  curl -sSfL https://raw.githubusercontent.com/dever-labs/mockly/main/install.sh | bash

# Start in background and wait for ready
mockly start -c mockly.yaml &
until curl -sf http://localhost:9090/api/protocols; do sleep 1; done
```

---

### Docker

```sh
# Run with your local config
docker run --rm \
  -v "$PWD/mockly.yaml:/config/mockly.yaml:ro" \
  -p 8080:8080 -p 9090:9090 \
  ghcr.io/dever-labs/mockly:latest

# Or with docker compose
docker compose up
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                            Single Binary                                  │
│                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │                     Management API + Web UI  :9091                  │ │
│  │  CRUD mocks/rules  ·  scenarios  ·  fault  ·  state  ·  logs/SSE  │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                           │
│  ┌──────┐ ┌─────────┐ ┌──────┐ ┌─────────┐ ┌─────┐ ┌───────┐ ┌──────┐ ┌──────┐ │
│  │ HTTP │ │WebSocket│ │ gRPC │ │GraphQL  │ │ TCP │ │ Redis │ │ SMTP │ │ MQTT │ │
│  │:8080 │ │ :8081   │ │:50051│ │ :8082   │ │:8083│ │ :6379 │ │:2525 │ │:1883 │ │
│  └──────┘ └─────────┘ └──────┘ └─────────┘ └─────┘ └───────┘ └──────┘ └──────┘ │
│                                                                           │
│  Shared:  State Store  ·  Request Logger  ·  Scenario Store               │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Development

```sh
make build        # build UI + Go binary
make test         # run unit + integration tests
make test-e2e     # run e2e tests (builds binary first)
make lint         # run golangci-lint
make dev          # hot-reload with air
```

---

## License

MIT

| **Response control** | Status code, headers, body, artificial delay |
| **Template responses** | Go template syntax in response bodies (`{{now}}`, `{{.body}}`, etc.) |
| **State conditions** | Fire a mock only when a runtime state variable matches |
| **Web UI** | Served from the binary itself — no separate install |
| **Management API** | Full CRUD over mocks, state, and logs via REST |
| **Live request log** | SSE-streamed in real time to the UI |
| **CI-friendly** | Zero dependencies, single binary, YAML config |

---

## Quickstart

### Download

Grab the binary for your platform from the [releases page](https://github.com/dever-labs/mockly/releases), or build from source:

```bash
# Clone and build
git clone https://github.com/dever-labs/mockly
cd mockly

# Build UI first, then embed into Go binary
npm --prefix ui ci
npm --prefix ui run build
go build -o mockly ./cmd/mockly
```

### Start

```bash
# Start with an example config
mockly start --config configs/example.yaml

# → HTTP mock server  on :8080
# → WebSocket server  on :8081
# → gRPC server       on :50051
# → Management API    on http://localhost:9091/api
# → Web UI            on http://localhost:9091
```

Open **http://localhost:9091** to access the web UI.

---

## Configuration (YAML)

```yaml
mockly:
  ui:
    enabled: true
    port: 9090         # UI port (served on same server as API)
  api:
    port: 9091         # Management API port

protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: get-users
        request:
          method: GET
          path: /api/users
        response:
          status: 200
          headers:
            Content-Type: application/json
          body: '[{"id":1,"name":"Alice"}]'
          delay: 50ms        # Optional artificial delay

      # Regex path matching
      - id: get-user
        request:
          method: GET
          path: re:^/api/users/\d+$
        response:
          status: 200
          body: '{"id":1,"name":"Alice"}'

      # State-conditional mock
      - id: authenticated-profile
        request:
          method: GET
          path: /api/me
        state:
          key: authenticated
          value: "true"
        response:
          status: 200
          body: '{"user":"alice"}'

  websocket:
    enabled: true
    port: 8081
    mocks:
      - id: echo
        path: /ws/echo
        on_connect:
          send: '{"type":"connected"}'
        on_message:
          - match: ping
            respond: pong
          - match: re:^bye
            close: true

  grpc:
    enabled: true
    port: 50051
    services:
      - proto: ./protos/users.proto
        mocks:
          - id: get-user
            method: GetUser
            response:
              id: "1"
              name: Alice
          - id: delete-error
            method: DeleteUser
            error:
              code: 7           # PERMISSION_DENIED
              message: not allowed
```

See [`configs/example.yaml`](configs/example.yaml) for a full example.

---

## CLI Reference

```
mockly start [flags]           Start all configured servers
  -c, --config string          Config file path (default "mockly.yaml")
      --ui-port int            Override UI/API port
      --api-port int           Override management API port

mockly apply                   Apply a config file to a running instance
  -f, --config string

mockly list                    List active HTTP mocks (JSON)

mockly add http [flags]        Add an HTTP mock at runtime
      --id string              Mock ID (auto-generated if empty)
      --method string          HTTP method (default "GET")
      --path string            URL path (default "/")
      --status string          Status code (default "200")
      --body string            Response body
      --delay string           Artificial delay (e.g. 100ms)

mockly delete <id> [flags]     Delete a mock
      --protocol string        Protocol: http, websocket, grpc (default "http")

mockly status                  Show protocol server status

mockly reset                   Reset all state and clear logs
```

---

## Management API

Base URL: `http://localhost:9091`

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/protocols` | List all protocol statuses |
| `GET` | `/api/mocks/http` | List HTTP mocks |
| `POST` | `/api/mocks/http` | Create HTTP mock |
| `PUT` | `/api/mocks/http/:id` | Update HTTP mock |
| `DELETE` | `/api/mocks/http/:id` | Delete HTTP mock |
| `GET` | `/api/mocks/websocket` | List WebSocket mocks |
| `POST` | `/api/mocks/websocket` | Create WebSocket mock |
| `GET` | `/api/mocks/grpc` | List gRPC mocks |
| `POST` | `/api/mocks/grpc` | Create gRPC mock |
| `GET` | `/api/state` | Get all state variables |
| `POST` | `/api/state` | Set state variables |
| `DELETE` | `/api/state/:key` | Delete a state key |
| `GET` | `/api/logs` | Get request log |
| `GET` | `/api/logs/stream` | SSE stream of live logs |
| `DELETE` | `/api/logs` | Clear logs |
| `POST` | `/api/reset` | Reset state and clear logs |

### Example: Add a mock at runtime

```bash
curl -X POST http://localhost:9091/api/mocks/http \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "my-mock",
    "request": { "method": "GET", "path": "/api/hello" },
    "response": { "status": 200, "body": "{\"hello\":\"world\"}" }
  }'
```

---

## Using in CI

Mockly starts as a background process in your CI pipeline and requires no external dependencies:

```yaml
# GitHub Actions example
- name: Start Mockly
  run: |
    ./mockly start --config mockly.yaml &
    sleep 1   # wait for servers to be ready

- name: Run tests
  run: npm test
```

---

## Request Path Matching

| Pattern | Example | Matches |
|---|---|---|
| Exact | `/api/users` | Only `/api/users` |
| Prefix wildcard | `/api/*` | `/api/users`, `/api/users/1`, ... |
| Regex | `re:^/users/\d+$` | `/users/42`, not `/users/abc` |
| Wildcard | `*` | Any path |

---

## Template Responses

Response bodies support Go template syntax:

```yaml
response:
  body: '{"time":"{{now}}","method":"{{.headers.X-Request-Id}}"}'
```

Available functions: `now`, `upper`, `lower`.

---

## Development

```bash
# Run tests
go test ./internal/... -v

# Build UI (outputs to assets/dist/)
npm --prefix ui ci && npm --prefix ui run build

# Build binary
go build -o mockly ./cmd/mockly

# Or use Make
make build
make test
```
