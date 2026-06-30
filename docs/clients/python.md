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
from mockly_driver import Mock, MockRequest, MockResponse, MockResponsePatch

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

# Inspect the currently registered mocks
mocks = server.list_mocks()

# Replace a mock definition
updated = server.update_mock("get-orders", Mock(
    id="get-orders",
    request=MockRequest(method="GET", path="/orders"),
    response=MockResponse(
        status=200,
        body='[{"id":1},{"id":2}]',
        headers={"Content-Type": "application/json"},
    ),
))

# Patch only the response fields you want to change
patched = server.patch_mock("get-orders", MockResponsePatch(
    status=201,
    body='[]',
    headers={"X-Mock-Version": "v2"},
    delay="250ms",
))

# Remove a mock
server.delete_mock("get-orders")
```

### Scenarios

```python
from mockly_driver import Scenario, ScenarioPatch

created_scenario = server.create_scenario(Scenario(
    id="slow-checkout",
    name="Slow checkout",
    description="Used for retry-path tests",
    patches=[
        ScenarioPatch(mock_id="charge", status=503, delay="750ms"),
    ],
))

scenarios = server.list_scenarios()
loaded_scenario = server.get_scenario("slow-checkout")

updated_scenario = server.update_scenario("slow-checkout", Scenario(
    id=loaded_scenario.id,
    name="Slow checkout v2",
    description=loaded_scenario.description,
    patches=loaded_scenario.patches,
))

# Activate a scenario before exercising your service
server.activate_scenario("slow-checkout")
active_scenarios = server.list_active_scenarios()
print(active_scenarios.active)

# Deactivate or delete it when you're done
server.deactivate_scenario("slow-checkout")
server.delete_scenario("slow-checkout")
```

### Call verification

```python
summary = server.wait_for_calls("get-orders", count=2, timeout_seconds=5)
assert summary.count == 2

latest_calls = server.get_calls("get-orders")
print(latest_calls.calls[0].path)

server.clear_calls("get-orders")
server.clear_all_calls()
```

### State

```python
state = server.get_state()
print(state.get("order-status"))

updated_state = server.set_state({
    "order-status": "pending",
    "retry-count": "1",
})
print(updated_state["retry-count"])

server.delete_state("retry-count")
```

### Logs

```python
all_logs = server.get_logs()
matched_logs = server.get_logs("get-orders")

print(server.get_logs_count())
print(server.get_logs_count("get-orders"))
print(all_logs[0].path if all_logs else None)
print(matched_logs[0].matched_id if matched_logs else None)

server.clear_logs()
```

### Fault injection

```python
from mockly_driver import FaultConfig

# Add latency and override status codes on all requests
server.set_fault(FaultConfig(
    enabled=True,
    delay="500ms",
    status=503,
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

## Testcontainers

Mockly also ships a Docker-backed Python testcontainers module: `mockly-testcontainers`.

Use it instead of the driver when you want Docker-managed lifecycle, no local binary download, and the same container image in local tests and CI.

### Install

```sh
pip install mockly-testcontainers
```

### Example

```python
import urllib.request

from mockly_testcontainers import Mock, MockRequest, MockResponse, MocklyContainer

with MocklyContainer() as container:
    container.add_mock(
        Mock(
            id="get-user",
            request=MockRequest(method="GET", path="/users/1"),
            response=MockResponse(status=200, body='{"id":1}'),
        )
    )

    with urllib.request.urlopen(f"{container.get_http_base()}/users/1") as response:
        assert response.status == 200
        assert response.read().decode() == '{"id":1}'
```

### Key API

- `MocklyContainer.with_inline_config(yaml)`
- `get_http_base()` / `get_api_base()`
- `add_mock`, `delete_mock`, `reset`
- `activate_scenario`, `deactivate_scenario`
- `set_fault`, `clear_fault`

### Requirements

- Python 3.10+
- Docker

See `clients/python-testcontainers/README.md` for the full module reference.
