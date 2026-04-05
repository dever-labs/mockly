"""Unit tests for mockly-driver (no real binary required)."""

import os

import pytest

from mockly_driver._install import get_binary_path, install
from mockly_driver._server import _get_free_port


# ---------------------------------------------------------------------------
# _get_free_port
# ---------------------------------------------------------------------------

def test_get_free_port_returns_valid_port():
    port = _get_free_port()
    assert 1025 <= port <= 65535


# ---------------------------------------------------------------------------
# get_binary_path
# ---------------------------------------------------------------------------

def test_get_binary_path_returns_none_when_missing(tmp_path, monkeypatch):
    monkeypatch.delenv("MOCKLY_BINARY_PATH", raising=False)
    result = get_binary_path(bin_dir=str(tmp_path / "nonexistent"))
    assert result is None


def test_get_binary_path_respects_env_var(tmp_path, monkeypatch):
    fake_binary = tmp_path / "mockly"
    fake_binary.write_bytes(b"fake")
    monkeypatch.setenv("MOCKLY_BINARY_PATH", str(fake_binary))
    result = get_binary_path()
    assert result == str(fake_binary)


def test_get_binary_path_ignores_missing_env_var(tmp_path, monkeypatch):
    monkeypatch.setenv("MOCKLY_BINARY_PATH", str(tmp_path / "does_not_exist"))
    result = get_binary_path()
    assert result is None


# ---------------------------------------------------------------------------
# install
# ---------------------------------------------------------------------------

def test_install_raises_with_mockly_no_install(monkeypatch):
    monkeypatch.setenv("MOCKLY_NO_INSTALL", "1")
    monkeypatch.delenv("MOCKLY_BINARY_PATH", raising=False)
    with pytest.raises(RuntimeError, match="MOCKLY_NO_INSTALL"):
        install()


def test_install_returns_existing_binary_via_env(tmp_path, monkeypatch):
    fake_binary = tmp_path / "mockly"
    fake_binary.write_bytes(b"fake")
    monkeypatch.setenv("MOCKLY_BINARY_PATH", str(fake_binary))
    monkeypatch.delenv("MOCKLY_NO_INSTALL", raising=False)
    result = install()
    assert result == str(fake_binary)


# ---------------------------------------------------------------------------
# _is_port_conflict
# ---------------------------------------------------------------------------

from mockly_driver._server import _is_port_conflict


def test_is_port_conflict_address_already_in_use():
    assert _is_port_conflict("address already in use") is True


def test_is_port_conflict_eaddrinuse():
    assert _is_port_conflict("EADDRINUSE") is True


def test_is_port_conflict_eaddrinuse_with_port():
    assert _is_port_conflict("EADDRINUSE :::8080") is True


def test_is_port_conflict_bind_prefix():
    assert _is_port_conflict("bind: address already in use") is True


def test_is_port_conflict_connection_refused():
    assert _is_port_conflict("connection refused") is False


def test_is_port_conflict_timeout():
    assert _is_port_conflict("timeout") is False


def test_is_port_conflict_empty():
    assert _is_port_conflict("") is False


# ---------------------------------------------------------------------------
# _yaml_str
# ---------------------------------------------------------------------------

from mockly_driver._server import _yaml_str


def test_yaml_str_plain():
    assert _yaml_str("hello") == '"hello"'


def test_yaml_str_single_quote():
    # single quotes need no special escaping in double-quoted YAML
    result = _yaml_str("it's")
    assert "it's" in result
    assert result.startswith('"') and result.endswith('"')


def test_yaml_str_backslash():
    result = _yaml_str("a\\b")
    # backslash must be escaped as \\
    assert "\\\\" in result


def test_yaml_str_empty():
    assert _yaml_str("") == '""'


def test_yaml_str_newline():
    result = _yaml_str("line1\nline2")
    assert "\\n" in result
    assert "\n" not in result[1:-1]  # no literal newline inside the quoted value


# ---------------------------------------------------------------------------
# _write_config
# ---------------------------------------------------------------------------

from mockly_driver._server import _write_config


def test_write_config_returns_existing_path():
    path = _write_config(8080, 9090, None)
    try:
        assert os.path.isfile(path)
    finally:
        os.unlink(path)


def test_write_config_contains_port_keys():
    path = _write_config(8080, 9090, None)
    try:
        with open(path) as f:
            content = f.read()
        assert "port:" in content
    finally:
        os.unlink(path)


def test_write_config_contains_correct_ports():
    path = _write_config(7777, 8888, None)
    try:
        with open(path) as f:
            content = f.read()
        assert "7777" in content
        assert "8888" in content
    finally:
        os.unlink(path)


def test_write_config_no_scenarios_section_when_empty():
    path = _write_config(8080, 9090, None)
    try:
        with open(path) as f:
            content = f.read()
        assert "scenarios:" not in content
    finally:
        os.unlink(path)


def test_write_config_empty_list_omits_scenarios():
    path = _write_config(8080, 9090, [])
    try:
        with open(path) as f:
            content = f.read()
        assert "scenarios:" not in content
    finally:
        os.unlink(path)


# ---------------------------------------------------------------------------
# HTTP API method tests using a fake HTTP server
# ---------------------------------------------------------------------------

import http.server
import threading
from mockly_driver._server import MocklyServer
from mockly_driver._types import Mock, MockRequest, MockResponse, FaultConfig


class _FakeHandler(http.server.BaseHTTPRequestHandler):
    """Records (method, path) and returns configured status codes."""

    def log_message(self, fmt, *args):  # suppress output
        pass

    def _respond(self, status: int):
        self.send_response(status)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        if content_length:
            self.rfile.read(content_length)
        self.server._requests.append(("POST", self.path))
        self._respond(self.server._next_status)

    def do_DELETE(self):
        self.server._requests.append(("DELETE", self.path))
        self._respond(self.server._next_status)


@pytest.fixture()
def fake_http_server():
    """Start a fake HTTP server; yield a controller object; shut down after test."""

    class _Controller:
        def __init__(self, server, thread):
            self._server = server
            self._thread = thread
            self.port = server.server_address[1]
            self.base_url = f"http://127.0.0.1:{self.port}"

        @property
        def requests(self):
            return self._server._requests

        def set_status(self, status: int):
            self._server._next_status = status

        def stop(self):
            self._server.shutdown()
            self._thread.join(timeout=5)

    srv = http.server.HTTPServer(("127.0.0.1", 0), _FakeHandler)
    srv._requests = []
    srv._next_status = 200

    t = threading.Thread(target=srv.serve_forever, daemon=True)
    t.start()

    ctrl = _Controller(srv, t)
    yield ctrl
    ctrl.stop()


def _make_mockly_server(api_base: str) -> MocklyServer:
    server = MocklyServer.__new__(MocklyServer)
    server.http_port = 9999
    server.api_port = 9998
    server.http_base = "http://127.0.0.1:9999"
    server.api_base = api_base
    server._process = None
    server._config_path = None
    return server


def test_add_mock_sends_post(fake_http_server):
    fake_http_server.set_status(201)
    srv = _make_mockly_server(fake_http_server.base_url)
    mock = Mock(
        id="m1",
        request=MockRequest(method="GET", path="/hello"),
        response=MockResponse(status=200, body="hi"),
    )
    srv.add_mock(mock)
    assert ("POST", "/api/mocks/http") in fake_http_server.requests


def test_add_mock_raises_on_500(fake_http_server):
    fake_http_server.set_status(500)
    srv = _make_mockly_server(fake_http_server.base_url)
    mock = Mock(
        id="m1",
        request=MockRequest(method="GET", path="/hello"),
        response=MockResponse(status=200, body="hi"),
    )
    with pytest.raises(RuntimeError):
        srv.add_mock(mock)


def test_delete_mock_sends_delete(fake_http_server):
    fake_http_server.set_status(204)
    srv = _make_mockly_server(fake_http_server.base_url)
    srv.delete_mock("test-id")
    assert ("DELETE", "/api/mocks/http/test-id") in fake_http_server.requests


def test_reset_sends_post(fake_http_server):
    fake_http_server.set_status(200)
    srv = _make_mockly_server(fake_http_server.base_url)
    srv.reset()
    assert ("POST", "/api/reset") in fake_http_server.requests


def test_activate_scenario_sends_post(fake_http_server):
    fake_http_server.set_status(200)
    srv = _make_mockly_server(fake_http_server.base_url)
    srv.activate_scenario("sc1")
    assert ("POST", "/api/scenarios/sc1/activate") in fake_http_server.requests


def test_deactivate_scenario_sends_post(fake_http_server):
    fake_http_server.set_status(200)
    srv = _make_mockly_server(fake_http_server.base_url)
    srv.deactivate_scenario("sc1")
    assert ("POST", "/api/scenarios/sc1/deactivate") in fake_http_server.requests


def test_set_fault_sends_post(fake_http_server):
    fake_http_server.set_status(200)
    srv = _make_mockly_server(fake_http_server.base_url)
    srv.set_fault(FaultConfig(enabled=True, delay="100ms"))
    assert ("POST", "/api/fault") in fake_http_server.requests


def test_clear_fault_sends_delete(fake_http_server):
    fake_http_server.set_status(200)
    srv = _make_mockly_server(fake_http_server.base_url)
    srv.clear_fault()
    assert ("DELETE", "/api/fault") in fake_http_server.requests
