# Mockly

**Cross-platform, multi-protocol mock server** — HTTP, WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, MQTT, SNMP, DNS, AMQP, Kafka, LDAP, IMAP, FTP, Memcached, STOMP, CoAP, and SIP in a single binary with a built-in web UI, REST management API, scenario system, and fault injection.

[![CI](https://github.com/dever-labs/mockly/actions/workflows/ci.yml/badge.svg)](https://github.com/dever-labs/mockly/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/dever-labs/mockly)](https://github.com/dever-labs/mockly/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Table of contents

- [Features](#features)
- [Quickstart](#quickstart)
- [Configuration](#configuration)
- [Protocols](#protocols)
- [Component Testing](#component-testing)
- [Scenarios](#scenarios)
- [Fault Injection](#fault-injection)
- [PATCH Mocks](#patch-mocks)
- [Preset Configs](#preset-configs)
- [CLI Reference](#cli-reference)
- [Management API Reference](#management-api-reference)
- [Client Libraries](#client-libraries)
- [CI Integration](#ci-integration)
- [Architecture](#architecture)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## Features

| Feature | Details |
|---|---|
| **Protocols** | HTTP, WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, MQTT, SNMP, DNS, AMQP, Kafka, LDAP, IMAP, FTP, Memcached, STOMP, CoAP, SIP |
| **Request matching** | Method + path (exact / wildcard / regex), headers, query params, JSON body fields |
| **Response sequences** | Return a different response on each successive call — loop, hold last, or 404 when exhausted |
| **Response control** | Status code, headers, body, artificial delay |
| **Template responses** | Go template syntax in response bodies and headers (`{{now}}`, `{{.query.foo}}`, `{{.body}}`, etc.) |
| **State conditions** | Fire a mock only when a runtime state variable matches |
| **Scenarios** | Named sets of mock patches — activate/deactivate atomically via API or CLI |
| **Per-protocol fault injection** | Each protocol exposes its own native fault fields (DNS rcode, gRPC status code, Kafka error code, etc.) — activate via API or bundled inside a scenario |
| **Per-mock fault injection** | Fault fields on individual HTTP mocks with independent delay, status/body override, and error rate |
| **Call verification** | Track how many times each mock was hit; block until an expected count is reached |
| **PATCH mocks** | Change only specific response fields at runtime without replacing the whole mock |
| **Preset configs** | Drop-in YAML configs for Keycloak, Authelia, OAuth2, GitHub, Stripe, OpenAI, Slack, Twilio, SendGrid |
| **Web UI** | Served from the binary itself — no separate install |
| **Management API** | 40+ REST endpoints covering all protocols, scenarios, fault, state, logs, and call counts |
| **Live request log** | SSE-streamed in real time to the UI |
| **CI-friendly** | Zero dependencies, single binary, YAML config, Docker image |

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
    # cors:      # CORS for the management API. Defaults to wide-open ("*").
    #   enabled: true                          # Set false to disable CORS headers entirely
    #   allowed_origins: ["http://localhost:3000"]
    #   allowed_methods: ["GET","POST","PUT","DELETE","OPTIONS"]
    #   allowed_headers: ["Content-Type","Authorization"]

protocols:
  http:
    enabled: true
    port: 8080
    # max_body_bytes: 10485760  # Request body size limit in bytes (0 = unlimited, default)
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
          - match: ping
            respond: pong

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
        match: "HELLO"
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

Response bodies **and response headers** are rendered as Go templates. Built-in functions:

| Function | Example | Description |
|---|---|---|
| `{{now}}` | `2024-01-15T10:30:00Z` | Current UTC time (RFC3339) |
| `{{date "2006-01-02"}}` | `2024-01-15` | Current date in Go format |
| `{{date_add "2006-01-02" "-7d"}}` | `2024-01-08` | Date with duration offset |
| `{{uuid}}` | `550e8400-e29b-41d4-a716-446655440000` | Random UUID v4 |
| `{{rand_int 1 100}}` | `42` | Random integer in [min, max] |
| `{{rand_float 0.0 1.0 2}}` | `0.73` | Random float with N decimal places |
| `{{rand_string 8}}` | `aB3xKp7m` | Random alphanumeric string |
| `{{rand_string 8 "hex"}}` | `3f9a1c2b` | Charset: `alpha`, `lower`, `upper`, `numeric`, `hex`, `alphanumeric`, or custom |
| `{{rand_bool}}` | `true` | Random boolean |
| `{{pick "a" "b" "c"}}` | `b` | Randomly pick one of the given values |
| `{{fake "name"}}` | `Alice Smith` | Fake full name |
| `{{fake "email"}}` | `alice.smith@example.com` | Fake email |
| `{{fake "phone"}}` | `+1-555-0142` | Fake phone number |
| `{{fake "company"}}` | `Apex Labs` | Fake company name |
| `{{fake "city"}}` | `Berlin` | Fake city |
| `{{fake "country"}}` | `Germany` | Fake country |
| `{{fake "street"}}` | `42 Main St` | Fake street address |
| `{{fake "zip"}}` | `10115` | Fake postal code |
| `{{fake "ip"}}` | `192.168.1.42` | Fake IPv4 |
| `{{fake "ipv6"}}` | `2001:db8::1a2b:3c4d` | Fake IPv6 |
| `{{fake "url"}}` | `https://apex.io/api/lorem` | Fake URL |
| `{{fake "username"}}` | `alice42` | Fake username |
| `{{fake "useragent"}}` | `Mozilla/5.0 …` | Random User-Agent string |
| `{{fake "word"}}` | `lorem` | Single lorem ipsum word |
| `{{fake "sentence"}}` | `lorem ipsum dolor sit amet` | Short lorem ipsum phrase |
| `{{seq "counter"}}` | `1`, `2`, `3`, … | Auto-incrementing integer per named counter |
| `{{lorem 5}}` | `lorem ipsum dolor sit amet` | N lorem ipsum words |
| `{{upper "hello"}}` | `HELLO` | Uppercase string |
| `{{lower "WORLD"}}` | `world` | Lowercase string |
| `{{.body}}` | *(request body)* | Incoming request body |
| `{{.headers.X-Foo}}` | *(header value)* | Incoming request header |
| `{{.query.foo}}` | *(query value)* | Incoming request query parameter |
| `{{state "key"}}` | *(state value)* | Value from runtime state store |

**Sequence counters** (`{{seq "name"}}`) are reset to zero by `POST /api/reset` or `mockly reset`.

Example — generate a realistic user object on every request:

```yaml
response:
  status: 200
  headers:
    Content-Type: application/json
  body: |
    {
      "id": "{{uuid}}",
      "name": "{{fake "name"}}",
      "email": "{{fake "email"}}",
      "role": "{{pick "user" "admin" "viewer"}}",
      "score": {{rand_float 0 100 1}},
      "created_at": "{{now}}"
    }
```

---

## Protocols

### HTTP

Full HTTP mock server. Matching on method + path (exact/wildcard/regex), optional query params, header match, JSON body field match, and state condition.

```yaml
protocols:
  http:
    enabled: true
    port: 8080
    max_body_bytes: 10485760  # optional: limit request body size (bytes); 0 = unlimited (default)
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

#### Query parameter matching

```yaml
      - id: admin-users
        request:
          method: GET
          path: /users
          query:
            role: admin          # exact match
            page: "*"            # any value (wildcard)
        response:
          status: 200
          body: '[{"id":1,"role":"admin"}]'
```

#### JSON body field matching

Use dot-notation paths to match fields anywhere in a JSON body:

```yaml
      - id: gbp-payment
        request:
          method: POST
          path: /payments
          body_json:
            currency: GBP        # exact
            "user.tier": premium # nested: {"user":{"tier":"premium"}}
            "items.0.sku": "*"   # any SKU (wildcard)
        response:
          status: 200
          body: '{"ok":true}'
```

#### Response sequences

Return a different response on each successive call. Useful for simulating transient errors or pagination.

```yaml
      - id: flaky-service
        request:
          method: GET
          path: /data
        sequence:
          - status: 503
            body: '{"error":"unavailable"}'
          - status: 503
            body: '{"error":"unavailable"}'
          - status: 200
            body: '{"data":"ok"}'
        sequence_exhausted: hold_last   # hold_last (default) | loop | not_found
        response:
          status: 200
          body: '{"data":"ok"}'
```

| `sequence_exhausted` | Behaviour after all entries are consumed |
|---|---|
| `hold_last` | Keep returning the last entry (default) |
| `loop` | Restart from the first entry |
| `not_found` | Return 404 |

#### Per-mock fault injection

Every HTTP mock can have its own `fault:` block — independently of protocol-level faults. This is useful for targeted latency tests or intermittent failures on one endpoint without affecting the rest of the protocol server.

```yaml
      - id: slow-search
        request:
          method: GET
          path: /search
        fault:
          delay: 2s       # add latency
          error_rate: 0.5 # only apply 50% of the time (0 = always)
        response:
          status: 200
          body: '[]'
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
      - proto: ./protos/payments.proto   # informational — no compilation needed
        mocks:
          - id: charge
            method: Charge
            response:
              success: true
              charge_id: ch_123
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
        match: "PING\r\n"
        response: "+PONG\r\n"
      - id: hex-response
        match: "re:^\\x02.*\\x03$"
        response: "hex:060000"
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

### SNMP

Full SNMP agent (powered by GoSNMPServer) that responds to GET, GETNEXT, GETBULK, and SET requests. Supports SNMPv1, v2c, and v3 (USM with MD5/SHA auth and DES/AES privacy). Can also send outbound TRAPs to any target host via the management API.

```yaml
protocols:
  snmp:
    enabled: true
    port: 1161            # default; 161 requires root / CAP_NET_BIND_SERVICE
    community: "public"   # v1/v2c community string
    v3_users:
      - username: mocklyuser
        auth_protocol: md5        # md5 | sha | sha224 | sha256 | sha384 | sha512
        auth_passphrase: mocklyauth
        priv_protocol: des        # des | aes | aes192 | aes256
        priv_passphrase: mocklypriv
    mocks:
      - id: sys-descr
        oid: 1.3.6.1.2.1.1.1.0
        type: string
        value: "Mockly Virtual Device"
      - id: sys-uptime
        oid: 1.3.6.1.2.1.1.3.0
        type: timeticks
        value: 987654
      - id: if-number
        oid: 1.3.6.1.2.1.2.1.0
        type: integer
        value: 4
    traps:
      - id: cold-start
        target: "127.0.0.1:1162"
        version: "2c"
        community: "public"
        oid: 1.3.6.1.6.3.1.1.5.1
        bindings:
          - oid: 1.3.6.1.2.1.1.1.0
            type: string
            value: "Device restarted"
```

**Supported OID types:**

| `type` value | SNMP ASN.1 type | Example `value` |
|---|---|---|
| `string` / `octetstring` | OctetString | `"Mockly Virtual Device"` |
| `integer` / `int` | Integer | `42` |
| `gauge32` | Gauge32 | `100` |
| `counter32` | Counter32 | `1048576` |
| `counter64` | Counter64 | `9000000000` |
| `timeticks` | TimeTicks | `987654` |
| `ipaddress` | IPAddress | `"192.168.1.1"` |
| `objectidentifier` / `oid` | ObjectIdentifier | `"1.3.6.1.2.1.1.2.0"` |

**TRAP sending** — POST to `/api/snmp/traps/{id}/send` to trigger any configured trap. The agent connects to the trap's `target` over UDP and sends the PDU.

### DNS

Full DNS mock server over UDP and TCP. Responds to `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `PTR`, `SRV`, and `NS` queries. `records` hold the raw answer values; for `MX` use `"<priority> <host>"`, and for `SRV` use `"<priority> <weight> <port> <target>"`.

```yaml
protocols:
  dns:
    enabled: true
    port: 5353
    mocks:
      - id: api-host
        name: "api.example.com"
        type: A
        records:
          - "127.0.0.1"
        ttl: 60
      - id: mail
        name: "example.com"
        type: MX
        records:
          - "10 mail.example.com"
```

### AMQP

AMQP 0.9.1 mock broker. Handles connection, channel, and basic frames, supports publish/consume flows, and captures published messages for inspection via the management API.

```yaml
protocols:
  amqp:
    enabled: true
    port: 5672
    mocks:
      - id: order-created
        exchange: orders
        routing_key: "order.created"
        response:
          body: '{"status":"accepted"}'
```

### Kafka

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

### LDAP

LDAP mock server handling Bind (success) and Search requests. Matches on base DN and filter, then returns configured attributes.

```yaml
protocols:
  ldap:
    enabled: true
    port: 3893
    mocks:
      - id: user-lookup
        base_dn: "dc=example,dc=com"
        filter: "*"
        attributes:
          cn:
            - "Alice Smith"
          mail:
            - "alice@example.com"
          uid:
            - "alice"
```

### IMAP

IMAP4rev1 mock server serving pre-configured mailboxes and messages. Supports LOGIN, SELECT, FETCH, SEARCH, and LOGOUT.

```yaml
protocols:
  imap:
    enabled: true
    port: 1143
    mailboxes:
      - id: inbox
        name: INBOX
        messages:
          - seq_num: 1
            from: "sender@example.com"
            to: "user@example.com"
            subject: "Test email"
            body: "Hello world"
```

### FTP

FTP mock server with PASV support plus LIST, RETR, STOR, and DELE. Files are pre-loaded from config.

```yaml
protocols:
  ftp:
    enabled: true
    port: 2121
    files:
      - id: daily-report
        path: /reports/daily.csv
        content: |
          date,revenue
          2024-01-01,1000
      - id: app-config
        path: /data/config.json
        content: '{"version":"1.0"}'
```

### Memcached

Memcached text-protocol mock handling `get`, `set`, `delete`, `flush_all`, `stats`, and `quit`. Keys support `*` wildcards and `re:` regex patterns.

```yaml
protocols:
  memcached:
    enabled: true
    port: 11211
    mocks:
      - id: session-cache
        command: get
        key: "session:*"
        response:
          value: '{"user_id":42,"role":"admin"}'
      - id: any-delete
        command: delete
        key: "*"
        response:
          status: DELETED
```

### STOMP

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

### CoAP

CoAP UDP mock server handling GET, POST, PUT, and DELETE requests. Matches on method + path with exact, wildcard, or regex patterns.

```yaml
protocols:
  coap:
    enabled: true
    port: 5683
    mocks:
      - id: temperature
        method: GET
        path: /sensors/temperature
        response:
          code: "2.05"
          payload: "23.5"
          content_format: 0   # text/plain
```

### SIP

SIP UDP mock server handling INVITE, REGISTER, OPTIONS, BYE, CANCEL, and ACK. Matches on method + URI using exact, wildcard, or regex patterns.

```yaml
protocols:
  sip:
    enabled: true
    port: 5060
    mocks:
      - id: invite-ok
        method: INVITE
        uri: "sip:*@example.com"
        response:
          status: 200
          reason: "OK"
      - id: register
        method: REGISTER
        uri: "*"
        response:
          status: 200
          reason: "OK"
```

---

## Component Testing

Mockly is designed for **component testing** — testing how your application behaves when a dependency returns errors, timeouts, unexpected data, or edge-case responses. The config file is owned by the dependency team; consuming teams just load it and toggle scenarios.

### Typical workflow

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

### Call verification

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

### Full component-test example

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

### Fault injection in scenarios

Scenarios can bundle protocol faults alongside mock patches — activating a scenario sets both atomically:

```yaml
scenarios:
  - id: backend-degraded
    name: Backend Degraded
    patches:
      - mock_id: get-user
        status: 503
    faults:
      redis:
        error: "LOADING Redis is loading the dataset in memory"
        error_rate: 1.0
      grpc:
        code: UNAVAILABLE
        delay: 500ms
        error_rate: 0.8

  - id: dns-failure
    name: DNS Resolution Failure
    faults:
      dns:
        rcode: NXDOMAIN
        error_rate: 0.5
```

When `backend-degraded` is activated: the `get-user` mock is patched and Redis/gRPC start returning faults. Deactivating the scenario restores normal behaviour.

---

## Fault Injection

Inject protocol-native faults to test your application's resilience without touching individual mocks. Each protocol has its own fault shape using native error codes.

### Via CLI

```sh
# DNS: 50% of queries return NXDOMAIN
mockly fault set --protocol dns --rcode NXDOMAIN --rate 0.5

# gRPC: always return UNAVAILABLE
mockly fault set --protocol grpc --code UNAVAILABLE

# Redis: always return LOADING error
mockly fault set --protocol redis --error "LOADING"

# Add 200ms latency to all Kafka requests
mockly fault set --protocol kafka --delay 200ms

# Clear a specific protocol's fault
mockly fault clear --protocol dns

# Clear all faults
mockly fault clear
```

### Via API

```sh
# Set DNS fault
curl -X POST http://localhost:9091/api/fault/dns \
  -H 'Content-Type: application/json' \
  -d '{"rcode":"NXDOMAIN","error_rate":0.5}'

# Set gRPC fault
curl -X POST http://localhost:9091/api/fault/grpc \
  -H 'Content-Type: application/json' \
  -d '{"code":"UNAVAILABLE","delay":"500ms","error_rate":1.0}'

# Get all active faults
curl http://localhost:9091/api/fault

# Clear DNS fault only
curl -X DELETE http://localhost:9091/api/fault/dns

# Clear all faults
curl -X DELETE http://localhost:9091/api/fault
```

### Fault fields per protocol

| Protocol | Fields | Values / notes |
|---|---|---|
| `http` / `graphql` | `status`, `body`, `delay`, `error_rate` | HTTP status code (default 503) |
| `websocket` | `close_code`, `message`, `delay`, `error_rate` | WS close code (default 1011) |
| `grpc` | `code`, `message`, `delay`, `error_rate` | `UNAVAILABLE` \| `NOT_FOUND` \| `DEADLINE_EXCEEDED` \| `PERMISSION_DENIED` \| `RESOURCE_EXHAUSTED` \| `INTERNAL` |
| `tcp` | `response`, `delay`, `error_rate` | Send `response` bytes then close (default: just close) |
| `redis` | `error`, `delay`, `error_rate` | Raw Redis error string e.g. `"LOADING"` (default `"ERR fault injected"`) |
| `dns` | `rcode`, `delay`, `error_rate` | `NXDOMAIN` \| `SERVFAIL` \| `REFUSED` \| `NOTIMP` \| `FORMERR` (default `SERVFAIL`) |
| `smtp` | `code`, `message`, `delay`, `error_rate` | SMTP code e.g. 421, 450, 550 (default 421) |
| `imap` | `response`, `message`, `delay`, `error_rate` | `NO` \| `BAD` \| `BYE` (default `NO`) |
| `ftp` | `code`, `message`, `delay`, `error_rate` | FTP code e.g. 421, 530, 550 (default 421) |
| `ldap` | `result_code`, `message`, `delay`, `error_rate` | LDAP result code: 32=NO_SUCH_OBJECT, 49=INVALID_CREDENTIALS, 50=INSUFFICIENT_ACCESS, 52=UNAVAILABLE (default 52) |
| `kafka` | `error_code`, `delay`, `error_rate` | Kafka error code: 3=UNKNOWN_TOPIC, 5=LEADER_NOT_AVAILABLE, 7=REQUEST_TIMED_OUT (default 5) |
| `memcached` | `error_type`, `message`, `delay`, `error_rate` | `SERVER_ERROR` \| `CLIENT_ERROR` (default `SERVER_ERROR`) |
| `stomp` | `message`, `delay`, `error_rate` | Sends STOMP ERROR frame |
| `amqp` | `delay`, `error_rate` | Silently drops message delivery |
| `mqtt` | `delay`, `error_rate` | Silently drops response publish |
| `coap` | `code`, `delay`, `error_rate` | CoAP code: `4.01`, `4.03`, `4.04`, `5.00`, `5.03` (default `5.00`) |
| `sip` | `status`, `reason`, `delay`, `error_rate` | SIP status: 404, 408, 486, 503 (default 503) |
| `snmp` | `message`, `delay`, `error_rate` | Returns error from OID callback |

`error_rate`: probability 0.0–1.0 that the fault fires; 0 = always (default).

### Via scenarios (recommended for reproducible tests)

See [Fault injection in scenarios](#fault-injection-in-scenarios).

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
mockly fault       set --protocol <proto> [--delay <d>] [--rate <f>] [protocol-specific flags] | clear [--protocol <proto>] | show
```

---

## Management API Reference

Base URL: `http://localhost:9091`

> **API documentation files** — two ready-to-use references ship in `docs/`:
>
> | File | Format | How to use |
> |---|---|---|
> | [`docs/openapi.yaml`](docs/openapi.yaml) | OpenAPI 3.1 | Open in [Swagger UI](https://editor.swagger.io/), Redoc, Stoplight, or any OpenAPI-compatible tool |
> | [`docs/mockly.postly.json`](docs/mockly.postly.json) | [Postly](https://github.com/dever-labs/postly) collection | Import into Postly and set the `baseUrl` environment variable to your Mockly instance (e.g. `http://localhost:9091`) |

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

Similarly for WebSocket (`/api/mocks/websocket`), gRPC (`/api/mocks/grpc`), GraphQL (`/api/mocks/graphql`), TCP (`/api/mocks/tcp`), Redis (`/api/mocks/redis`), SMTP (`/api/mocks/smtp`), MQTT (`/api/mocks/mqtt`), SNMP (`/api/mocks/snmp`), DNS (`/api/mocks/dns`), AMQP (`/api/mocks/amqp`), Kafka (`/api/mocks/kafka`), LDAP (`/api/mocks/ldap`), IMAP (`/api/mocks/imap`), FTP (`/api/mocks/ftp`), Memcached (`/api/mocks/memcached`), STOMP (`/api/mocks/stomp`), CoAP (`/api/mocks/coap`), and SIP (`/api/mocks/sip`).

### Call Verification (HTTP)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/calls/http/{mockId}` | Get call count + log entries for a mock |
| `POST` | `/api/calls/http/{mockId}/wait` | Block until mock has been called N times (body: `{"count":N,"timeout":"5s"}`) |
| `DELETE` | `/api/calls/http/{mockId}` | Clear log entries for mock + reset all HTTP call counts |
| `DELETE` | `/api/calls/http` | Clear all HTTP log entries and reset all call counts |

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

### Message stores (AMQP, Kafka, STOMP)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/amqp/messages` | List captured AMQP messages |
| `DELETE` | `/api/amqp/messages` | Clear AMQP message store |
| `GET` | `/api/kafka/messages` | List captured Kafka messages |
| `DELETE` | `/api/kafka/messages` | Clear Kafka message store |
| `GET` | `/api/stomp/messages` | List captured STOMP messages |
| `DELETE` | `/api/stomp/messages` | Clear STOMP message store |

### SNMP Mocks & Traps

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/mocks/snmp` | List configured OID mocks |
| `POST` | `/api/mocks/snmp` | Add an OID mock |
| `PUT` | `/api/mocks/snmp/{id}` | Replace an OID mock |
| `DELETE` | `/api/mocks/snmp/{id}` | Remove an OID mock |
| `GET` | `/api/snmp/traps` | List configured traps |
| `POST` | `/api/snmp/traps` | Add a trap config |
| `POST` | `/api/snmp/traps/{id}/send` | Send a configured trap immediately |

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
| `GET` | `/api/fault` | Get all active direct faults |
| `DELETE` | `/api/fault` | Clear all direct faults |
| `GET` | `/api/fault/{protocol}` | Get fault config for a protocol |
| `POST` | `/api/fault/{protocol}` | Set fault config for a protocol |
| `DELETE` | `/api/fault/{protocol}` | Clear fault for a protocol |

`{protocol}` is one of: `http`, `graphql`, `websocket`, `grpc`, `tcp`, `redis`, `mqtt`, `smtp`, `snmp`, `dns`, `amqp`, `kafka`, `ldap`, `imap`, `ftp`, `memcached`, `stomp`, `coap`, `sip`.

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

## Client Libraries

Mockly ships official clients that manage the process lifecycle, port allocation, and the management API for you — so tests stay clean and portable.

| Language | Package | Install |
|---|---|---|
| **Go** | `github.com/dever-labs/mockly/clients/go` | `go get github.com/dever-labs/mockly/clients/go` |
| **Node.js / TypeScript** | `@dever-labs/mockly-driver` | `npm i -D @dever-labs/mockly-driver` |
| **Java** | `io.github.dever-labs:mockly-driver` | See Maven/Gradle below |
| **.NET / C#** | `Mockly.Driver` | `dotnet add package Mockly.Driver` |
| **Python** | `mockly-driver` | `pip install mockly-driver` |
| **Rust** | `mockly-driver` | `mockly-driver = "0.4"` in `[dev-dependencies]` |

All clients:
- Automatically find or install the Mockly binary for the current platform
- Allocate two free ports atomically (no TOCTOU races)
- Retry startup up to 3 times on port conflicts
- Expose the same concepts: `addMock`, `activateScenario`, `setFault`, `reset`, `stop`

### Go

```go
import mocklydriver "github.com/dever-labs/mockly/clients/go"

server, err := mocklydriver.Ensure(mocklydriver.Options{}, mocklydriver.InstallOptions{})
defer server.Stop()

server.AddMock(mocklydriver.Mock{
    ID:       "get-user",
    Request:  mocklydriver.Request{Method: "GET", Path: "/users/1"},
    Response: mocklydriver.Response{Status: 200, Body: `{"id":1}`},
})
// server.HTTPBase = "http://127.0.0.1:<port>"
```

[→ Full Go docs](docs/clients/go.md)

### Node.js / TypeScript

```ts
import { MocklyServer } from '@dever-labs/mockly-driver'

const server = await MocklyServer.ensure()
await server.addMock({
    id: 'get-user',
    request: { method: 'GET', path: '/users/1' },
    response: { status: 200, body: '{"id":1}' },
})
// server.httpBase = "http://127.0.0.1:<port>"
await server.stop()
```

[→ Full Node.js docs](docs/clients/node.md)

### Java

```xml
<dependency>
  <groupId>io.github.dever-labs</groupId>
  <artifactId>mockly-driver</artifactId>
  <version>0.4.7</version>
  <scope>test</scope>
</dependency>
```

```java
try (MocklyServer server = MocklyServer.ensure(MocklyConfig.builder().build())) {
    server.addMock(Mock.builder("get-user",
        MockRequest.builder("GET", "/users/1").build(),
        MockResponse.builder(200).body("{\"id\":1}").build()
    ).build());
    // server.httpBase = "http://127.0.0.1:<port>"
}
```

[→ Full Java docs](docs/clients/java.md)

### .NET / C#

```sh
dotnet add package Mockly.Driver
```

```csharp
await using var server = await MocklyServer.CreateAsync();
await server.AddMockAsync(new Mock {
    Id = "get-user",
    Request  = new MockRequest { Method = "GET", Path = "/users/1" },
    Response = new MockResponse { Status = 200, Body = """{"id":1}""" },
});
// server.HttpBase = "http://127.0.0.1:<port>"
```

[→ Full .NET docs](docs/clients/dotnet.md)

### Python

```sh
pip install mockly-driver
```

```python
from mockly_driver import MocklyServer, Mock, MockRequest, MockResponse

server = MocklyServer.ensure()
server.add_mock(Mock(
    id="get-user",
    request=MockRequest(method="GET", path="/users/1"),
    response=MockResponse(status=200, body='{"id":1}'),
))
# server.http_base = "http://127.0.0.1:<port>"
server.stop()
```

[→ Full Python docs](docs/clients/python.md)

### Rust

```toml
[dev-dependencies]
mockly-driver = "0.4"
```

```rust
let mut server = MocklyServer::ensure(ServerOptions::default(), Default::default()).unwrap();
server.add_mock(&Mock {
    id: "get-user".into(),
    request: Request { method: "GET".into(), path: "/users/1".into(), ..Default::default() },
    response: Response { status: 200, body: Some(r#"{"id":1}"#.into()), ..Default::default() },
}).unwrap();
// server.http_base = "http://127.0.0.1:<port>"
```

[→ Full Rust docs](docs/clients/rust.md)

---

## CI Integration

Mockly is a single static binary with no runtime dependencies — ideal for CI.

### GitHub Actions (composite action)

```yaml
steps:
  - uses: actions/checkout@v5

  - name: Start Mockly
    uses: dever-labs/mockly/.github/actions/setup-mockly@v0.4.7
    with:
      version: v0.4.7          # pin to a specific version
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
    MOCKLY_VERSION: "v0.4.7"
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
MOCKLY_VERSION=v0.2.0 \
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
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                                Single Binary                                             │
│                                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐                 │
│  │                     Management API + Web UI  :9091                  │                 │
│  │  CRUD mocks/rules  ·  scenarios  ·  fault  ·  state  ·  logs/SSE    │                 │
│  └─────────────────────────────────────────────────────────────────────┘                 │
│                                                                                          │
│  ┌──────┐ ┌─────────┐ ┌──────┐ ┌─────────┐ ┌─────┐ ┌───────┐ ┌──────┐ ┌──────┐ ┌──────┐  │
│  │ HTTP │ │WebSocket│ │ gRPC │ │GraphQL  │ │ TCP │ │ Redis │ │ SMTP │ │ MQTT │ │ SNMP │  │
│  │:8080 │ │ :8081   │ │:50051│ │ :8082   │ │:8083│ │ :6379 │ │:2525 │ │:1883 │ │:1161 │  │
│  └──────┘ └─────────┘ └──────┘ └─────────┘ └─────┘ └───────┘ └──────┘ └──────┘ └──────┘  │
│                                                                                          │
│  Shared:  State Store  ·  Request Logger  ·  Scenario Store                              │
└──────────────────────────────────────────────────────────────────────────────────────────┘
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

See [CONTRIBUTING.md](CONTRIBUTING.md) for full setup instructions, commit conventions, and the PR process.

---

## Contributing

Contributions are welcome — bug reports, feature requests, preset configs, and code.

Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.
By participating you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

For security issues, follow the process in [SECURITY.md](SECURITY.md) — **do not open a public issue**.

---

## License

Copyright © 2026 dever-labs. Released under the [MIT License](LICENSE).

