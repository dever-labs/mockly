# mockly-testcontainers

Run Mockly in Docker-backed PHPUnit tests using `testcontainers/testcontainers`.

The package starts `ghcr.io/dever-labs/mockly:latest`, waits for the management API to be ready, and provides helpers for mocks, scenarios, faults, state, logs, and calls.

## Requirements

- PHP 8.1+
- Docker

## Install

```bash
composer require --dev dever-labs/mockly-testcontainers
```

## Quickstart

```php
use Mockly\Testcontainers\MocklyContainer;
use Mockly\Testcontainers\Model\Mock;
use Mockly\Testcontainers\Model\MockRequest;
use Mockly\Testcontainers\Model\MockResponse;

$container = (new MocklyContainer())->start();

$container->addMock(new Mock(
    id: 'get-user',
    request: new MockRequest(method: 'GET', path: '/users/1'),
    response: new MockResponse(status: 200, body: '{"id":1}'),
));

$body = file_get_contents($container->getHttpBase() . '/users/1');
assert($body === '{"id":1}');

$container->stop();
```

## When to use the testcontainers module

Use `mockly-testcontainers` when you want Docker-managed lifecycle, no native binary
download, or the same Mockly image in local tests and CI.

Use `mockly-driver` when you want to run the native Mockly binary directly from PHP.

## Construction and configuration

| API | Description |
|---|---|
| `new MocklyContainer(string $image = MocklyContainer::DEFAULT_IMAGE)` | Create a container using the default Mockly image, or override it with another image name. |
| `withInlineConfig(string $yaml): self` | Replace `/config/mockly.yaml` with inline YAML before startup. |
| `withScenarios(?array $scenarios): self` | Render the config with the given pre-configured scenarios. |
| `start(): self` | Copy the config into the container, start it, and wait for the management API. |
| `stop(): void` | Stop the container. |

### Custom YAML config

```php
$container = (new MocklyContainer())
    ->withInlineConfig(<<<YAML
    mockly:
      api:
        port: 9091
    protocols:
      http:
        enabled: true
        port: 8090
    YAML)
    ->start();
```

## Management methods

| Method | Description |
|---|---|
| `getHttpBase(): string` | Base URL of the mock HTTP server. |
| `getApiBase(): string` | Base URL of the management API. |
| `addMock(Mock $mock): void` | Register a dynamic HTTP mock. |
| `listMocks(): array` / `updateMock()` / `patchMock()` / `deleteMock()` | List, update, patch, or remove mocks. |
| `reset(): void` | Remove dynamic mocks, deactivate scenarios, and clear faults. |
| `activateScenario(string $scenarioId): void` / `deactivateScenario()` | Toggle a configured scenario. |
| `setFault(FaultConfig $config): void` / `clearFault()` | Apply or remove a global HTTP fault. |
| `getCalls()` / `clearCalls()` / `clearAllCalls()` / `waitForCalls()` | Inspect or poll recorded calls. |
| `getState()` / `setState()` / `deleteState()` | Read, write, or delete server-side key/value state. |
| `getLogs()` / `clearLogs()` / `getLogsCount()` | Retrieve or clear request logs. |

## Exported types

The package exports these readonly value objects under `Mockly\Testcontainers\Model\`:

- `Mock`, `MockRequest`, `MockResponse`, `MockResponsePatch`
- `Scenario`, `ScenarioPatch`, `ActiveScenariosResponse`
- `FaultConfig`
- `CallEntry`, `CallSummary`

## Running tests

```bash
composer install
composer test                       # unit tests only
vendor/bin/phpunit --group integration   # requires Docker
```

---

## License

MIT © dever-labs
