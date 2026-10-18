defmodule Mockly.Testcontainers.ClientTest do
  @moduledoc """
  Exercises every `Mockly.Testcontainers.Client` function against the fake
  management API (`Mockly.Testcontainers.Test.FakeApi`), without needing
  Docker or a running `Testcontainers` process. Complements
  `Mockly.Testcontainers.IntegrationTest`, which validates the real
  container wiring end-to-end.
  """

  use ExUnit.Case, async: true

  alias Mockly.Testcontainers.Client
  alias Mockly.Testcontainers.Test.FakeApi

  setup do
    fake = FakeApi.start_link()
    on_exit(fn -> FakeApi.stop(fake) end)
    {:ok, fake: fake, base_url: FakeApi.base_url(fake)}
  end

  test "add_mock sends POST with JSON body", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 201, ~s({"id":"m1"}))

    assert {:ok, %{"id" => "m1"}} = Client.add_mock(base_url, %{"id" => "m1"})

    req = FakeApi.last_request(fake)
    assert req.method == "POST"
    assert req.path == "/api/mocks/http"
    assert req.body == ~s({"id":"m1"})
  end

  test "list_mocks sends GET and decodes a JSON array", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 200, ~s([{"id":"m1"}]))

    assert {:ok, [%{"id" => "m1"}]} = Client.list_mocks(base_url)
    assert FakeApi.last_request(fake).method == "GET"
    assert FakeApi.last_request(fake).path == "/api/mocks/http"
  end

  test "update_mock sends PUT to the mock's path", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 200, ~s({"id":"m1"}))
    assert {:ok, _} = Client.update_mock(base_url, "m1", %{"id" => "m1"})

    assert FakeApi.last_request(fake).method == "PUT"
    assert FakeApi.last_request(fake).path == "/api/mocks/http/m1"
  end

  test "patch_mock sends PATCH", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 200, ~s({"id":"m1"}))
    assert {:ok, _} = Client.patch_mock(base_url, "m1", %{"status" => 503})

    assert FakeApi.last_request(fake).method == "PATCH"
    assert FakeApi.last_request(fake).body == ~s({"status":503})
  end

  test "delete_mock sends DELETE and returns :ok", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 204, "")
    assert :ok = Client.delete_mock(base_url, "m1")

    assert FakeApi.last_request(fake).method == "DELETE"
    assert FakeApi.last_request(fake).path == "/api/mocks/http/m1"
  end

  test "reset sends POST with an empty body (no JSON null)", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 200, "")
    assert :ok = Client.reset(base_url)

    req = FakeApi.last_request(fake)
    assert req.method == "POST"
    assert req.path == "/api/reset"
    assert req.body == ""
  end

  test "activate_scenario sends POST to the activate path", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 200, "")
    assert :ok = Client.activate_scenario(base_url, "s1")
    assert FakeApi.last_request(fake).path == "/api/scenarios/s1/activate"
  end

  test "get_logs without a matched_id omits the query string", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 200, "[]")
    assert {:ok, []} = Client.get_logs(base_url)
    assert FakeApi.last_request(fake).path == "/api/logs"
  end

  test "get_logs with a matched_id appends the query string", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 200, "[]")
    assert {:ok, []} = Client.get_logs(base_url, "m1")
    assert FakeApi.last_request(fake).path == "/api/logs?matched_id=m1"
  end

  test "wait_for_calls encodes count/timeout into the request body", %{
    fake: fake,
    base_url: base_url
  } do
    FakeApi.set_response(fake, 200, ~s({"mock_id":"m1","count":1}))

    assert {:ok, %{"count" => 1}} =
             Client.wait_for_calls(base_url, "m1", count: 1, timeout_seconds: 5)

    req = FakeApi.last_request(fake)
    assert req.path == "/api/calls/http/m1/wait"
    assert req.body == ~s({"count":1,"timeout":"5s"})
  end

  test "non-2xx responses are returned as an error tuple", %{fake: fake, base_url: base_url} do
    FakeApi.set_response(fake, 500, ~s({"error":"boom"}))

    assert {:error, {:unexpected_status, 500, %{"error" => "boom"}}} =
             Client.add_mock(base_url, %{"id" => "m1"})
  end
end
