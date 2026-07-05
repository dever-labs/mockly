"""Integration tests — require a running Docker daemon.

Run with:
    pytest -m integration tests/
"""

import urllib.error
import urllib.request

import pytest

from mockly_testcontainers import (
    Mock,
    MockResponsePatch,
    MocklyContainer,
    MockRequest,
    MockResponse,
    Scenario,
)


pytestmark = pytest.mark.integration


@pytest.fixture(scope="module")
def container():
    c = MocklyContainer()
    c.start()
    yield c
    c.stop()


def _fetch(url: str) -> tuple[int, bytes]:
    try:
        with urllib.request.urlopen(url) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read()


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
    assert logs[0].path == "/logging-path"


@pytest.mark.integration
def test_list_mocks(container):
    container.reset()

    container.add_mock(
        Mock(
            id="list-mocks-test",
            request=MockRequest(method="GET", path="/list-mocks"),
            response=MockResponse(status=200, body="ok"),
        )
    )

    mocks = container.list_mocks()
    assert len(mocks) > 0
    assert "list-mocks-test" in {mock.id for mock in mocks}


@pytest.mark.integration
def test_update_and_patch_mock(container):
    container.reset()

    container.add_mock(
        Mock(
            id="upd-test",
            request=MockRequest(method="GET", path="/upd"),
            response=MockResponse(status=200, body="orig"),
        )
    )

    container.update_mock(
        "upd-test",
        Mock(
            id="upd-test",
            request=MockRequest(method="GET", path="/upd"),
            response=MockResponse(status=200, body="updated"),
        ),
    )

    status, body = _fetch(f"{container.get_http_base()}/upd")
    assert status == 200
    assert body == b"updated"

    container.patch_mock("upd-test", MockResponsePatch(status=418))

    status, _ = _fetch(f"{container.get_http_base()}/upd")
    assert status == 418


@pytest.mark.integration
def test_state_crud(container):
    container.reset()

    container.set_state({"k": "v"})
    state = container.get_state()
    assert state["k"] == "v"

    container.delete_state("k")
    state = container.get_state()
    assert "k" not in state


@pytest.mark.integration
def test_get_logs_count(container):
    container.reset()
    container.clear_logs()

    try:
        urllib.request.urlopen(container.get_http_base())
    except urllib.error.HTTPError:
        pass

    assert container.get_logs_count() > 0
    container.get_logs_count(matched_id="nonexistent")


@pytest.mark.integration
def test_scenario_crud(container):
    container.reset()

    container.list_scenarios()

    container.create_scenario(Scenario(id="tc-s", name="TC", patches=[]))

    scenario = container.get_scenario("tc-s")
    assert scenario.id == "tc-s"

    scenarios = container.list_scenarios()
    assert len(scenarios) > 0

    container.delete_scenario("tc-s")


@pytest.mark.integration
def test_get_calls_and_clear(container):
    container.reset()

    container.add_mock(
        Mock(
            id="calls",
            request=MockRequest(method="GET", path="/calls"),
            response=MockResponse(status=200, body="ok"),
        )
    )

    with urllib.request.urlopen(f"{container.get_http_base()}/calls") as response:
        assert response.status == 200

    calls = container.get_calls("calls")
    assert calls.count > 0

    container.clear_calls("calls")
    calls = container.get_calls("calls")
    assert calls.count == 0

    container.clear_all_calls()


@pytest.mark.integration
def test_wait_for_calls(container):
    container.reset()

    container.add_mock(
        Mock(
            id="wait",
            request=MockRequest(method="GET", path="/wait"),
            response=MockResponse(status=200, body="ok"),
        )
    )

    with urllib.request.urlopen(f"{container.get_http_base()}/wait") as response:
        assert response.status == 200

    calls = container.wait_for_calls("wait", count=1, timeout_seconds=10)
    assert calls.count >= 1
