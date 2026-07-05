from dataclasses import asdict
from pathlib import Path
from unittest.mock import MagicMock

from mockly_testcontainers import MocklyContainer
from mockly_testcontainers._types import (
    CallEntry,
    FaultConfig,
    Mock,
    MockRequest,
    MockResponse,
    MockResponsePatch,
    MocklyServerOptions,
    Scenario,
    ScenarioPatch,
)
from mockly_testcontainers.container import (
    API_PORT,
    CONTAINER_CONFIG_PATH,
    DEFAULT_CONFIG,
    DEFAULT_IMAGE,
    HTTP_PORT,
)


def test_default_image():
    assert DEFAULT_IMAGE == "ghcr.io/dever-labs/mockly:latest"


def test_default_ports():
    assert HTTP_PORT == 8090
    assert API_PORT == 9091


def test_with_inline_config():
    container = MocklyContainer()
    yaml = "protocols:\n  http:\n    enabled: true"
    result = container.with_inline_config(yaml)
    assert result is container
    assert container._config_yaml == yaml


def test_default_config_contains_api_port():
    container = MocklyContainer()
    assert container._config_yaml == DEFAULT_CONFIG
    assert "api:\n    port: 9091" in container._config_yaml


def test_configure_sets_command_and_mounts_config():
    container = MocklyContainer().with_inline_config("mockly:\n  api:\n    port: 9091\n")

    try:
        container._configure()

        assert container._command == f"start -c {CONTAINER_CONFIG_PATH}"
        assert container._config_host_path is not None
        assert Path(container._config_host_path).read_text() == container._config_yaml
        assert container.volumes[container._config_host_path] == {
            "bind": CONTAINER_CONFIG_PATH,
            "mode": "ro",
        }
    finally:
        container._cleanup_config_file()


def test_mock_json_serialization():
    mock = Mock(
        id="test",
        request=MockRequest(method="GET", path="/hello"),
        response=MockResponse(status=200, body="world"),
    )
    data = asdict(mock)
    assert data["id"] == "test"
    assert data["request"]["method"] == "GET"
    assert data["response"]["status"] == 200


def test_add_mock_posts_serialized_body():
    container = MocklyContainer()
    container._request = MagicMock()

    mock = Mock(
        id="test",
        request=MockRequest(method="GET", path="/hello"),
        response=MockResponse(status=200, body="world"),
    )

    container.add_mock(mock)

    container._request.assert_called_once_with(
        "POST",
        "/api/mocks/http",
        {
            "id": "test",
            "request": {"method": "GET", "path": "/hello"},
            "response": {"status": 200, "body": "world"},
        },
        expected=(200, 201),
    )


def test_set_fault_posts_config():
    container = MocklyContainer()
    container._request = MagicMock()

    container.set_fault(FaultConfig(enabled=True, delay="50ms", error_rate=0.1, status=503))

    container._request.assert_called_once_with(
        "POST",
        "/api/fault/http",
        {"enabled": True, "delay": "50ms", "error_rate": 0.1, "status": 503},
        expected=(200,),
    )


def test_get_logs_returns_call_entries():
    container = MocklyContainer()
    container._request = MagicMock(return_value=b'[{"matched_id":"users"}]')

    logs = container.get_logs()

    assert logs == [
        CallEntry(
            id="",
            timestamp="",
            protocol="",
            path="",
            duration_ms=0,
            matched_id="users",
        )
    ]
    container._request.assert_called_once_with("GET", "/api/logs", expected=(200,))


def test_get_logs_with_matched_id_adds_query_param():
    container = MocklyContainer()
    container._request = MagicMock(return_value=b"[]")

    container.get_logs("users/mock")

    container._request.assert_called_once_with(
        "GET",
        "/api/logs?matched_id=users%2Fmock",
        expected=(200,),
    )


def test_get_logs_count_returns_count():
    container = MocklyContainer()
    container._request = MagicMock(return_value=b'{"count":3}')

    count = container.get_logs_count("users")

    assert count == 3
    container._request.assert_called_once_with(
        "GET",
        "/api/logs/count?matched_id=users",
        expected=(200,),
    )


def test_delete_mock_accepts_200():
    container = MocklyContainer()
    container._request = MagicMock()

    container.delete_mock("mock-id")

    container._request.assert_called_once_with(
        "DELETE",
        "/api/mocks/http/mock-id",
        expected=(200,),
    )


def test_wait_for_calls_posts_expected_payload():
    container = MocklyContainer()
    container._request = MagicMock(return_value=b'{"mock_id":"m1","count":2,"calls":[]}')

    result = container.wait_for_calls("m1", count=2, timeout_seconds=15)

    assert result.mock_id == "m1"
    assert result.count == 2
    container._request.assert_called_once_with(
        "POST",
        "/api/calls/http/m1/wait",
        {"count": 2, "timeout": "15s"},
        expected=(200,),
    )


def test_with_options_renders_scenarios_into_config():
    container = MocklyContainer().with_options(
        MocklyServerOptions(
            scenarios=[
                Scenario(
                    id="happy-path",
                    name="Happy path",
                    description="switch response",
                    patches=[
                        ScenarioPatch(
                            mock_id="users",
                            status=201,
                            body="created",
                            headers={"X-Test": "1"},
                            delay="10ms",
                            disabled=False,
                        )
                    ],
                )
            ]
        )
    )

    assert "scenarios:" in container._config_yaml
    assert 'id: "happy-path"' in container._config_yaml
    assert 'name: "Happy path"' in container._config_yaml
    assert 'description: "switch response"' in container._config_yaml
    assert 'mock_id: "users"' in container._config_yaml
    assert "status: 201" in container._config_yaml
    assert 'body: "created"' in container._config_yaml
    assert '"X-Test": "1"' in container._config_yaml
    assert 'delay: "10ms"' in container._config_yaml
    assert "disabled: false" in container._config_yaml


def test_patch_mock_serializes_patch_body():
    container = MocklyContainer()
    container._request = MagicMock(
        return_value=b'{"id":"users","request":{"method":"GET","path":"/users"},"response":{"status":201}}'
    )

    result = container.patch_mock("users", MockResponsePatch(status=201))

    assert result.response.status == 201
    container._request.assert_called_once_with(
        "PATCH",
        "/api/mocks/http/users",
        {"status": 201},
        expected=(200,),
    )
