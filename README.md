# Mockly

**Cross-platform, multi-protocol mock server** — HTTP, WebSocket, and gRPC in a single binary with a built-in web UI and REST management API.

[![CI](https://github.com/dever-labs/mockly/actions/workflows/ci.yml/badge.svg)](https://github.com/dever-labs/mockly/actions/workflows/ci.yml)

---

## Features

| Feature | Details |
|---|---|
| **Protocols** | HTTP, WebSocket, gRPC |
| **Request matching** | Exact path, prefix wildcard (`/api/*`), regex (`re:^/users/\d+$`) |
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
