defmodule Mockly.Testcontainers.IntegrationTest do
  @moduledoc """
  End-to-end test: starts a real `ghcr.io/dever-labs/mockly:latest` Docker
  container via `Testcontainers.start_container/1` and exercises
  `Mockly.Testcontainers.Client` against it over the network.

  Requires a running Docker daemon — run with:

      mix test --only integration

  (excluded by default; see `test/test_helper.exs`).
  """

  use ExUnit.Case, async: false

  alias Mockly.Testcontainers.Client
  alias Mockly.Testcontainers.Container, as: MocklyContainer

  @moduletag :integration
  @moduletag timeout: 120_000

  setup_all do
    {:ok, _pid} = start_supervised(Testcontainers)
    # No explicit `Testcontainers.stop_container/1` on_exit here: the
    # library's own Ryuk reaper process removes every container started in
    # this test session once the BEAM VM exits, which sidesteps a teardown
    # race between this module's `on_exit` and `start_supervised`'s own
    # shutdown of the `Testcontainers` GenServer.
    {:ok, container} = Testcontainers.start_container(MocklyContainer.new())

    {:ok, base_url: MocklyContainer.api_base_url(container)}
  end

  test "management API is reachable at /api/protocols", %{base_url: base_url} do
    {:ok, {{_version, status, _reason}, _headers, _body}} =
      :httpc.request(:get, {String.to_charlist("#{base_url}/api/protocols"), []}, [], [])

    assert status == 200
  end

  test "add_mock, call it over HTTP, then reset clears it", %{base_url: base_url} do
    mock = %{
      "id" => "hello-mock",
      "request" => %{"method" => "GET", "path" => "/hello"},
      "response" => %{"status" => 200, "body" => "world"}
    }

    assert {:ok, _} = Client.add_mock(base_url, mock)

    assert {:ok, mocks} = Client.list_mocks(base_url)
    assert Enum.any?(List.wrap(mocks), &(&1["id"] == "hello-mock"))

    assert :ok = Client.reset(base_url)
    assert {:ok, mocks_after_reset} = Client.list_mocks(base_url)
    refute Enum.any?(List.wrap(mocks_after_reset), &(&1["id"] == "hello-mock"))
  end

  test "wait_for_calls observes a call made against the mocked HTTP port" do
    # Uses its own container/mock so it doesn't race with the reset in the
    # other test when run with `async: false` but without ordering guarantees
    # between examples within the same module.
    {:ok, container} = Testcontainers.start_container(MocklyContainer.new())

    base_url = MocklyContainer.api_base_url(container)
    http_base_url = MocklyContainer.http_base_url(container)

    mock = %{
      "id" => "wait-mock",
      "request" => %{"method" => "GET", "path" => "/wait"},
      "response" => %{"status" => 200, "body" => "ok"}
    }

    assert {:ok, _} = Client.add_mock(base_url, mock)

    {:ok, _} = :httpc.request(:get, {String.to_charlist("#{http_base_url}/wait"), []}, [], [])

    assert {:ok, %{"count" => count}} =
             Client.wait_for_calls(base_url, "wait-mock", count: 1, timeout_seconds: 5)

    assert count >= 1
  end
end
