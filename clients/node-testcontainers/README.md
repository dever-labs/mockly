# @dever-labs/mockly-testcontainers

Run Mockly in Docker-backed Node.js and TypeScript tests with `testcontainers`.

The package starts `ghcr.io/dever-labs/mockly:latest`, waits for the management API to be ready, and exposes helper methods for mocks, scenarios, faults, and logs.

## Requirements

- Node 18+
- Docker
- `testcontainers` 10+

## Install

```sh
npm i -D @dever-labs/mockly-testcontainers testcontainers
```

## Quickstart

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

## When to use the testcontainers module

Use `@dever-labs/mockly-testcontainers` when you want Docker-managed lifecycle, no native binary download, or a consistent Mockly environment across laptops and CI.

Use `@dever-labs/mockly-driver` when you want to run the Mockly binary directly in the test process instead.

## Builder API

`MocklyContainerBuilder` configures the container before startup.

| Method | Description |
|---|---|
| `new MocklyContainerBuilder()` | Create a builder with the default image and default config. |
| `withImage(image)` | Override the Docker image. |
| `withInlineConfig(yaml)` | Replace `/config/mockly.yaml` with inline YAML. |
| `start()` | Start the container and return a `StartedMocklyContainer`. |

### Custom YAML config

```ts
const container = await new MocklyContainerBuilder()
  .withInlineConfig(`mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
`)
  .start()
```

## Started container API

| Method | Description |
|---|---|
| `getHttpBase()` | Base URL of the mock HTTP server. |
| `getApiBase()` | Base URL of the management API. |
| `stop()` | Stop and remove the container. |
| `addMock(mock)` | Register a dynamic HTTP mock. |
| `deleteMock(id)` | Delete a mock by ID. |
| `reset()` | Remove dynamic mocks, deactivate scenarios, and clear faults. |
| `activateScenario(id)` | Activate a configured scenario. |
| `deactivateScenario(id)` | Deactivate a configured scenario. |
| `setFault(config)` | Apply a global HTTP fault. |
| `clearFault()` | Remove the active fault. |
| `getLogs()` | Read request logs as JSON. |
| `clearLogs()` | Clear stored request logs. |

## Exported types

The package also exports these TypeScript types:

- `HttpMock`
- `MockRequest`
- `MockResponse`
- `FaultConfig`
