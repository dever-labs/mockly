# mockly-testcontainers

Run Mockly in Docker-backed Python tests with `testcontainers-python`.

The package starts `ghcr.io/dever-labs/mockly:latest`, waits for the management API to be ready, and provides helpers for mocks, scenarios, faults, and logs.

## Requirements

- Python 3.10+
- Docker

## Install

```sh
pip install mockly-testcontainers
```

## Quickstart

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

## When to use the testcontainers module

Use `mockly-testcontainers` when you want Docker-managed lifecycle, no native binary download, or the same Mockly image in local tests and CI.

Use `mockly-driver` when you want to run the native Mockly binary directly from Python.

## Construction and configuration

`MocklyContainer` extends `testcontainers.core.container.DockerContainer`.

| API | Description |
|---|---|
| `MocklyContainer(image=DEFAULT_IMAGE)` | Create a container using the default Mockly image, or override it with another image name. |
| `with_inline_config(yaml)` | Replace `/config/mockly.yaml` with inline YAML before startup. |
| `start()` | Start the container and wait for the management API. |
| `stop()` | Stop the container and clean up the generated config file. |

### Custom YAML config

```python
with MocklyContainer().with_inline_config("""mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
""") as container:
    ...
```

## Management methods

| Method | Description |
|---|---|
| `get_http_base()` | Base URL of the mock HTTP server. |
| `get_api_base()` | Base URL of the management API. |
| `add_mock(mock)` | Register a dynamic HTTP mock. |
| `delete_mock(mock_id)` | Delete a mock by ID. |
| `reset()` | Remove dynamic mocks, deactivate scenarios, and clear faults. |
| `activate_scenario(scenario_id)` | Activate a configured scenario. |
| `deactivate_scenario(scenario_id)` | Deactivate a configured scenario. |
| `set_fault(config)` | Apply a global HTTP fault. |
| `clear_fault()` | Remove the active fault. |
| `get_logs()` | Read request logs as JSON. |
| `clear_logs()` | Clear stored request logs. |

## Exported types

The package exports these dataclasses:

- `Mock`
- `MockRequest`
- `MockResponse`
- `FaultConfig`
