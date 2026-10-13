# HTTP

Full HTTP mock server. Matching on method + path (exact/wildcard/named params/regex), optional query params, header match (with `re:` / `*` pattern support), JSON/multipart/XML body field match, authentication, and state condition.

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

## Query parameter matching

```yaml
      - id: admin-users
        request:
          method: GET
          path: /users
          query:
            role: admin              # exact match
            page: "*"                # any value (wildcard)
            order_id: 're:^ORD-\d+$' # regex match ("re:" prefix)
            debug: "!present"        # asserts the param must be ABSENT
        response:
          status: 200
          body: '[{"id":1,"role":"admin"}]'
```

When a query param is repeated (e.g. `?tag=a&tag=b`), a match succeeds if
**any** occurrence satisfies the configured value/pattern.

## JSON body field matching

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

## Form body field matching

For classic HTML `<form method="post">` submissions (e.g. a hosted
payment/checkout page posting back card details), `body_form` matches
fields in an `application/x-www-form-urlencoded` body. Unlike
`body_json`/`body_multipart`/`body_xml`, values also support the `"re:"`
regex prefix — useful for matching a card number pattern rather than an
exact value:

```yaml
      - id: checkout-declined
        request:
          method: POST
          path: /checkout/{id}
          body_form:
            card_number: "re:.*0000$" # any card number ending in 0000
        response:
          status: 200
          body: {file: ./mocks/declined.html}

      - id: checkout-approved
        request:
          method: POST
          path: /checkout/{id}
        response:
          status: 200
          body: '{{.request.body.card_number}}' # last-4 echoed back via templating, no extra config needed
```

A repeated field name (e.g. multiple same-named checkboxes) matches if
**any** submitted value satisfies the configured value/pattern, mirroring
query-param matching. The request Content-Type must declare
`application/x-www-form-urlencoded` (a charset parameter is fine) for
`body_form` to apply at all — this avoids spurious matches on non-form
bodies.

See the [Nets/Nexi preset](https://github.com/dever-labs/mockly/blob/main/configs/presets/nets.yaml) for a complete,
runnable worked example: a hosted checkout page (`GET /checkout/{id}`)
with a real HTML form, posted back to `body_form`-matched
approved/declined responses — no HTML authoring required, just
`mockly preset use nets`.

## Multipart and XML body field matching

For `multipart/form-data` requests (e.g. file uploads), `body_multipart`
matches individual form fields by name. Text fields are matched by value;
file parts are matched via `<field>.filename`/`<field>.content_type`:

```yaml
      - id: avatar-upload
        request:
          method: POST
          path: /profile/avatar
          body_multipart:
            "name": "Alice"              # text field, exact match
            "avatar.filename": "*"       # asserts a file was uploaded
            "avatar.content_type": "image/png"
        response:
          status: 200
          body: '{"ok":true}'
```

For XML bodies (SOAP, legacy enterprise APIs), `body_xml` matches
element text and attributes using the same dot-notation style as
`body_json`; a leading `@` segment matches an attribute instead of
descending into a child element:

```yaml
      - id: soap-login
        request:
          method: POST
          path: /soap
          body_xml:
            "user.role": admin     # <user><role>admin</role></user>
            "user.@id": "*"        # <user id="...">
        response:
          status: 200
          body: '<ok/>'
```

## Response sequences

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

## Streaming responses (SSE / chunked)

Set `response.stream.events` to turn a mock into a sequence of flushed
events instead of one static body — for Server-Sent Events
(`text/event-stream`) or plain chunked/flushed streaming. Each event's
`data` is rendered as a template (same `{{ }}` syntax as `body`), and its
optional `delay` is slept before that event is written and flushed.

```yaml
      - id: chat-stream
        request:
          method: POST
          path: /v1/chat/completions
        response:
          status: 200
          headers:
            Content-Type: text/event-stream
          stream:
            events:
              - data: '{"chunk":"Hello"}'
                delay: 100ms
              - data: '{"chunk":" world"}'
                delay: 100ms
              - data: "[DONE]"
```

When `Content-Type` contains `text/event-stream`, each event is framed as
an SSE message: optional `event:`/`id:` lines, one `data:` line per line
of `data` (multi-line payloads are supported per the SSE spec), then a
blank line. Without that content type, `data` is written and flushed
as-is — no SSE framing — for plain chunked streaming.

`stream` is not combined with `sequence`, scenario patches, or fault
injection: if any of those fire for a given call, that call falls back to
a normal static response instead of streaming.

By default the response ends once all configured `events` have been sent.
Set `stream.keep_open: true` to leave the connection open indefinitely
afterwards instead, so it can receive further ad-hoc events pushed through
the [stream management API](graphql.md#stream-management-api-push-close) — see that
section for examples and the cross-protocol behavior this also applies to
for gRPC streaming and GraphQL subscriptions.

```yaml
      - id: live-ticker
        request:
          method: GET
          path: /v1/ticker
        response:
          status: 200
          headers:
            Content-Type: text/event-stream
          stream:
            keep_open: true
            events:
              - data: '{"price":100}'
```

## Per-mock fault injection

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

Instead of (or in addition to) a fixed `delay`, use `delay_range` for a
uniform-random jittered delay per request — more realistic than one constant
value for testing timeout/retry/loading-state handling:

```yaml
        fault:
          delay_range:
            min: 150ms
            max: 400ms
```

Use `rate_limit` to simulate a throttled API: once more than
`requests_per_second` requests are observed in the trailing one-second
window, `over_limit_status` (default 429) is returned instead of the normal
response. Each mock with a `rate_limit` fault tracks its own independent
window.

```yaml
        fault:
          rate_limit:
            requests_per_second: 5
            over_limit_status: 429     # default 429
            body: '{"error":"rate limit exceeded"}'
```

## Near-miss diagnostics for unmatched requests

By default, a request that matches no mock returns a plain
`{"error":"no mock matched"}` (404) — this never changes for normal
client traffic. To debug *why* a request didn't hit the mock you
expected, opt in with `?debug=true` or an `X-Mockly-Debug: true` header:

```bash
curl "http://localhost:8080/orders?order_id=bad-id&debug=true"
```

```json
{
  "error": "no mock matched",
  "near_misses": [
    { "mock_id": "get-order", "reason": "query \"order_id\" value(s) [bad-id] did not match \"re:^ORD-\\d+$\"" }
  ]
}
```

Up to 3 of the closest candidates are reported (mocks that passed the
most checks — method, path, headers, query, body, `body_json`,
`body_form`, `body_multipart`, `body_xml`, state, auth — before failing), ranked
closest-first. Near-miss info is also
included in the mock's request log entry whenever diagnostics were
requested, so it shows up in the management UI's live log stream too.

## Authentication matching

Use the `auth:` block to require valid credentials before a mock is considered a match. When auth fails the mock is skipped — add a fallback mock (without `auth`) to return your preferred 401 response.

Header values everywhere also support `re:` regex patterns and `*` wildcard, making it easy to match any bearer token without an `auth:` block if you prefer.

**Bearer token**

```yaml
      - id: secure-endpoint
        request:
          method: GET
          path: /api/data
          auth:
            type: bearer
            token: "my-secret-token"  # exact | "re:^ey[A-Za-z0-9]" regex | "*" (any token)
        response:
          status: 200
          body: '{"data":"sensitive"}'

      - id: secure-endpoint-unauth        # fallback for missing/wrong token
        request:
          method: GET
          path: /api/data
        response:
          status: 401
          headers:
            WWW-Authenticate: 'Bearer realm="api"'
          body: '{"error":"unauthorized"}'
```

**Basic auth**

```yaml
      - id: admin-page
        request:
          method: GET
          path: /admin
          auth:
            type: basic
            username: admin
            password: s3cret     # Mockly decodes the Base64 header internally
        response:
          status: 200
```

**API key** (header or query parameter)

```yaml
      - id: weather
        request:
          method: GET
          path: /weather
          auth:
            type: api_key
            header: X-API-Key    # or: query: api_key
            value: "key-abc123"  # exact | "re:…" | "*"
        response:
          status: 200
```

**NTLM** (Windows-integrated auth)

Mockly drives the full 3-step NTLM challenge/response handshake automatically. Any well-formed token sequence is accepted — no credential validation.

```yaml
      - id: windows-endpoint
        request:
          method: GET
          path: /api/windows
          auth:
            type: ntlm
        response:
          status: 200
          body: '{"authenticated":true}'
```

The handshake sequence:
1. No `Authorization` header → `401 Unauthorized` + `WWW-Authenticate: NTLM`
2. `Authorization: NTLM <type-1 Negotiate>` → `401 Unauthorized` + `WWW-Authenticate: NTLM <type-2 Challenge>`
3. `Authorization: NTLM <type-3 Authenticate>` → mock's configured response

See [`configs/presets/ntlm.yaml`](https://github.com/dever-labs/mockly/blob/main/configs/presets/ntlm.yaml) for a complete working example.

**Digest**

Requires an `Authorization: Digest …` header to be present (no nonce validation):

```yaml
          auth:
            type: digest
```

## HTML page mocking (`body: {file: ...}` / `{dir: ...}`)

A response `body` can be a plain string (as above), or a structured
reference to a file or a whole folder on disk — handy for serving real
HTML/CSS/JS/images without hand-escaping markup into a YAML string.
File content is read fresh on every request (no caching), so editing the
file on disk takes effect immediately, no restart required. Templates
(`{{ }}`) still render normally inside a file's content, except for
binary content-types (images, fonts, pdf, wasm, audio/video), which are
served as-is.

```yaml
      - id: checkout-page
        request:
          method: GET
          path: /checkout
        response:
          status: 200
          body:
            file: ./pages/checkout.html   # resolved relative to this config file's directory
```

The `Content-Type` header is inferred from the file's extension unless
you set an explicit `headers: {Content-Type: ...}` (which always wins).

**Serving a whole folder (`dir`)**

`body: {dir: ...}` expands, at config-load time, into one generated mock
per file found recursively underneath that folder (dotfiles/dot-folders
are skipped). Each file's path relative to the folder is joined with the
parent mock's own `request.path` using URL (not OS) path semantics:

```yaml
      - id: docs-site
        request:
          method: GET
          path: /docs
        response:
          status: 200
          body:
            dir: ./site/docs
```

Given `./site/docs/index.html` and `./site/docs/guide/intro.html`, this
registers `GET /docs/index.html`, `GET /docs/guide/intro.html`, **and**
(the standard "directory index" convention) `GET /docs` itself — any
`index.html` found at any depth is additionally registered at its parent
directory's own path. A `body.dir` mock must have a static, literal
`request.path` (no `path_regex`, wildcards, `{name}` captures, or `re:`
prefixes) and can't be combined with `sequence`. Loading fails fast if
two mocks (hand-written or generated) end up with the same method+path.

**Scaffolding a page without editing YAML (`mockly add page`)**

```bash
# Serve a single HTML file:
mockly add page --path /checkout --file ./pages/checkout.html

# Expand a whole folder of static assets into one mock per file:
mockly add page --path /docs --dir ./site/docs
```

Both forms resolve paths relative to the CLI's own working directory, then
read each file's content immediately and send it as a literal response
body — the management API never resolves a file/folder path supplied by a
caller, by design (it's a network-reachable control plane, so letting it
touch the server's filesystem on request would be a path-traversal risk).
Binary assets (images, fonts, PDFs, …) transfer byte-for-byte intact, not
just text. This means mocks added via `mockly add page` are **point-in-time
snapshots**: unlike `body.file`/`body.dir` configured directly in
`mockly.yaml` (which always re-read the file fresh from disk on every
request), editing the source file after running `add page` has no effect
until you re-run the same command. Re-running it (e.g. after editing the
folder's contents) replaces the previously-added mock(s) instead of
duplicating them, so refreshing is just re-running the same command.
