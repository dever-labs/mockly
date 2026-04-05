# mockly-driver

Python client for [Mockly](https://github.com/dever-labs/mockly) — download, start, and control a Mockly mock HTTP server from your Python/pytest test suite.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Environment Variables](#environment-variables)
- [Proxy / Artifactory / Air-gap](#proxy--artifactory--air-gap)
- [CLI](#cli)

---

## Installation

```bash
pip install mockly-driver
```

Then download the Mockly binary once:

```bash
mockly-install
# binary is placed in ./bin/mockly (or ./bin/mockly.exe on Windows)
```

---

## Quick Start

### pytest fixture

```python
import pytest
from mockly_driver import MocklyServer, Mock, MockRequest, MockResponse

@pytest.fixture(scope="session")
def mockly():
    # Downloads binary if needed, then starts the server
    server = MocklyServer.ensure()
    yield server
    server.stop()

def test_my_service(mockly):
    mockly.add_mock(Mock(
        id="get-users",
        request=MockRequest(method="GET", path="/users"),
        response=MockResponse(status=200, body='[{"id":1}]',
                              headers={"Content-Type": "application/json"}),
    ))

    # Your service under test talks to mockly.http_base
    import urllib.request
    with urllib.request.urlopen(f"{mockly.http_base}/users") as resp:
        assert resp.status == 200

    mockly.reset()
```

### Scenarios

```python
from mockly_driver import Scenario, ScenarioPatch

server = MocklyServer.ensure(
    scenarios=[
        Scenario(
            id="slow-api",
            name="Slow API",
            patches=[ScenarioPatch(mock_id="get-users", delay="2s")],
        )
    ]
)

server.activate_scenario("slow-api")
# ... run slow-path tests ...
server.deactivate_scenario("slow-api")
```

### Faults

```python
from mockly_driver import FaultConfig

server.set_fault(FaultConfig(enabled=True, status_override=503, error_rate=0.5))
# ... run fault-tolerance tests ...
server.clear_fault()
```

---

## API Reference

### `MocklyServer.ensure(scenarios=None, **install_kwargs) -> MocklyServer`

Downloads the binary (if not already present) and starts the server.
`install_kwargs` are forwarded to `install()` — see below.

### `MocklyServer.create(scenarios=None) -> MocklyServer`

Starts the server using an already-installed binary.
Raises `RuntimeError` if no binary is found.

### `server.stop() -> None`

Kills the Mockly process and removes the temp config file.

### `server.add_mock(mock: Mock) -> None`

Registers a new HTTP mock at runtime.

```python
Mock(
    id="my-mock",
    request=MockRequest(method="POST", path="/echo",
                        headers={"X-Api-Key": "secret"}),
    response=MockResponse(status=201, body='{"ok":true}',
                          headers={"Content-Type": "application/json"},
                          delay="10ms"),
)
```

### `server.delete_mock(mock_id: str) -> None`

Removes a previously registered mock by ID.

### `server.reset() -> None`

Removes all dynamic mocks, deactivates all scenarios, and clears any active fault.

### `server.activate_scenario(scenario_id: str) -> None`

Activates a pre-configured scenario.

### `server.deactivate_scenario(scenario_id: str) -> None`

Deactivates a scenario.

### `server.set_fault(config: FaultConfig) -> None`

Enables a fault injection policy.

```python
FaultConfig(
    enabled=True,
    delay="200ms",        # add latency to all responses
    status_override=503,  # force every response to 503
    error_rate=0.3,       # randomly fail 30% of requests
)
```

### `server.clear_fault() -> None`

Removes the active fault policy.

### `install(version=None, base_url=None, bin_dir=None, force=False) -> str`

Downloads the Mockly binary for the current platform. Returns the path.

### `get_binary_path(bin_dir=None) -> str | None`

Returns the path to an existing binary, or `None` if not found.

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `MOCKLY_BINARY_PATH` | Absolute path to a pre-staged binary — skips all download logic | — |
| `MOCKLY_VERSION` | Binary version to download | `v0.1.0` |
| `MOCKLY_DOWNLOAD_BASE_URL` | Override the GitHub releases base URL (for mirrors/Artifactory) | `https://github.com/dever-labs/mockly/releases/download` |
| `MOCKLY_NO_INSTALL` | If set, `install()` raises `RuntimeError` instead of downloading | — |
| `HTTPS_PROXY` / `HTTP_PROXY` | Route downloads through an HTTP proxy | — |

---

## Proxy / Artifactory / Air-gap

### HTTP/HTTPS proxy

```bash
export HTTPS_PROXY=http://proxy.corp.example.com:3128
mockly-install
```

### Internal mirror / Artifactory

Set `MOCKLY_DOWNLOAD_BASE_URL` to your internal artifact repository:

```bash
export MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.corp.example.com/mockly/releases/download
mockly-install
```

### Air-gap / pre-staged binary

Copy the binary for your platform to a known location and set:

```bash
export MOCKLY_BINARY_PATH=/opt/mockly/mockly
export MOCKLY_NO_INSTALL=1   # optional: prevents accidental downloads
```

---

## CLI

```
mockly-install [--version VERSION] [--base-url URL] [--bin-dir DIR] [--force]
```

Downloads the platform binary to `<bin-dir>/mockly[.exe]` (default: `./bin/`).

---

## License

MIT © dever-labs
