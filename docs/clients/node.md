# Mockly — Node.js / TypeScript Client

The Node.js client starts, controls, and stops a Mockly process from your JavaScript or TypeScript tests.

## Install

```sh
npm install --save-dev @dever-labs/mockly-driver
# or
yarn add --dev @dever-labs/mockly-driver
```

The Mockly binary is downloaded automatically for your platform when you run `npm install`. You can also trigger it manually:

```sh
npx mockly-install
```

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

// Inspect the currently registered mocks
const mocks = await server.listMocks()

// Replace a mock definition
const updated = await server.updateMock('get-orders', {
  id: 'get-orders',
  request: { method: 'GET', path: '/orders' },
  response: {
    status: 200,
    body: '[{"id":1},{"id":2}]',
    headers: { 'Content-Type': 'application/json' },
  },
})

// Patch only the response fields you want to change
const patched = await server.patchMock('get-orders', {
  status: 201,
  body: '[]',
  headers: { 'X-Mock-Version': 'v2' },
  delay: '250ms',
})

// Remove a mock
await server.deleteMock('get-orders')
```

### Scenarios

```ts
const createdScenario = await server.createScenario({
  id: 'slow-checkout',
  name: 'Slow checkout',
  description: 'Used for retry-path tests',
  patches: [
    { mock_id: 'charge', status: 503, delay: '750ms' },
  ],
})

const scenarios = await server.listScenarios()
const loadedScenario = await server.getScenario('slow-checkout')

const updatedScenario = await server.updateScenario('slow-checkout', {
  ...loadedScenario,
  name: 'Slow checkout v2',
})

// Activate a scenario before exercising your service
await server.activateScenario('slow-checkout')
const activeScenarios = await server.listActiveScenarios()
console.log(activeScenarios.active)

// Deactivate or delete it when you're done
await server.deactivateScenario('slow-checkout')
await server.deleteScenario('slow-checkout')
```

### Call verification

```ts
const summary = await server.waitForCalls('get-orders', 2, '5s')
if (summary.count !== 2) {
  throw new Error(`expected 2 calls, got ${summary.count}`)
}

const latestCalls = await server.getCalls('get-orders')
console.log(latestCalls.calls[0]?.path)

await server.clearCalls('get-orders')
await server.clearAllCalls()
```

### State

```ts
const state = await server.getState()
console.log(state['order-status'])

const updatedState = await server.setState({
  'order-status': 'pending',
  'retry-count': '1',
})
console.log(updatedState['retry-count'])

await server.deleteState('retry-count')
```

### Logs

```ts
const allLogs = await server.getLogs()
const matchedLogs = await server.getLogs('get-orders')

const totalLogs = await server.getLogsCount()
const matchedCount = await server.getLogsCount('get-orders')
console.log({ totalLogs, matchedCount, firstPath: allLogs[0]?.path, firstMatch: matchedLogs[0]?.matched_id })

await server.clearLogs()
```

### Fault injection

```ts
// Add latency and override status codes on all requests
await server.setFault({
  enabled: true,
  delay: '500ms',
  status: 503,
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

## Testcontainers

Mockly also ships a Docker-backed Node.js / TypeScript testcontainers module: `@dever-labs/mockly-testcontainers`.

Use it instead of the driver when you want Docker-managed lifecycle, no local binary download, and the same container image in local tests and CI.

### Install

```sh
npm i -D @dever-labs/mockly-testcontainers testcontainers
```

### Example

```ts
import assert from 'node:assert/strict'
import { MocklyContainerBuilder } from '@dever-labs/mockly-testcontainers'

const container = await new MocklyContainerBuilder().start()

try {
  await container.addMock({
    id: 'get-user',
    request: { method: 'GET', path: '/users/1' },
    response: { status: 200, body: '{"id":1}' },
  })

  const response = await fetch(`${container.getHttpBase()}/users/1`)
  assert.equal(response.status, 200)
  assert.equal(await response.text(), '{"id":1}')
} finally {
  await container.stop()
}
```

### Key API

- `MocklyContainerBuilder.withInlineConfig(yaml)`
- `StartedMocklyContainer.getHttpBase()` / `getApiBase()`
- `addMock`, `deleteMock`, `reset`
- `activateScenario`, `deactivateScenario`
- `setFault`, `clearFault`

### Requirements

- Node 18+
- Docker

See `clients/node-testcontainers/README.md` for the full module reference.
