"""Unit tests for mockly-driver (no real binary required)."""

import json
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
from mockly_driver._types import FaultConfig, Mock, MockRequest, MockResponse, MockResponsePatch, Scenario, ScenarioPatch


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


class _RichFakeHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass

    def _respond(self):
        body = self.server._response_body.encode()
        self.send_response(self.server._next_status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _handle(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length).decode() if length else ""
        path = self.path.split("?")[0]
        self.server._requests.append((self.command, path, self.path, body))
        self._respond()

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = _handle


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


@pytest.fixture()
def rich_fake_server():
    class _Controller:
        def __init__(self, server, thread):
            self._server = server
            self._thread = thread
            self.port = server.server_address[1]
            self.base_url = f"http://127.0.0.1:{self.port}"

        @property
        def requests(self):
            return self._server._requests

        def set_response(self, status: int = 200, body: str = ""):
            self._server._next_status = status
            self._server._response_body = body

        def stop(self):
            self._server.shutdown()
            self._thread.join(timeout=5)

    srv = http.server.HTTPServer(("127.0.0.1", 0), _RichFakeHandler)
    srv._requests = []
    srv._next_status = 200
    srv._response_body = ""

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




def _assert_rich_request(server, method: str, path: str, full_path: str | None = None) -> str:
    got_method, got_path, got_full_path, got_body = server.requests[-1]
    assert got_method == method
    assert got_path == path
    if full_path is not None:
        assert got_full_path == full_path
    return got_body


def _sample_mock(mock_id: str = "m1") -> Mock:
    return Mock(
        id=mock_id,
        request=MockRequest(method="GET", path="/ping"),
        response=MockResponse(status=200, body="ok"),
    )


def _sample_scenario(name: str = "Test") -> Scenario:
    return Scenario(
        id="s1",
        name=name,
        patches=[ScenarioPatch(mock_id="m1")],
    )


def _sample_call_entry() -> dict:
    return {
        "id": "c1",
        "timestamp": "2026-01-01T00:00:00Z",
        "protocol": "http",
        "method": "GET",
        "path": "/ping",
        "status": 200,
        "duration_ms": 5,
        "matched_id": "m1",
    }

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
    fake_http_server.set_status(200)
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
    assert ("POST", "/api/fault/http") in fake_http_server.requests


def test_clear_fault_sends_delete(fake_http_server):
    fake_http_server.set_status(204)
    srv = _make_mockly_server(fake_http_server.base_url)
    srv.clear_fault()
    assert ("DELETE", "/api/fault") in fake_http_server.requests


def test_list_mocks_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps([{
        "id": "m1",
        "request": {"method": "GET", "path": "/ping"},
        "response": {"status": 200, "body": "ok"},
    }]))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.list_mocks()
    _assert_rich_request(rich_fake_server, "GET", "/api/mocks/http")
    assert result[0].id == "m1"


def test_list_mocks_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).list_mocks()


def test_update_mock_sends_put(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({
        "id": "m1",
        "request": {"method": "GET", "path": "/ping"},
        "response": {"status": 201, "body": "updated"},
    }))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.update_mock("m1", _sample_mock())
    body = _assert_rich_request(rich_fake_server, "PUT", "/api/mocks/http/m1")
    assert json.loads(body)["id"] == "m1"
    assert result.response.status == 201


def test_update_mock_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).update_mock("m1", _sample_mock())


def test_patch_mock_sends_patch(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({
        "id": "m1",
        "request": {"method": "GET", "path": "/ping"},
        "response": {"status": 201, "body": "patched"},
    }))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.patch_mock("m1", MockResponsePatch(status=201, body="patched"))
    body = _assert_rich_request(rich_fake_server, "PATCH", "/api/mocks/http/m1")
    assert json.loads(body)["status"] == 201
    assert result.response.body == "patched"


def test_patch_mock_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).patch_mock("m1", MockResponsePatch(status=201))


def test_get_state_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({"key1": "val1"}))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.get_state()
    _assert_rich_request(rich_fake_server, "GET", "/api/state")
    assert result["key1"] == "val1"


def test_get_state_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).get_state()


def test_set_state_sends_post(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({"key1": "val1"}))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.set_state({"key1": "val1"})
    body = _assert_rich_request(rich_fake_server, "POST", "/api/state")
    assert json.loads(body)["key1"] == "val1"
    assert result["key1"] == "val1"


def test_set_state_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).set_state({"key1": "val1"})


def test_delete_state_sends_delete(rich_fake_server):
    rich_fake_server.set_response(200, "")
    srv = _make_mockly_server(rich_fake_server.base_url)
    srv.delete_state("key1")
    _assert_rich_request(rich_fake_server, "DELETE", "/api/state/key1")


def test_delete_state_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).delete_state("key1")


def test_get_logs_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps([_sample_call_entry()]))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.get_logs("m1")
    _assert_rich_request(rich_fake_server, "GET", "/api/logs", "/api/logs?matched_id=m1")
    assert result[0].matched_id == "m1"


def test_get_logs_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).get_logs()


def test_clear_logs_sends_delete(rich_fake_server):
    rich_fake_server.set_response(200, "")
    srv = _make_mockly_server(rich_fake_server.base_url)
    srv.clear_logs()
    _assert_rich_request(rich_fake_server, "DELETE", "/api/logs")


def test_clear_logs_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).clear_logs()


def test_get_logs_count_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({"count": 5}))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.get_logs_count("m1")
    _assert_rich_request(rich_fake_server, "GET", "/api/logs/count", "/api/logs/count?matched_id=m1")
    assert result == 5


def test_get_logs_count_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).get_logs_count()


def test_list_scenarios_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps([{"id": "s1", "name": "Test", "patches": []}]))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.list_scenarios()
    _assert_rich_request(rich_fake_server, "GET", "/api/scenarios")
    assert result[0].id == "s1"


def test_list_scenarios_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).list_scenarios()


def test_create_scenario_sends_post(rich_fake_server):
    rich_fake_server.set_response(201, json.dumps({"id": "s1", "name": "Test", "patches": []}))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.create_scenario(_sample_scenario())
    body = _assert_rich_request(rich_fake_server, "POST", "/api/scenarios")
    assert json.loads(body)["id"] == "s1"
    assert result.name == "Test"


def test_create_scenario_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).create_scenario(_sample_scenario())


def test_get_scenario_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({"id": "s1", "name": "Test", "patches": []}))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.get_scenario("s1")
    _assert_rich_request(rich_fake_server, "GET", "/api/scenarios/s1")
    assert result.id == "s1"


def test_get_scenario_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).get_scenario("s1")


def test_update_scenario_sends_put(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({"id": "s1", "name": "Updated", "patches": []}))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.update_scenario("s1", _sample_scenario(name="Updated"))
    body = _assert_rich_request(rich_fake_server, "PUT", "/api/scenarios/s1")
    assert json.loads(body)["name"] == "Updated"
    assert result.name == "Updated"


def test_update_scenario_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).update_scenario("s1", _sample_scenario())


def test_delete_scenario_sends_delete(rich_fake_server):
    rich_fake_server.set_response(200, "")
    srv = _make_mockly_server(rich_fake_server.base_url)
    srv.delete_scenario("s1")
    _assert_rich_request(rich_fake_server, "DELETE", "/api/scenarios/s1")


def test_delete_scenario_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).delete_scenario("s1")


def test_list_active_scenarios_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({
        "active": ["s1"],
        "scenarios": [{"id": "s1", "name": "Test", "patches": []}],
    }))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.list_active_scenarios()
    _assert_rich_request(rich_fake_server, "GET", "/api/scenarios/active")
    assert result.active == ["s1"]
    assert result.scenarios[0].id == "s1"


def test_list_active_scenarios_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).list_active_scenarios()


def test_get_calls_sends_get(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({
        "mock_id": "m1",
        "count": 2,
        "calls": [_sample_call_entry()],
    }))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.get_calls("m1")
    _assert_rich_request(rich_fake_server, "GET", "/api/calls/http/m1")
    assert result.mock_id == "m1"
    assert result.calls[0].id == "c1"


def test_get_calls_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).get_calls("m1")


def test_clear_calls_sends_delete(rich_fake_server):
    rich_fake_server.set_response(200, "")
    srv = _make_mockly_server(rich_fake_server.base_url)
    srv.clear_calls("m1")
    _assert_rich_request(rich_fake_server, "DELETE", "/api/calls/http/m1")


def test_clear_calls_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).clear_calls("m1")


def test_clear_all_calls_sends_delete(rich_fake_server):
    rich_fake_server.set_response(200, "")
    srv = _make_mockly_server(rich_fake_server.base_url)
    srv.clear_all_calls()
    _assert_rich_request(rich_fake_server, "DELETE", "/api/calls/http")


def test_clear_all_calls_raises_on_500(rich_fake_server):
    rich_fake_server.set_response(500, json.dumps({"error": "boom"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).clear_all_calls()


def test_wait_for_calls_sends_post(rich_fake_server):
    rich_fake_server.set_response(200, json.dumps({
        "mock_id": "m1",
        "count": 2,
        "calls": [_sample_call_entry()],
    }))
    srv = _make_mockly_server(rich_fake_server.base_url)
    result = srv.wait_for_calls("m1", count=2, timeout_seconds=5)
    body = _assert_rich_request(rich_fake_server, "POST", "/api/calls/http/m1/wait")
    assert json.loads(body) == {"count": 2, "timeout": "5s"}
    assert result.count == 2
    assert result.calls[0].id == "c1"


def test_wait_for_calls_raises_on_408(rich_fake_server):
    rich_fake_server.set_response(408, json.dumps({"error": "timeout"}))
    with pytest.raises(RuntimeError):
        _make_mockly_server(rich_fake_server.base_url).wait_for_calls("m1", count=2, timeout_seconds=5)
