import json
import os
import tempfile
import time
import urllib.error
import urllib.request
from dataclasses import asdict
from typing import Any

from testcontainers.core.container import DockerContainer

from ._types import FaultConfig, Mock

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


def _without_none(value: Any) -> Any:
    if isinstance(value, dict):
        return {
            key: _without_none(item)
            for key, item in value.items()
            if item is not None
        }
    if isinstance(value, list):
        return [_without_none(item) for item in value]
    return value


class MocklyContainer(DockerContainer):
    def __init__(self, image: str = DEFAULT_IMAGE) -> None:
        super().__init__(image)
        self._config_yaml = DEFAULT_CONFIG
        self._config_host_path: str | None = None
        self.with_exposed_ports(HTTP_PORT, API_PORT)

    def with_inline_config(self, yaml: str) -> "MocklyContainer":
        self._config_yaml = yaml
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
        self._request(
            "POST",
            "/api/mocks/http",
            _without_none(asdict(mock)),
            expected=(200, 201),
        )

    def delete_mock(self, mock_id: str) -> None:
        self._request("DELETE", f"/api/mocks/http/{mock_id}", expected=(204,))

    def reset(self) -> None:
        self._request("POST", "/api/reset", expected=(200,))

    def activate_scenario(self, scenario_id: str) -> None:
        self._request("POST", f"/api/scenarios/{scenario_id}/activate", expected=(200,))

    def deactivate_scenario(self, scenario_id: str) -> None:
        self._request("POST", f"/api/scenarios/{scenario_id}/deactivate", expected=(200,))

    def set_fault(self, config: FaultConfig) -> None:
        self._request(
            "POST",
            "/api/fault",
            _without_none(asdict(config)),
            expected=(200,),
        )

    def clear_fault(self) -> None:
        self._request("DELETE", "/api/fault", expected=(200,))

    def get_logs(self) -> str:
        return self._request("GET", "/api/logs", expected=(200,)).decode()

    def clear_logs(self) -> None:
        self._request("DELETE", "/api/logs", expected=(200,))

    def _request(
        self,
        method: str,
        path: str,
        body: dict | None = None,
        expected: tuple[int, ...] = (200,),
    ) -> bytes:
        url = f"{self.get_api_base()}{path}"
        data = json.dumps(body).encode() if body is not None else None
        headers = {"Content-Type": "application/json"} if data is not None else {}
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req) as resp:
                if resp.status not in expected:
                    raise RuntimeError(
                        f"Unexpected status {resp.status} for {method} {path}"
                    )
                return resp.read()
        except urllib.error.HTTPError as exc:
            if exc.code in expected:
                return exc.read()
            raise RuntimeError(
                f"HTTP {exc.code} for {method} {path}: {exc.read().decode(errors='replace')}"
            ) from exc
