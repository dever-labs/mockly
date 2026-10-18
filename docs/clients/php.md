# Mockly — PHP Client

The PHP client starts, controls, and stops a Mockly process from your PHPUnit tests.

## Install

```sh
composer require --dev dever-labs/mockly-driver
```

## Quickstart

```php
use Mockly\Driver\MocklyServer;
use Mockly\Driver\Model\{Mock, MockRequest, MockResponse};

$server = MocklyServer::ensure();

$server->addMock(new Mock(
    id: 'get-user',
    request: new MockRequest(method: 'GET', path: '/users/1'),
    response: new MockResponse(
        status: 200,
        body: '{"id":1,"name":"Alice"}',
        headers: ['Content-Type' => 'application/json'],
    ),
));

// Point your service under test at $server->httpBase
// e.g. "http://127.0.0.1:45678"

$server->stop();
```

## Factory methods

| Method | Description |
|---|---|
| `MocklyServer::ensure(?array $scenarios = null, ?string $version = null, ?string $baseUrl = null, ?string $binDir = null)` | Downloads the binary if not present, then starts the server. **Recommended for most cases.** |
| `MocklyServer::create(?array $scenarios = null)` | Starts using an already-installed binary. Throws `RuntimeException` if binary is not found. |

Both retry up to 3 times on ephemeral port conflicts.

## Configuration

```php
use Mockly\Driver\MocklyServer;
use Mockly\Driver\Model\{Scenario, ScenarioPatch};

$server = MocklyServer::ensure(scenarios: [
    new Scenario(
        id: 'payment-fail',
        name: 'Payment Failure',
        patches: [
            new ScenarioPatch(mockId: 'charge', status: 503, body: '{"error":"unavailable"}'),
        ],
    ),
]);
```

## API reference

### Mocks

```php
use Mockly\Driver\Model\{Mock, MockRequest, MockResponse, MockResponsePatch};

// Add a mock
$server->addMock(new Mock(
    id: 'get-orders',
    request: new MockRequest(
        method: 'GET',
        path: '/orders',
        headers: ['Authorization' => '******'],
    ),
    response: new MockResponse(
        status: 200,
        body: '[{"id":1}]',
        headers: ['Content-Type' => 'application/json'],
        delay: '100ms',
    ),
));

// Inspect the currently registered mocks
$mocks = $server->listMocks();

// Replace a mock definition
$updated = $server->updateMock('get-orders', new Mock(
    id: 'get-orders',
    request: new MockRequest(method: 'GET', path: '/orders'),
    response: new MockResponse(
        status: 200,
        body: '[{"id":1},{"id":2}]',
        headers: ['Content-Type' => 'application/json'],
    ),
));

// Patch only the response fields you want to change
$patched = $server->patchMock('get-orders', new MockResponsePatch(
    status: 201,
    body: '[]',
    headers: ['X-Mock-Version' => 'v2'],
    delay: '250ms',
));

// Remove a mock
$server->deleteMock('get-orders');
```

### Scenarios

```php
use Mockly\Driver\Model\{Scenario, ScenarioPatch};

$createdScenario = $server->createScenario(new Scenario(
    id: 'slow-checkout',
    name: 'Slow checkout',
    description: 'Used for retry-path tests',
    patches: [
        new ScenarioPatch(mockId: 'charge', status: 503, delay: '750ms'),
    ],
));

$scenarios = $server->listScenarios();
$loadedScenario = $server->getScenario('slow-checkout');

$updatedScenario = $server->updateScenario('slow-checkout', new Scenario(
    id: $loadedScenario->id,
    name: 'Slow checkout v2',
    patches: $loadedScenario->patches,
    description: $loadedScenario->description,
));

// Activate a scenario before exercising your service
$server->activateScenario('slow-checkout');
$activeScenarios = $server->listActiveScenarios();
print_r($activeScenarios->active);

// Deactivate or delete it when you're done
$server->deactivateScenario('slow-checkout');
$server->deleteScenario('slow-checkout');
```

### Call verification

```php
$summary = $server->waitForCalls('get-orders', count: 2, timeoutSeconds: 5);
assert($summary->count === 2);

$latestCalls = $server->getCalls('get-orders');
echo $latestCalls->calls[0]->path;

$server->clearCalls('get-orders');
$server->clearAllCalls();
```

### State

```php
$state = $server->getState();
echo $state['order-status'] ?? null;

$updatedState = $server->setState([
    'order-status' => 'pending',
    'retry-count' => '1',
]);
echo $updatedState['retry-count'];

$server->deleteState('retry-count');
```

### Logs

```php
$allLogs = $server->getLogs();
$matchedLogs = $server->getLogs('get-orders');

echo $server->getLogsCount();
echo $server->getLogsCount('get-orders');
echo $allLogs[0]?->path;
echo $matchedLogs[0]?->matchedId;

$server->clearLogs();
```

### Fault injection

```php
use Mockly\Driver\Model\FaultConfig;

// Add latency and override status codes on all requests
$server->setFault(new FaultConfig(
    enabled: true,
    delay: '500ms',
    status: 503,
    errorRate: 0.5, // 50% of requests
));

// Remove the fault
$server->clearFault();
```

### Reset and stop

```php
// Reset all dynamic mocks, active scenarios, and faults; keeps startup config
$server->reset();

// Kill the process
$server->stop();
```

## Integration with PHPUnit

### Shared server across a test class

```php
use Mockly\Driver\MocklyServer;
use Mockly\Driver\Model\{Mock, MockRequest, MockResponse};
use PHPUnit\Framework\TestCase;

final class PaymentTest extends TestCase
{
    private static MocklyServer $mockly;

    public static function setUpBeforeClass(): void
    {
        self::$mockly = MocklyServer::ensure();
    }

    public static function tearDownAfterClass(): void
    {
        self::$mockly->stop();
    }

    protected function tearDown(): void
    {
        self::$mockly->reset(); // clean state between tests
    }

    public function testReturnsUser(): void
    {
        self::$mockly->addMock(new Mock(
            id: 'get-user',
            request: new MockRequest(method: 'GET', path: '/users/1'),
            response: new MockResponse(status: 200, body: '{"id":1,"name":"Alice"}'),
        ));
        // ... call your service at self::$mockly->httpBase ...
    }

    public function testHandles503ViaScenario(): void
    {
        self::$mockly->addMock(new Mock(
            id: 'charge',
            request: new MockRequest(method: 'POST', path: '/charge'),
            response: new MockResponse(status: 200, body: '{"ok":true}'),
        ));
        self::$mockly->activateScenario('payment-fail');
        // ... assert your service handles 503 gracefully ...
    }
}
```

### Per-test server

```php
protected function setUp(): void
{
    $this->isolatedMockly = MocklyServer::ensure();
}

protected function tearDown(): void
{
    $this->isolatedMockly->stop();
}
```

## Server properties

| Property | Description |
|---|---|
| `$server->httpBase` | Base URL of the mock HTTP server, e.g. `http://127.0.0.1:45123` |
| `$server->apiBase` | Base URL of the management API, e.g. `http://127.0.0.1:45124` |
| `$server->httpPort` | Numeric HTTP port |
| `$server->apiPort` | Numeric API port |

## Testcontainers

Mockly also ships a Docker-backed PHP testcontainers module: `dever-labs/mockly-testcontainers`.

Use it instead of the driver when you want Docker-managed lifecycle, no local binary download, and the same container image in local tests and CI.

### Install

```sh
composer require --dev dever-labs/mockly-testcontainers
```

### Example

```php
use Mockly\Testcontainers\MocklyContainer;
use Mockly\Testcontainers\Model\{Mock, MockRequest, MockResponse};

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

### Key API

- `MocklyContainer::withInlineConfig(string $yaml)`
- `getHttpBase()` / `getApiBase()`
- `addMock()`, `deleteMock()`, `reset()`
- `activateScenario()`, `deactivateScenario()`
- `setFault()`, `clearFault()`

### Requirements

- PHP 8.1+
- Docker

See `clients/php-testcontainers/README.md` for the full module reference.
