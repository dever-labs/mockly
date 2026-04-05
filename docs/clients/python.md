# Mockly — Python Client

The Python client starts, controls, and stops a Mockly process from your pytest tests.

## Install

```sh
pip install mockly-driver
```

## Quickstart

```python
from mockly_driver import MocklyServer, Mock, MockRequest, MockResponse

server = MocklyServer.ensure()

server.add_mock(Mock(
    id="get-user",
    request=MockRequest(method="GET", path="/users/1"),
    response=MockResponse(
        status=200,
        body='{"id":1,"name":"Alice"}',
        headers={"Content-Type": "application/json"},
    ),
))

# Point your service under test at server.http_base
# e.g. "http://127.0.0.1:45678"

server.stop()
```

## Factory methods

| Method | Description |
|---|---|
| `MocklyServer.ensure(**install_kwargs)` | Downloads the binary if not present, then starts the server. **Recommended for most cases.** |
| `MocklyServer.create(scenarios=None)` | Starts using an already-installed binary. Raises `RuntimeError` if binary is not found. |

Both retry up to 3 times on ephemeral port conflicts.

## Configuration

```python
from mockly_driver import MocklyServer, Scenario, ScenarioPatch

server = MocklyServer.ensure(
    scenarios=[
        Scenario(
            id="payment-fail",
            name="Payment Failure",
            patches=[
                ScenarioPatch(mock_id="charge", status=503, body='{"error":"unavailable"}'),
            ],
        ),
    ],
)
```

## API reference

### Mocks

```python
from mockly_driver import Mock, MockRequest, MockResponse

# Add a mock
server.add_mock(Mock(
    id="get-orders",
    request=MockRequest(
        method="GET",
        path="/orders",
        headers={"Authorization": "Bearer *"},
    ),
    response=MockResponse(
        status=200,
        body='[{"id":1}]',
        headers={"Content-Type": "application/json"},
        delay="100ms",
    ),
))

# Remove a mock
server.delete_mock("get-orders")
```

### Scenarios

```python
# Activate a pre-configured scenario
server.activate_scenario("payment-fail")

# Deactivate it
server.deactivate_scenario("payment-fail")
```

### Fault injection

```python
from mockly_driver import FaultConfig

# Add latency and override status codes on all requests
server.set_fault(FaultConfig(
    enabled=True,
    delay="500ms",
    status_override=503,
    error_rate=0.5,  # 50% of requests
))

# Remove the fault
server.clear_fault()
```

### Reset and stop

```python
# Reset all dynamic mocks, active scenarios, and faults; keeps startup config
server.reset()

# Kill the process
server.stop()
```

## Integration with pytest

### Session-scoped fixture (recommended)

```python
# conftest.py
import pytest
from mockly_driver import MocklyServer

@pytest.fixture(scope="session")
def mockly():
    server = MocklyServer.ensure()
    yield server
    server.stop()

@pytest.fixture(autouse=True)
def reset_mockly(mockly):
    yield
    mockly.reset()  # clean state between tests
```

```python
# test_payment.py
from mockly_driver import Mock, MockRequest, MockResponse

def test_returns_user(mockly):
    mockly.add_mock(Mock(
        id="get-user",
        request=MockRequest(method="GET", path="/users/1"),
        response=MockResponse(status=200, body='{"id":1,"name":"Alice"}'),
    ))
    # ... call your service at mockly.http_base ...

def test_handles_503_via_scenario(mockly):
    mockly.add_mock(Mock(
        id="charge",
        request=MockRequest(method="POST", path="/charge"),
        response=MockResponse(status=200, body='{"ok":true}'),
    ))
    mockly.activate_scenario("payment-fail")
    # ... assert your service handles 503 gracefully ...
```

### Per-test server

```python
@pytest.fixture
def isolated_mockly():
    server = MocklyServer.ensure()
    yield server
    server.stop()

def test_isolated(isolated_mockly):
    isolated_mockly.add_mock(...)
```

## Server properties

| Attribute | Description |
|---|---|
| `server.http_base` | Base URL of the mock HTTP server, e.g. `http://127.0.0.1:45123` |
| `server.api_base` | Base URL of the management API, e.g. `http://127.0.0.1:45124` |
| `server.http_port` | Numeric HTTP port |
| `server.api_port` | Numeric API port |
