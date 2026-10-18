# mockly-driver

PHP client for [Mockly](https://github.com/dever-labs/mockly) — download, start, and control a Mockly mock HTTP server from your PHPUnit test suite.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Environment Variables](#environment-variables)
- [Proxy / Artifactory / Air-gap](#proxy--artifactory--air-gap)

---

## Installation

```bash
composer require dever-labs/mockly-driver
```

Then download the Mockly binary once, e.g. in a PHPUnit bootstrap script:

```php
<?php
use Mockly\Driver\Installer;

Installer::install();
// binary is placed in ./bin/mockly (or ./bin/mockly.exe on Windows)
```

---

## Quick Start

### PHPUnit

```php
use Mockly\Driver\MocklyServer;
use Mockly\Driver\Model\Mock;
use Mockly\Driver\Model\MockRequest;
use Mockly\Driver\Model\MockResponse;
use PHPUnit\Framework\TestCase;

final class MyServiceTest extends TestCase
{
    private static MocklyServer $mockly;

    public static function setUpBeforeClass(): void
    {
        // Downloads the binary if needed, then starts the server.
        self::$mockly = MocklyServer::ensure();
    }

    public static function tearDownAfterClass(): void
    {
        self::$mockly->stop();
    }

    protected function tearDown(): void
    {
        self::$mockly->reset();
    }

    public function testMyService(): void
    {
        self::$mockly->addMock(new Mock(
            id: 'get-users',
            request: new MockRequest(method: 'GET', path: '/users'),
            response: new MockResponse(status: 200, body: '[{"id":1}]',
                headers: ['Content-Type' => 'application/json']),
        ));

        // Your service under test talks to self::$mockly->httpBase
        $body = file_get_contents(self::$mockly->httpBase . '/users');
        self::assertStringContainsString('"id":1', $body);
    }
}
```

### Scenarios

```php
use Mockly\Driver\Model\Scenario;
use Mockly\Driver\Model\ScenarioPatch;

$server = MocklyServer::ensure(scenarios: [
    new Scenario(
        id: 'slow-api',
        name: 'Slow API',
        patches: [new ScenarioPatch(mockId: 'get-users', delay: '2s')],
    ),
]);

$server->activateScenario('slow-api');
// ... run slow-path tests ...
$server->deactivateScenario('slow-api');
```

### Faults

```php
use Mockly\Driver\Model\FaultConfig;

$server->setFault(new FaultConfig(enabled: true, status: 503, errorRate: 0.5));
// ... run fault-tolerance tests ...
$server->clearFault();
```

---

## API Reference

### `MocklyServer::ensure(?array $scenarios = null, ...): MocklyServer`

Downloads the binary (if not already present) and starts the server. Extra arguments
(`$version`, `$baseUrl`, `$binDir`) are forwarded to `Installer::install()`.

### `MocklyServer::create(?array $scenarios = null): MocklyServer`

Starts the server using an already-installed binary. Throws `RuntimeException` if no
binary is found.

### `$server->stop(): void`

Kills the Mockly process and removes the temp config file.

### `$server->addMock(Mock $mock): void`

Registers a new HTTP mock at runtime.

### `$server->listMocks(): array` / `updateMock()` / `patchMock()` / `deleteMock()`

List, update (full replace), patch (partial update), or remove mocks by ID.

### `$server->reset(): void`

Removes all dynamic mocks, deactivates all scenarios, and clears any active fault.

### `$server->activateScenario(string $scenarioId): void` / `deactivateScenario()`

Activates or deactivates a pre-configured scenario.

### `$server->setFault(FaultConfig $config): void` / `clearFault()`

Enables or removes a fault injection policy (latency, forced status, error rate).

### `$server->getCalls(string $mockId): CallSummary` / `clearCalls()` / `clearAllCalls()` / `waitForCalls()`

Inspect, clear, or poll for recorded calls against a mock.

### `$server->getState(): array` / `setState()` / `deleteState()`

Read, write, or delete server-side key/value state.

### `$server->getLogs(?string $matchedId = null): array` / `clearLogs()` / `getLogsCount()`

Retrieve or clear request logs, optionally filtered by matched mock ID.

### `Installer::install(?string $version = null, ?string $baseUrl = null, ?string $binDir = null, bool $force = false): string`

Downloads the Mockly binary for the current platform. Returns the path.

### `Installer::getBinaryPath(?string $binDir = null): ?string`

Returns the path to an existing binary, or `null` if not found.

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `MOCKLY_BINARY_PATH` | Absolute path to a pre-staged binary — skips all download logic | — |
| `MOCKLY_VERSION` | Binary version to download | `v0.16.0`<!-- x-release-please-version --> |
| `MOCKLY_DOWNLOAD_BASE_URL` | Override the GitHub releases base URL (for mirrors/Artifactory) | `https://github.com/dever-labs/mockly/releases/download` |
| `MOCKLY_NO_INSTALL` | If set, `Installer::install()` throws instead of downloading | — |
| `HTTPS_PROXY` / `HTTP_PROXY` | Route downloads through an HTTP proxy | — |

---

## Proxy / Artifactory / Air-gap

### HTTP/HTTPS proxy

```bash
export HTTPS_PROXY=http://proxy.corp.example.com:3128
```

### Internal mirror / Artifactory

```bash
export MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.corp.example.com/mockly/releases/download
```

### Air-gap / pre-staged binary

```bash
export MOCKLY_BINARY_PATH=/opt/mockly/mockly
export MOCKLY_NO_INSTALL=1   # optional: prevents accidental downloads
```

---

## License

MIT © dever-labs
