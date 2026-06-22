"""Integration tests — require a running Docker daemon.

Run with:
    pytest -m integration tests/
"""

import json
import urllib.error
import urllib.request

import pytest

from mockly_testcontainers import Mock, MocklyContainer, MockRequest, MockResponse


pytestmark = pytest.mark.integration


@pytest.fixture(scope="module")
def container():
    c = MocklyContainer()
    c.start()
    yield c
    c.stop()


def test_smoke(container):
    assert container.get_http_base().startswith("http://")
    assert container.get_api_base().startswith("http://")

    with urllib.request.urlopen(f"{container.get_api_base()}/api/protocols") as response:
        assert response.status == 200


def test_add_mock_and_reset(container):
    container.add_mock(
        Mock(
            id="hello-mock",
            request=MockRequest(method="GET", path="/hello"),
            response=MockResponse(status=200, body="world"),
        )
    )

    with urllib.request.urlopen(f"{container.get_http_base()}/hello") as response:
        assert response.status == 200
        assert response.read() == b"world"

    container.reset()

    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(f"{container.get_http_base()}/hello")

    assert exc_info.value.code != 200


def test_get_logs(container):
    container.clear_logs()

    try:
        urllib.request.urlopen(f"{container.get_http_base()}/logging-path")
    except urllib.error.HTTPError:
        pass

    logs = container.get_logs()
    assert len(logs) > 0
    json.loads(logs)
