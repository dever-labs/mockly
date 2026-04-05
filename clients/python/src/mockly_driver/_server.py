"""MocklyServer — start, control, and stop a Mockly process from Python tests."""

import json
import os
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from typing import Optional

from ._install import get_binary_path, install
from ._types import FaultConfig, Mock, Scenario


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
            if scenario.patches:
                lines.append("    patches:")
                for patch in scenario.patches:
                    lines.append(f"      - mock_id: {_yaml_str(patch.mock_id)}")
                    if patch.status is not None:
                        lines.append(f"        status: {patch.status}")
                    if patch.body is not None:
                        lines.append(f"        body: {_yaml_str(patch.body)}")
                    if patch.delay is not None:
                        lines.append(f"        delay: {_yaml_str(patch.delay)}")

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

    # ------------------------------------------------------------------
    # Factory methods
    # ------------------------------------------------------------------

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
                    [binary, "start", "--config", config_path,
                     "--api-port", str(api_port)],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                )
                server = cls(process, http_port, api_port, config_path)
                try:
                    server._wait_ready()
                except TimeoutError as exc:
                    # Check whether the process died with a port conflict
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

        raise RuntimeError(
            f"Failed to start mockly after {attempts} attempts"
        ) from last_error

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------

    def stop(self) -> None:
        """Kill the Mockly process and clean up the config file."""
        if self._process.poll() is None:
            self._process.kill()
            self._process.wait()
        try:
            os.unlink(self._config_path)
        except OSError:
            pass

    # ------------------------------------------------------------------
    # Management API helpers
    # ------------------------------------------------------------------

    def add_mock(self, mock: Mock) -> None:
        body: dict = {
            "id": mock.id,
            "request": {
                "method": mock.request.method,
                "path": mock.request.path,
            },
            "response": {
                "status": mock.response.status,
                "body": mock.response.body,
            },
        }
        if mock.request.headers:
            body["request"]["headers"] = mock.request.headers
        if mock.response.headers:
            body["response"]["headers"] = mock.response.headers
        if mock.response.delay is not None:
            body["response"]["delay"] = mock.response.delay

        self._request("POST", "/api/mocks/http", body, expected=(200, 201))

    def delete_mock(self, mock_id: str) -> None:
        self._request("DELETE", f"/api/mocks/http/{mock_id}", expected=(204,))

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
        if config.status_override is not None:
            body["status_override"] = config.status_override
        if config.error_rate is not None:
            body["error_rate"] = config.error_rate
        self._request("POST", "/api/fault", body, expected=(200,))

    def clear_fault(self) -> None:
        self._request("DELETE", "/api/fault", expected=(200,))

    # ------------------------------------------------------------------
    # Private helpers
    # ------------------------------------------------------------------

    def _wait_ready(self, max_ms: int = 10000) -> None:
        url = f"{self.api_base}/api/protocols"
        deadline = time.monotonic() + max_ms / 1000
        while time.monotonic() < deadline:
            try:
                with urllib.request.urlopen(url, timeout=1):
                    return
            except Exception:
                time.sleep(0.05)
        raise TimeoutError(
            f"Mockly did not become ready within {max_ms}ms (api={self.api_base})"
        )

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
