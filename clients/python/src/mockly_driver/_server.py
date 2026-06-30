"""MocklyServer — start, control, and stop a Mockly process from Python tests."""

import json
import os
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from ._install import get_binary_path, install
from ._types import (
    ActiveScenariosResponse,
    CallEntry,
    CallSummary,
    FaultConfig,
    Mock,
    MockResponse,
    MockResponsePatch,
    MockRequest,
    Scenario,
    ScenarioPatch,
)


def _get_free_port() -> int:
    """Bind to port 0 to let the OS pick a free port, then return it."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def _write_config(http_port: int, api_port: int, scenarios: list[Scenario] | None) -> str:
    """Write a Mockly YAML config to a temp file and return its path."""
    lines = [
        "mockly:",
        "  api:",
        f"    port: {api_port}",
        "protocols:",
        "  http:",
        "    enabled: true",
        f"    port: {http_port}",
    ]

    if scenarios:
        lines.append("scenarios:")
        for scenario in scenarios:
            lines.append(f"  - id: {_yaml_str(scenario.id)}")
            lines.append(f"    name: {_yaml_str(scenario.name)}")
            if scenario.description is not None:
                lines.append(f"    description: {_yaml_str(scenario.description)}")
            if scenario.patches:
                lines.append("    patches:")
                for patch in scenario.patches:
                    lines.append(f"      - mock_id: {_yaml_str(patch.mock_id)}")
                    if patch.status is not None:
                        lines.append(f"        status: {patch.status}")
                    if patch.body is not None:
                        lines.append(f"        body: {_yaml_str(patch.body)}")
                    if patch.headers:
                        lines.append("        headers:")
                        for key, value in patch.headers.items():
                            lines.append(f"          {_yaml_str(key)}: {_yaml_str(value)}")
                    if patch.delay is not None:
                        lines.append(f"        delay: {_yaml_str(patch.delay)}")
                    if patch.disabled is not None:
                        lines.append(f"        disabled: {'true' if patch.disabled else 'false'}")

    content = "\n".join(lines) + "\n"

    fd, path = tempfile.mkstemp(suffix=".yaml", prefix="mockly-")
    try:
        with os.fdopen(fd, "w") as fh:
            fh.write(content)
    except Exception:
        os.unlink(path)
        raise
    return path


def _yaml_str(value: str) -> str:
    """Wrap a string in double-quotes with minimal escaping for YAML."""
    escaped = value.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")
    return f'"{escaped}"'


def _is_port_conflict(error_msg: str) -> bool:
    msg = error_msg.lower()
    return any(token in msg for token in ("address already in use", "eaddrinuse", "bind:"))


class MocklyServer:
    """A running Mockly process with helpers for managing HTTP mocks."""

    http_port: int
    api_port: int
    http_base: str
    api_base: str

    def __init__(
        self,
        process: subprocess.Popen,
        http_port: int,
        api_port: int,
        config_path: str,
    ) -> None:
        self._process = process
        self._config_path = config_path
        self.http_port = http_port
        self.api_port = api_port
        self.http_base = f"http://127.0.0.1:{http_port}"
        self.api_base = f"http://127.0.0.1:{api_port}"

    @classmethod
    def create(
        cls,
        scenarios: list[Scenario] | None = None,
    ) -> "MocklyServer":
        """Start Mockly using an already-installed binary.

        Raises RuntimeError if the binary cannot be found.
        Retries up to 3 times on port conflicts.
        """
        binary = get_binary_path()
        if not binary:
            raise RuntimeError(
                "mockly binary not found. Run install() or set MOCKLY_BINARY_PATH."
            )
        return cls._start(binary, scenarios)

    @classmethod
    def ensure(
        cls,
        scenarios: list[Scenario] | None = None,
        **install_kwargs,
    ) -> "MocklyServer":
        """Install the binary if necessary, then start Mockly."""
        install(**install_kwargs)
        return cls.create(scenarios=scenarios)

    @classmethod
    def _start(
        cls,
        binary: str,
        scenarios: list[Scenario] | None,
        attempts: int = 3,
    ) -> "MocklyServer":
        last_error: Exception | None = None
        for _ in range(attempts):
            http_port = _get_free_port()
            api_port = _get_free_port()
            config_path = _write_config(http_port, api_port, scenarios)
            try:
                process = subprocess.Popen(
                    [binary, "start", "--config", config_path, "--api-port", str(api_port)],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                )
                server = cls(process, http_port, api_port, config_path)
                try:
                    server._wait_ready()
                except TimeoutError as exc:
                    process.poll()
                    if process.returncode is not None:
                        stderr_text = ""
                        if process.stderr:
                            stderr_text = process.stderr.read().decode(errors="replace")
                        if _is_port_conflict(stderr_text):
                            last_error = exc
                            try:
                                os.unlink(config_path)
                            except OSError:
                                pass
                            continue
                    raise
                return server
            except Exception as exc:
                last_error = exc
                try:
                    os.unlink(config_path)
                except OSError:
                    pass
                continue

        raise RuntimeError(f"Failed to start mockly after {attempts} attempts") from last_error

    def stop(self) -> None:
        """Kill the Mockly process and clean up the config file."""
        if self._process.poll() is None:
            self._process.kill()
            self._process.wait()
        try:
            os.unlink(self._config_path)
        except OSError:
            pass

    def add_mock(self, mock: Mock) -> None:
        self._request("POST", "/api/mocks/http", _mock_to_dict(mock), expected=(200, 201))

    def list_mocks(self) -> list[Mock]:
        data = json.loads(self._request("GET", "/api/mocks/http", expected=(200,)))
        return [_parse_mock(item) for item in data]

    def update_mock(self, mock_id: str, mock: Mock) -> Mock:
        path = f"/api/mocks/http/{urllib.parse.quote(mock_id, safe='')}"
        data = json.loads(self._request("PUT", path, _mock_to_dict(mock), expected=(200,)))
        return _parse_mock(data)

    def patch_mock(self, mock_id: str, patch: MockResponsePatch) -> Mock:
        path = f"/api/mocks/http/{urllib.parse.quote(mock_id, safe='')}"
        data = json.loads(self._request("PATCH", path, _mock_response_patch_to_dict(patch), expected=(200,)))
        return _parse_mock(data)

    def delete_mock(self, mock_id: str) -> None:
        self._request("DELETE", f"/api/mocks/http/{urllib.parse.quote(mock_id, safe='')}", expected=(200,))

    def get_state(self) -> dict[str, str]:
        return json.loads(self._request("GET", "/api/state", expected=(200,)))

    def set_state(self, kv_map: dict[str, str]) -> dict[str, str]:
        return json.loads(self._request("POST", "/api/state", kv_map, expected=(200,)))

    def delete_state(self, key: str) -> None:
        path = f"/api/state/{urllib.parse.quote(key, safe='')}"
        self._request("DELETE", path, expected=(200,))

    def get_logs(self, matched_id: str | None = None) -> list[CallEntry]:
        path = _with_optional_matched_id("/api/logs", matched_id)
        data = json.loads(self._request("GET", path, expected=(200,)))
        return [_parse_call_entry(item) for item in data]

    def clear_logs(self) -> None:
        self._request("DELETE", "/api/logs", expected=(200,))

    def get_logs_count(self, matched_id: str | None = None) -> int:
        path = _with_optional_matched_id("/api/logs/count", matched_id)
        data = json.loads(self._request("GET", path, expected=(200,)))
        return data.get("count", 0)

    def list_scenarios(self) -> list[Scenario]:
        data = json.loads(self._request("GET", "/api/scenarios", expected=(200,)))
        return [_parse_scenario(item) for item in data]

    def create_scenario(self, scenario: Scenario) -> Scenario:
        data = json.loads(self._request("POST", "/api/scenarios", _scenario_to_dict(scenario), expected=(201,)))
        return _parse_scenario(data)

    def get_scenario(self, scenario_id: str) -> Scenario:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}"
        data = json.loads(self._request("GET", path, expected=(200,)))
        return _parse_scenario(data)

    def update_scenario(self, scenario_id: str, scenario: Scenario) -> Scenario:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}"
        data = json.loads(self._request("PUT", path, _scenario_to_dict(scenario), expected=(200,)))
        return _parse_scenario(data)

    def delete_scenario(self, scenario_id: str) -> None:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}"
        self._request("DELETE", path, expected=(200,))

    def list_active_scenarios(self) -> ActiveScenariosResponse:
        data = json.loads(self._request("GET", "/api/scenarios/active", expected=(200,)))
        return ActiveScenariosResponse(
            active=list(data.get("active") or []),
            scenarios=[_parse_scenario(item) for item in (data.get("scenarios") or [])],
        )

    def reset(self) -> None:
        self._request("POST", "/api/reset", expected=(200,))

    def activate_scenario(self, scenario_id: str) -> None:
        self._request("POST", f"/api/scenarios/{scenario_id}/activate", expected=(200,))

    def deactivate_scenario(self, scenario_id: str) -> None:
        self._request("POST", f"/api/scenarios/{scenario_id}/deactivate", expected=(200,))

    def set_fault(self, config: FaultConfig) -> None:
        body: dict = {"enabled": config.enabled}
        if config.delay is not None:
            body["delay"] = config.delay
        if config.status is not None:
            body["status"] = config.status
        if config.error_rate is not None:
            body["error_rate"] = config.error_rate
        self._request("POST", "/api/fault/http", body, expected=(200,))

    def clear_fault(self) -> None:
        self._request("DELETE", "/api/fault", expected=(200, 204))

    def get_calls(self, mock_id: str) -> CallSummary:
        data = json.loads(self._request("GET", f"/api/calls/http/{urllib.parse.quote(mock_id, safe='')}", expected=(200,)))
        return _parse_call_summary(data)

    def clear_calls(self, mock_id: str) -> None:
        self._request("DELETE", f"/api/calls/http/{urllib.parse.quote(mock_id, safe='')}", expected=(200,))

    def clear_all_calls(self) -> None:
        self._request("DELETE", "/api/calls/http", expected=(200,))

    def wait_for_calls(self, mock_id: str, count: int = 1, timeout_seconds: int = 10) -> CallSummary:
        body = {"count": count, "timeout": f"{timeout_seconds}s"}
        try:
            raw = self._request(
                "POST",
                f"/api/calls/http/{urllib.parse.quote(mock_id, safe='')}/wait",
                body,
                expected=(200,),
            )
        except RuntimeError as exc:
            if "408" in str(exc):
                raise RuntimeError(
                    f"wait_for_calls: timeout waiting for {count} call(s) on '{mock_id}'"
                ) from exc
            raise
        return _parse_call_summary(json.loads(raw))

    def _wait_ready(self, max_ms: int = 10000) -> None:
        url = f"{self.api_base}/api/protocols"
        deadline = time.monotonic() + max_ms / 1000
        while time.monotonic() < deadline:
            try:
                with urllib.request.urlopen(url, timeout=1):
                    return
            except Exception:
                time.sleep(0.05)
        raise TimeoutError(f"Mockly did not become ready within {max_ms}ms (api={self.api_base})")

    def _request(
        self,
        method: str,
        path: str,
        body: dict | None = None,
        expected: tuple[int, ...] = (200,),
    ) -> bytes:
        url = f"{self.api_base}{path}"
        data = json.dumps(body).encode() if body is not None else None
        headers = {"Content-Type": "application/json"} if data is not None else {}
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req) as resp:
                if resp.status not in expected:
                    raise RuntimeError(f"Unexpected status {resp.status} for {method} {path}")
                return resp.read()
        except urllib.error.HTTPError as exc:
            if exc.code in expected:
                return exc.read()
            raise RuntimeError(
                f"HTTP {exc.code} for {method} {path}: {exc.read().decode(errors='replace')}"
            ) from exc


def _with_optional_matched_id(path: str, matched_id: str | None) -> str:
    if not matched_id:
        return path
    return f"{path}?matched_id={urllib.parse.quote(matched_id, safe='')}"


def _mock_to_dict(mock: Mock) -> dict:
    body: dict = {
        "id": mock.id,
        "request": {
            "method": mock.request.method,
            "path": mock.request.path,
        },
        "response": _mock_response_to_dict(mock.response),
    }
    if mock.request.headers:
        body["request"]["headers"] = mock.request.headers
    return body


def _mock_response_to_dict(response: MockResponse) -> dict:
    body: dict = {"status": response.status}
    if response.body != "":
        body["body"] = response.body
    if response.headers:
        body["headers"] = response.headers
    if response.delay is not None:
        body["delay"] = response.delay
    return body


def _mock_response_patch_to_dict(patch: MockResponsePatch) -> dict:
    body: dict = {}
    if patch.status is not None:
        body["status"] = patch.status
    if patch.body is not None:
        body["body"] = patch.body
    if patch.headers is not None:
        body["headers"] = patch.headers
    if patch.delay is not None:
        body["delay"] = patch.delay
    return body


def _scenario_to_dict(scenario: Scenario) -> dict:
    body: dict = {
        "id": scenario.id,
        "name": scenario.name,
        "patches": [_scenario_patch_to_dict(patch) for patch in scenario.patches],
    }
    if scenario.description is not None:
        body["description"] = scenario.description
    return body


def _scenario_patch_to_dict(patch: ScenarioPatch) -> dict:
    body: dict = {"mock_id": patch.mock_id}
    if patch.status is not None:
        body["status"] = patch.status
    if patch.body is not None:
        body["body"] = patch.body
    if patch.headers is not None:
        body["headers"] = patch.headers
    if patch.delay is not None:
        body["delay"] = patch.delay
    if patch.disabled is not None:
        body["disabled"] = patch.disabled
    return body


def _parse_mock(data: dict) -> Mock:
    request = data.get("request") or {}
    response = data.get("response") or {}
    return Mock(
        id=data.get("id", ""),
        request=MockRequest(
            method=request.get("method", ""),
            path=request.get("path", ""),
            headers=request.get("headers") or {},
        ),
        response=MockResponse(
            status=response.get("status", 0),
            body=response.get("body") or "",
            headers=response.get("headers") or {},
            delay=response.get("delay"),
        ),
    )


def _parse_scenario(data: dict) -> Scenario:
    return Scenario(
        id=data.get("id", ""),
        name=data.get("name", ""),
        description=data.get("description"),
        patches=[_parse_scenario_patch(item) for item in (data.get("patches") or [])],
    )


def _parse_scenario_patch(data: dict) -> ScenarioPatch:
    return ScenarioPatch(
        mock_id=data.get("mock_id", ""),
        status=data.get("status"),
        body=data.get("body"),
        headers=data.get("headers"),
        delay=data.get("delay"),
        disabled=data.get("disabled"),
    )


def _parse_call_summary(data: dict) -> CallSummary:
    calls = [_parse_call_entry(item) for item in (data.get("calls") or [])]
    return CallSummary(
        mock_id=data.get("mock_id", ""),
        count=data.get("count", 0),
        calls=calls,
    )


def _parse_call_entry(data: dict) -> CallEntry:
    return CallEntry(
        id=data.get("id", ""),
        timestamp=data.get("timestamp", ""),
        protocol=data.get("protocol", ""),
        path=data.get("path", ""),
        duration_ms=data.get("duration_ms", 0),
        method=data.get("method"),
        status=data.get("status"),
        headers=data.get("headers") or {},
        body=data.get("body"),
        matched_id=data.get("matched_id"),
        path_params=data.get("path_params") or {},
    )
