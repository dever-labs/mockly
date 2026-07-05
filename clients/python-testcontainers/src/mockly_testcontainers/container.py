import json
import os
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

from testcontainers.core.container import DockerContainer

from ._types import (
    ActiveScenariosResponse,
    CallEntry,
    CallSummary,
    FaultConfig,
    Mock,
    MocklyServerOptions,
    MockRequest,
    MockResponse,
    MockResponsePatch,
    Scenario,
    ScenarioPatch,
)

DEFAULT_IMAGE = "ghcr.io/dever-labs/mockly:latest"
HTTP_PORT = 8090
API_PORT = 9091
CONTAINER_CONFIG_PATH = "/config/mockly.yaml"
DEFAULT_CONFIG = """mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
"""


class MocklyContainer(DockerContainer):
    def __init__(self, image: str = DEFAULT_IMAGE) -> None:
        super().__init__(image)
        self._config_yaml = DEFAULT_CONFIG
        self._config_host_path: str | None = None
        self.with_exposed_ports(HTTP_PORT, API_PORT)

    def with_inline_config(self, yaml: str) -> "MocklyContainer":
        self._config_yaml = yaml
        return self

    def with_options(self, options: MocklyServerOptions) -> "MocklyContainer":
        self._config_yaml = _render_config(HTTP_PORT, API_PORT, options.scenarios)
        return self

    def _configure(self) -> None:
        self._cleanup_config_file()
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            prefix=".mockly-tc-",
            suffix=".yaml",
            dir=os.getcwd(),
            delete=False,
        ) as config_file:
            config_file.write(self._config_yaml)
            self._config_host_path = config_file.name
        self.with_command(f"start -c {CONTAINER_CONFIG_PATH}")
        self.with_volume_mapping(self._config_host_path, CONTAINER_CONFIG_PATH, "ro")

    def _cleanup_config_file(self) -> None:
        if self._config_host_path is None:
            return
        self.volumes.pop(self._config_host_path, None)
        try:
            os.unlink(self._config_host_path)
        except FileNotFoundError:
            pass
        self._config_host_path = None

    def get_http_base(self) -> str:
        host = self.get_container_host_ip()
        port = self.get_exposed_port(HTTP_PORT)
        return f"http://{host}:{port}"

    def get_api_base(self) -> str:
        host = self.get_container_host_ip()
        port = self.get_exposed_port(API_PORT)
        return f"http://{host}:{port}"

    def _wait_ready(self, max_ms: int = 60000) -> None:
        url = f"{self.get_api_base()}/api/protocols"
        deadline = time.monotonic() + max_ms / 1000
        while time.monotonic() < deadline:
            try:
                with urllib.request.urlopen(url, timeout=2):
                    return
            except Exception:
                time.sleep(0.1)
        raise TimeoutError(f"Mockly container did not become ready within {max_ms}ms")

    def start(self) -> "MocklyContainer":
        try:
            self._configure()
            super().start()
            self._wait_ready()
            return self
        except Exception:
            self.stop()
            raise

    def stop(self, force: bool = True, delete_volume: bool = True) -> None:
        try:
            super().stop(force=force, delete_volume=delete_volume)
        finally:
            self._cleanup_config_file()

    def add_mock(self, mock: Mock) -> None:
        self._request("POST", "/api/mocks/http", _mock_to_dict(mock), expected=(200, 201))

    def list_mocks(self) -> list[Mock]:
        return [_parse_mock(item) for item in self._get_json("GET", "/api/mocks/http", expected=(200,))]

    def update_mock(self, mock_id: str, mock: Mock) -> Mock:
        path = f"/api/mocks/http/{urllib.parse.quote(mock_id, safe='')}"
        return _parse_mock(self._get_json("PUT", path, _mock_to_dict(mock), expected=(200,)))

    def patch_mock(self, mock_id: str, patch: MockResponsePatch) -> Mock:
        path = f"/api/mocks/http/{urllib.parse.quote(mock_id, safe='')}"
        return _parse_mock(
            self._get_json(
                "PATCH",
                path,
                _mock_response_patch_to_dict(patch),
                expected=(200,),
            )
        )

    def delete_mock(self, mock_id: str) -> None:
        path = f"/api/mocks/http/{urllib.parse.quote(mock_id, safe='')}"
        self._request("DELETE", path, expected=(200,))

    def get_state(self) -> dict[str, str]:
        return self._get_json("GET", "/api/state", expected=(200,))

    def set_state(self, kv_map: dict[str, str]) -> dict[str, str]:
        return self._get_json("POST", "/api/state", kv_map, expected=(200,))

    def delete_state(self, key: str) -> None:
        path = f"/api/state/{urllib.parse.quote(key, safe='')}"
        self._request("DELETE", path, expected=(200,))

    def reset(self) -> None:
        self._request("POST", "/api/reset", expected=(200,))

    def activate_scenario(self, scenario_id: str) -> None:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}/activate"
        self._request("POST", path, expected=(200,))

    def deactivate_scenario(self, scenario_id: str) -> None:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}/deactivate"
        self._request("POST", path, expected=(200,))

    def list_scenarios(self) -> list[Scenario]:
        return [_parse_scenario(item) for item in self._get_json("GET", "/api/scenarios", expected=(200,))]

    def create_scenario(self, scenario: Scenario) -> Scenario:
        return _parse_scenario(
            self._get_json("POST", "/api/scenarios", _scenario_to_dict(scenario), expected=(201,))
        )

    def get_scenario(self, scenario_id: str) -> Scenario:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}"
        return _parse_scenario(self._get_json("GET", path, expected=(200,)))

    def update_scenario(self, scenario_id: str, scenario: Scenario) -> Scenario:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}"
        return _parse_scenario(
            self._get_json("PUT", path, _scenario_to_dict(scenario), expected=(200,))
        )

    def delete_scenario(self, scenario_id: str) -> None:
        path = f"/api/scenarios/{urllib.parse.quote(scenario_id, safe='')}"
        self._request("DELETE", path, expected=(200,))

    def list_active_scenarios(self) -> ActiveScenariosResponse:
        data = self._get_json("GET", "/api/scenarios/active", expected=(200,))
        return ActiveScenariosResponse(
            active=list(data.get("active") or []),
            scenarios=[_parse_scenario(item) for item in (data.get("scenarios") or [])],
        )

    def set_fault(self, config: FaultConfig) -> None:
        self._request("POST", "/api/fault/http", _fault_config_to_dict(config), expected=(200,))

    def clear_fault(self) -> None:
        self._request("DELETE", "/api/fault", expected=(200,))

    def get_logs(self, matched_id: str | None = None) -> list[CallEntry]:
        path = _with_optional_matched_id("/api/logs", matched_id)
        return [_parse_call_entry(item) for item in self._get_json("GET", path, expected=(200,))]

    def get_logs_count(self, matched_id: str | None = None) -> int:
        path = _with_optional_matched_id("/api/logs/count", matched_id)
        data = self._get_json("GET", path, expected=(200,))
        return int(data.get("count", 0))

    def clear_logs(self) -> None:
        self._request("DELETE", "/api/logs", expected=(200,))

    def get_calls(self, mock_id: str) -> CallSummary:
        path = f"/api/calls/http/{urllib.parse.quote(mock_id, safe='')}"
        return _parse_call_summary(self._get_json("GET", path, expected=(200,)))

    def clear_calls(self, mock_id: str) -> None:
        path = f"/api/calls/http/{urllib.parse.quote(mock_id, safe='')}"
        self._request("DELETE", path, expected=(200,))

    def clear_all_calls(self) -> None:
        self._request("DELETE", "/api/calls/http", expected=(200,))

    def wait_for_calls(
        self,
        mock_id: str,
        count: int = 1,
        timeout_seconds: int = 10,
    ) -> CallSummary:
        path = f"/api/calls/http/{urllib.parse.quote(mock_id, safe='')}/wait"
        body = {"count": count, "timeout": f"{timeout_seconds}s"}
        return _parse_call_summary(self._get_json("POST", path, body, expected=(200,)))

    def _get_json(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None = None,
        expected: tuple[int, ...] = (200,),
    ) -> dict[str, Any] | list[Any]:
        if body is None:
            raw = self._request(method, path, expected=expected)
        else:
            raw = self._request(method, path, body, expected=expected)
        return json.loads(raw.decode() if raw else "null")

    def _request(
        self,
        method: str,
        path: str,
        body: dict[str, Any] | None = None,
        expected: tuple[int, ...] = (200,),
    ) -> bytes:
        url = f"{self.get_api_base()}{path}"
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


def _render_config(http_port: int, api_port: int, scenarios: list[Scenario] | None) -> str:
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

    return "\n".join(lines) + "\n"


def _yaml_str(value: str) -> str:
    escaped = value.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")
    return f'"{escaped}"'


def _with_optional_matched_id(path: str, matched_id: str | None) -> str:
    if not matched_id:
        return path
    return f"{path}?matched_id={urllib.parse.quote(matched_id, safe='')}"


def _mock_to_dict(mock: Mock) -> dict[str, Any]:
    body: dict[str, Any] = {
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


def _mock_response_to_dict(response: MockResponse) -> dict[str, Any]:
    body: dict[str, Any] = {"status": response.status}
    if response.body != "":
        body["body"] = response.body
    if response.headers:
        body["headers"] = response.headers
    if response.delay is not None:
        body["delay"] = response.delay
    return body


def _mock_response_patch_to_dict(patch: MockResponsePatch) -> dict[str, Any]:
    body: dict[str, Any] = {}
    if patch.status is not None:
        body["status"] = patch.status
    if patch.body is not None:
        body["body"] = patch.body
    if patch.headers is not None:
        body["headers"] = patch.headers
    if patch.delay is not None:
        body["delay"] = patch.delay
    return body


def _scenario_to_dict(scenario: Scenario) -> dict[str, Any]:
    body: dict[str, Any] = {
        "id": scenario.id,
        "name": scenario.name,
        "patches": [_scenario_patch_to_dict(patch) for patch in scenario.patches],
    }
    if scenario.description is not None:
        body["description"] = scenario.description
    return body


def _scenario_patch_to_dict(patch: ScenarioPatch) -> dict[str, Any]:
    body: dict[str, Any] = {"mock_id": patch.mock_id}
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


def _fault_config_to_dict(config: FaultConfig) -> dict[str, Any]:
    body: dict[str, Any] = {"enabled": config.enabled}
    if config.delay is not None:
        body["delay"] = config.delay
    if config.status is not None:
        body["status"] = config.status
    if config.error_rate is not None:
        body["error_rate"] = config.error_rate
    return body


def _parse_mock(data: dict[str, Any]) -> Mock:
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


def _parse_scenario(data: dict[str, Any]) -> Scenario:
    return Scenario(
        id=data.get("id", ""),
        name=data.get("name", ""),
        description=data.get("description"),
        patches=[_parse_scenario_patch(item) for item in (data.get("patches") or [])],
    )


def _parse_scenario_patch(data: dict[str, Any]) -> ScenarioPatch:
    return ScenarioPatch(
        mock_id=data.get("mock_id", ""),
        status=data.get("status"),
        body=data.get("body"),
        headers=data.get("headers"),
        delay=data.get("delay"),
        disabled=data.get("disabled"),
    )


def _parse_call_summary(data: dict[str, Any]) -> CallSummary:
    return CallSummary(
        mock_id=data.get("mock_id", ""),
        count=data.get("count", 0),
        calls=[_parse_call_entry(item) for item in (data.get("calls") or [])],
    )


def _parse_call_entry(data: dict[str, Any]) -> CallEntry:
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
