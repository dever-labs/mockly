from dataclasses import asdict
from pathlib import Path
from unittest.mock import MagicMock

from mockly_testcontainers import MocklyContainer
from mockly_testcontainers._types import FaultConfig, Mock, MockRequest, MockResponse
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

    container.set_fault(FaultConfig(enabled=True, delay="50ms", error_rate=0.1))

    container._request.assert_called_once_with(
        "POST",
        "/api/fault",
        {"enabled": True, "delay": "50ms", "error_rate": 0.1},
        expected=(200,),
    )


def test_get_logs_returns_decoded_body():
    container = MocklyContainer()
    container._request = MagicMock(return_value=b'[{"matched_id":"users"}]')

    logs = container.get_logs()

    assert logs == '[{"matched_id":"users"}]'
    container._request.assert_called_once_with("GET", "/api/logs", expected=(200,))
