# mockly-driver

Node.js client for [Mockly](https://github.com/dever-labs/mockly) — start, stop, and control Mockly HTTP mock servers from Node.js test suites.

```ts
import { MocklyServer } from 'mockly-driver'

const server = await MocklyServer.ensure() // install binary if needed, then start

await server.addMock({
  id: 'get-users',
  request: { method: 'GET', path: '/users' },
  response: { status: 200, body: '[{"id":1}]', headers: { 'Content-Type': 'application/json' } },
})

const res = await fetch(`${server.httpBase}/users`)
// → 200 [{"id":1}]

await server.stop()
```

## Installation

```sh
npm install --save-dev mockly-driver
```

The Mockly binary is **not** bundled in the npm package. Download it once before running tests:

```sh
npx mockly-driver-install
```

Or let `MocklyServer.ensure()` handle it automatically on first run.

---

## Usage

### `MocklyServer.ensure(opts?)` _(recommended)_

Downloads the binary if missing, then starts the server.

```ts
const server = await MocklyServer.ensure()
```

### `MocklyServer.create(opts?)`

Starts the server using an already-installed binary. Throws if the binary is not found.

```ts
const server = await MocklyServer.create()
```

### Test framework integration

```ts
// vitest / jest
import { MocklyServer } from 'mockly-driver'

let server: MocklyServer

beforeAll(async () => {
  server = await MocklyServer.ensure()
}, 30_000)

afterAll(() => server?.stop())

beforeEach(() => server.reset()) // isolate each test

it('returns 200', async () => {
  await server.addMock({
    id: 'ping',
    request: { method: 'GET', path: '/ping' },
    response: { status: 200, body: 'pong' },
  })
  const res = await fetch(`${server.httpBase}/ping`)
  expect(res.status).toBe(200)
})
```

---

## Management API

### `server.addMock(mock)`

Adds an HTTP mock. Mocks are matched in insertion order — first match wins.

```ts
await server.addMock({
  id: 'create-user',
  request: {
    method: 'POST',
    path: '/users',
    headers: { Authorization: 'Bearer mytoken' }, // exact match
  },
  response: {
    status: 201,
    body: '{"id":42}',
    headers: { 'Content-Type': 'application/json' },
    delay: '50ms',
  },
})
```

> **Note:** Header matching is exact. Place mocks with header constraints _before_ less-specific fallbacks.

### `server.deleteMock(id)`

Removes a mock by id.

### `server.reset()`

Removes all dynamically added mocks, deactivates scenarios, and clears fault injection. Mocks defined in startup config are preserved. Call in `beforeEach` to keep tests isolated.

### Scenarios

Scenarios override mock responses when activated — useful for simulating outage modes.

```ts
const server = await MocklyServer.ensure({
  scenarios: [
    {
      id: 'auth-down',
      name: 'Auth service unavailable',
      patches: [{ mock_id: 'token-endpoint', status: 503, body: '{"error":"down"}' }],
    },
  ],
})

await server.addMock({
  id: 'token-endpoint',
  request: { method: 'POST', path: '/token' },
  response: { status: 200, body: '{"access_token":"abc"}' },
})

// Normal operation
const r1 = await fetch(`${server.httpBase}/token`, { method: 'POST' })
// → 200

await server.activateScenario('auth-down')

const r2 = await fetch(`${server.httpBase}/token`, { method: 'POST' })
// → 503

await server.deactivateScenario('auth-down')
```

### Fault injection

```ts
// Delay every response by 200 ms
await server.setFault({ enabled: true, delay: '200ms' })

// Override all responses with 503
await server.setFault({ enabled: true, status_override: 503 })

// Randomly fail 30% of requests
await server.setFault({ enabled: true, status_override: 503, error_rate: 0.3 })

await server.clearFault()
```

---

## Binary installation

### Default (download from GitHub)

```sh
npx mockly-driver-install
```

Or programmatically:

```ts
import { install } from 'mockly-driver'
await install()
```

### Artifactory / internal mirror

Set `MOCKLY_DOWNLOAD_BASE_URL` to your mirror's base URL (the path up to and including the version segment prefix):

```sh
MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.company.com/artifactory/github-releases/dever-labs/mockly/releases/download \
  npx mockly-driver-install
```

Artifactory setup: create a **Generic Remote Repository** pointing to `https://github.com` and enable "Store Artifacts Locally". The download URL then becomes `https://<artifactory>/artifactory/<repo>/dever-labs/mockly/releases/download`.

### HTTP / HTTPS proxy

Set `HTTPS_PROXY` or `HTTP_PROXY` before running the install:

```sh
HTTPS_PROXY=https://proxy.company.com:3128 npx mockly-driver-install
```

Proxy authentication is supported via the proxy URL: `https://user:pass@proxy:3128`.

> **Note:** If your proxy username or password contains special characters (e.g. `@`, `:`, `/`), URL-encode them first — e.g. `p@ss` → `p%40ss`. Use `encodeURIComponent()` in Node.js or an online encoder.

> **Tip:** For Artifactory, `MOCKLY_DOWNLOAD_BASE_URL` is simpler and more reliable than `HTTPS_PROXY`.

### Air-gapped environments

Pre-stage the binary and point to it:

```sh
# On a machine with internet access:
npx mockly-driver-install

# Copy bin/mockly[.exe] to the air-gapped machine, then:
MOCKLY_BINARY_PATH=/opt/tools/mockly npx vitest run
```

Or set `MOCKLY_NO_INSTALL=true` to make the binary absence a hard error with actionable instructions:

```sh
MOCKLY_NO_INSTALL=true npx vitest run  # fails fast if binary not staged
```

### Environment variable reference

| Variable | Description |
|---|---|
| `MOCKLY_BINARY_PATH` | Absolute path to a pre-existing binary. Skips all download logic. |
| `MOCKLY_DOWNLOAD_BASE_URL` | Base URL override for binary downloads (Artifactory / mirrors). |
| `MOCKLY_VERSION` | Binary version to install. Default: `v0.1.0`. |
| `MOCKLY_NO_INSTALL` | If set, fail with instructions instead of downloading. |
| `HTTPS_PROXY` / `HTTP_PROXY` | Route downloads through an HTTP proxy (supports CONNECT). |

---

## Requirements

- Node.js ≥ 18
- Platforms: Linux (x64/arm64), macOS (x64/arm64), Windows (x64)

## License

MIT
