# Mockly — Node.js / TypeScript Client

The Node.js client starts, controls, and stops a Mockly process from your JavaScript or TypeScript tests.

## Install

```sh
npm install --save-dev @dever-labs/mockly-driver
# or
yarn add --dev @dever-labs/mockly-driver
```

The package automatically selects the correct platform binary (`@dever-labs/mockly-driver-{platform}`) as an optional dependency.

## Quickstart

```ts
import { MocklyServer } from '@dever-labs/mockly-driver'

const server = await MocklyServer.ensure()

await server.addMock({
  id: 'get-user',
  request: { method: 'GET', path: '/users/1' },
  response: {
    status: 200,
    body: JSON.stringify({ id: 1, name: 'Alice' }),
    headers: { 'Content-Type': 'application/json' },
  },
})

// Point your service under test at server.httpBase
const res = await fetch(`${server.httpBase}/users/1`)

await server.stop()
```

## Factory methods

| Method | Description |
|---|---|
| `MocklyServer.ensure(opts?)` | Downloads the binary if not present, then starts the server. **Recommended for most cases.** |
| `MocklyServer.create(opts?)` | Starts using an already-installed binary. Throws if the binary is not found. |

Both retry up to 3 times on ephemeral port conflicts.

## Configuration

```ts
const server = await MocklyServer.ensure({
  // Pre-load scenarios at startup
  scenarios: [
    {
      id: 'payment-fail',
      name: 'Payment Failure',
      patches: [
        { mock_id: 'charge', status: 503, body: '{"error":"unavailable"}' },
      ],
    },
  ],
  // Override install location
  binDir: '/opt/mockly',
})
```

## API reference

### Mocks

```ts
// Add a mock
await server.addMock({
  id: 'get-orders',
  request: {
    method: 'GET',
    path: '/orders',
    headers: { Authorization: 'Bearer *' },
  },
  response: {
    status: 200,
    body: '[{"id":1}]',
    headers: { 'Content-Type': 'application/json' },
    delay: '100ms',
  },
})

// Remove a mock
await server.deleteMock('get-orders')
```

### Scenarios

```ts
// Activate a pre-configured scenario
await server.activateScenario('payment-fail')

// Deactivate it
await server.deactivateScenario('payment-fail')
```

### Fault injection

```ts
// Add latency and override status codes on all requests
await server.setFault({
  enabled: true,
  delay: '500ms',
  status_override: 503,
  error_rate: 0.5, // 50% of requests
})

// Remove the fault
await server.clearFault()
```

### Reset and stop

```ts
// Reset all dynamic mocks, active scenarios, and faults; keeps startup config
await server.reset()

// Kill the process
await server.stop()
```

## Integration with Jest

```ts
// tests/integration.test.ts
import { MocklyServer } from '@dever-labs/mockly-driver'

let server: MocklyServer

beforeAll(async () => {
  server = await MocklyServer.ensure()
})

afterAll(async () => {
  await server.stop()
})

beforeEach(async () => {
  await server.reset() // isolate each test
})

test('returns user from mock', async () => {
  await server.addMock({
    id: 'get-user',
    request: { method: 'GET', path: '/users/1' },
    response: { status: 200, body: '{"id":1,"name":"Alice"}' },
  })

  const res = await fetch(`${server.httpBase}/users/1`)
  expect(res.status).toBe(200)
})

test('handles 503 via scenario', async () => {
  await server.addMock({
    id: 'charge',
    request: { method: 'POST', path: '/charge' },
    response: { status: 200, body: '{"ok":true}' },
  })
  await server.activateScenario('payment-fail')

  const res = await fetch(`${server.httpBase}/charge`, { method: 'POST' })
  expect(res.status).toBe(503)
})
```

## Integration with Vitest

```ts
// vitest.config.ts
import { defineConfig } from 'vitest/config'
export default defineConfig({ test: { globalSetup: './tests/setup.ts' } })

// tests/setup.ts
import { MocklyServer } from '@dever-labs/mockly-driver'

let server: MocklyServer

export async function setup() {
  server = await MocklyServer.ensure()
  process.env.MOCK_BASE = server.httpBase
}

export async function teardown() {
  await server.stop()
}
```

## Server properties

| Property | Description |
|---|---|
| `server.httpBase` | Base URL of the mock HTTP server, e.g. `http://127.0.0.1:45123` |
| `server.apiBase` | Base URL of the management API, e.g. `http://127.0.0.1:45124` |
| `server.httpPort` | Numeric HTTP port |
| `server.apiPort` | Numeric API port |
