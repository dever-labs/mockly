# Configuration

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
      - mocks:
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

## Path matching

| Pattern | Matches |
|---|---|
| `/api/users` | Exact match |
| `/api/*` | Any path starting with `/api/` (trailing wildcard) |
| `/regions/*/emails` | Any single segment in the middle — e.g. `/regions/fr-par/emails` |
| `/users/{id}` | Named segment — value captured as `{id}` in templates and logs |
| `/orders/{id}/items/{item}` | Multiple named segments |
| `re:^/users/\d+$` | Inline regex with `re:` prefix |

Use `path_regex` when you prefer a dedicated regex field instead of the inline `re:` prefix on `path`:

```yaml
request:
  method: GET
  path_regex: "^/users/\\d+$"   # alternative to re: prefix
```

## Template responses

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

### Request context

In addition to the legacy shorthand aliases above, templates expose a `request.*` namespace:

| Variable | Example | Description |
|---|---|---|
| `{{.request.method}}` | `POST` | HTTP method of the incoming request |
| `{{.request.path}}` | `/users/42` | Request path |
| `{{.request.params.id}}` | `42` | Named path parameter captured by `{id}` |
| `{{.request.query.foo}}` | `bar` | Query parameter (alias: `{{.query.foo}}` still works) |
| `{{.request.headers.X-Foo}}` | `…` | Request header (alias: `{{.headers.X-Foo}}` still works) |
| `{{.request.body.field}}` | `…` | JSON field from request body (alias: `{{.body}}` for raw body still works) |

**Sequence counters** (`{{seq "name"}}`) are reset to zero by `POST /api/reset` or `mockly reset`.

Example — echo request fields in a create-style API:

```yaml
- id: create-email
  request:
    method: POST
    path: /transactional-email/v1alpha1/regions/{region}/emails
  response:
    status: 200
    body: |
      {
        "id": "{{uuid}}",
        "region": "{{.request.params.region}}",
        "project_id": "{{.request.body.project_id}}",
        "created_at": "{{now}}"
      }
```

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

## Variables and environment substitution

Config files support `${NAME}` and `${NAME:-default}` references, expanded
before the file is parsed as YAML. A reference is resolved in this order:

1. An OS environment variable named `NAME`, if set.
2. An entry named `NAME` in the config's own top-level `vars:` map.
3. The inline `:-default` fallback, if present.
4. Otherwise, config loading fails with an error listing every missing name.

Environment variables keep secrets and per-environment values (signing
keys, ports, hostnames) out of a config file that might be committed to
source control. The `vars:` map lets a config (or a preset you're sharing)
define its own committable defaults in one place instead of repeating the
same literal value throughout the file — and those defaults can still be
overridden at runtime via an OS environment variable of the same name,
without editing the file:

```yaml
vars:
  realm: myrealm
  issuer: http://localhost:8080

protocols:
  http:
    enabled: true
    port: ${MOCKLY_HTTP_PORT:-8080}
    mocks:
      - id: webhook
        request: { method: POST, path: /webhook }
        response:
          status: 200
          headers:
            X-Signature: "${WEBHOOK_SIGNING_KEY}"
      - id: realm-info
        request: { method: GET, path: "/realms/${realm}" }
        response: { status: 200, body: "{\"issuer\": \"${issuer}/realms/${realm}\"}" }
```

- `${NAME}` — replaced with the env var or `vars:` entry. If neither
  exists, config loading fails with an error listing every missing name.
- `${NAME:-default}` — replaced with the env var or `vars:` entry if either
  exists, otherwise the literal `default` (which may be empty: `${NAME:-}`).
- `vars:` entries are plain string values — they are not themselves
  further expanded (no recursive `${...}` resolution inside a `vars:`
  entry's value).

## Splitting mocks across multiple files

A large mock set doesn't have to live in one monolithic file. Two
equivalent ways to split it up:

**`include:`** — list additional YAML files (glob patterns, resolved
relative to the file that declares them) in the main config:

```yaml
mockly:
  api:
    port: 9091

protocols:
  http:
    enabled: true
    port: 8080

include:
  - orders.yaml
  - users.yaml
  - services/*.yaml
```

**Directory mode** — point `--config`/`-c` at a directory instead of a
file. It must contain a `mockly.yaml` (the main file, same as above);
every other `*.yaml`/`*.yml` file directly inside that same directory is
automatically included, in filename order — no explicit `include:` list
needed:

```
mocks/
├── mockly.yaml   # mockly:/protocols: top-level settings, ports, enabled flags
├── orders.yaml   # protocols.http.mocks only
└── users.yaml    # protocols.ldap: a protocol mockly.yaml never mentions
```

```bash
mockly start -c ./mocks/
```

Each included file is parsed with the exact same schema as a normal
config, but only two things from it are actually used:

- **A protocol section not yet configured anywhere else** (port, enabled,
  users, everything) is adopted as-is from the first file that defines it.
- **Mocks for a protocol that's already configured** (in the main file or
  an earlier include) are merged in: an included mock whose `id` matches
  an existing one overrides it in place, anything else is appended —
  the same override-by-`id` rule used for OpenAPI/AsyncAPI/`.proto`
  generated mocks layered under hand-written ones.

Everything else in an included file (a different `mockly:` section, a
different port/enabled for a protocol the main file already configures)
is ignored, so there's always exactly one place that's authoritative for
where a protocol listens. An included file's own `include:` field is
ignored too — include chains can't form.

## Hot-reload with `--watch`

Pass `--watch` to `mockly start` to reload mocks whenever the config file
(or, with `include:`/directory mode, any file that contributed to it)
changes on disk, without restarting the server or losing in-progress
connections:

```bash
mockly start -c ./mocks/ --watch
```

On each change, the config is re-read, inline schema refs
(`openapi`/`asyncapi`/`proto`) are re-resolved, and the result is
validated exactly like at startup:

- If it's **valid**, every already-running protocol server has its mocks
  replaced in place via the same `SetMocks`/`SetRules`/`SetMailboxes`/
  `SetFiles` code path the management API's bulk-set endpoints use.
- If it's **invalid** (parse error or failed validation), the reload is
  rejected, an error is logged to stderr, and the server keeps serving
  the last-known-good config — an editing mistake never crashes a running
  mock server.

`--watch` only updates mocks on protocols that were already enabled at
startup. Enabling a previously-disabled protocol, or changing a port,
requires a restart.

## TLS

Several protocols (`http`, `websocket`, `graphql`, `tcp`, and the
management API) accept a `tls:` block to serve over TLS:

```yaml
protocols:
  http:
    enabled: true
    port: 8443
    tls:
      enabled: true
      cert_file: ./certs/server.crt
      key_file: ./certs/server.key
```

`cert_file`/`key_file` must either both be set or both be omitted —
setting only one is rejected by `mockly config validate`.

**Automatic self-signed certificates.** If `cert_file`/`key_file` are
omitted (or point to files that don't exist yet), a self-signed
certificate is generated automatically — no extra flag needed:

```yaml
protocols:
  http:
    enabled: true
    port: 8443
    tls:
      enabled: true # no cert_file/key_file: generates one on the fly
```

- If `cert_file`/`key_file` are **omitted entirely**, the certificate
  stays in-memory only and a fresh one is generated on every start.
- If `cert_file`/`key_file` are **set but the files don't exist yet**,
  the generated cert/key are written there, so subsequent starts reuse
  the same certificate instead of generating a new one each time.

The certificate's content can be customized via `tls.generate`:

```yaml
tls:
  enabled: true
  cert_file: ./certs/server.crt
  key_file: ./certs/server.key
  generate:
    common_name: mocks.example.com
    organization: Example Corp
    dns_names: [mocks.example.com, api.example.local]
    ip_addresses: [127.0.0.1, "::1"]
    valid_for: 720h # default: 8760h (1 year)
    # For mutual-TLS/authorization test scenarios where the same cert is
    # also presented as a client certificate:
    client_auth: true
    uris: [spiffe://example.org/ns/default/sa/mockly]
```

All `generate` fields are optional; omitted ones fall back to
`common_name: localhost`, `dns_names: [localhost]`,
`ip_addresses: [127.0.0.1, "::1"]`, and `valid_for: 8760h`.

**Per-listener TLS (HTTP only).** `protocols.http.listeners` adds extra
named listeners, each on its own port with its own optional `tls:` config,
alongside the protocol's default listener (`protocols.http.port`/`tls`).
A mock opts into a named listener via `listener: <name>`; mocks that
leave `listener` unset stay on the default listener exactly as before —
configs that don't use `listeners` are unaffected.

```yaml
protocols:
  http:
    enabled: true
    port: 8080 # default listener: plain HTTP
    listeners:
      - name: secure
        port: 8443
        tls:
          enabled: true # auto-generates a cert if none is configured
    mocks:
      - id: public
        request: { method: GET, path: /public }
        response: { status: 200, body: "ok" }
      - id: admin
        listener: secure # only reachable on :8443, over TLS
        request: { method: GET, path: /admin }
        response: { status: 200, body: "ok" }
```

This lets a subset of mocks be served only over TLS (or a different TLS
cert/listener) while the rest stay on the plain default listener, without
a whole separate `protocols.http` block. `mockly config validate` rejects
listener names that are empty or duplicated, ports that collide with
another listener or the default port, and a `listener:` on a mock that
doesn't match a declared listener.

---
