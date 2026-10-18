defmodule Mockly.Testcontainers.ContainerTest do
  @moduledoc """
  Verifies `Mockly.Testcontainers.Container`'s config struct and the
  `ContainerBuilder.build/1` translation into a `%Testcontainers.Container{}`
  — all without starting Docker. Complements
  `Mockly.Testcontainers.IntegrationTest`, which actually starts the
  container.
  """

  use ExUnit.Case, async: true

  alias Mockly.Testcontainers.Container
  alias Testcontainers.ContainerBuilder
  alias Testcontainers.HttpWaitStrategy

  test "new/0 uses the default image, wait timeout, and inline config" do
    config = Container.new()

    assert config.image == "ghcr.io/dever-labs/mockly:latest"
    assert config.wait_timeout == 60_000
    assert config.reuse == false
    assert config.config_yaml =~ "port: 9091"
    assert config.config_yaml =~ "port: 8090"
  end

  test "with_image/2 overrides the image" do
    config = Container.new() |> Container.with_image("ghcr.io/dever-labs/mockly:v0.16.0")
    assert config.image == "ghcr.io/dever-labs/mockly:v0.16.0"
  end

  test "with_inline_config/2 overrides the rendered YAML" do
    yaml = "mockly:\n  api:\n    port: 9091\n"
    config = Container.new() |> Container.with_inline_config(yaml)
    assert config.config_yaml == yaml
  end

  test "with_wait_timeout/2 overrides the wait timeout" do
    config = Container.new() |> Container.with_wait_timeout(5_000)
    assert config.wait_timeout == 5_000
  end

  test "with_reuse/2 sets the reuse flag" do
    config = Container.new() |> Container.with_reuse(true)
    assert config.reuse == true
  end

  test "ContainerBuilder.build/1 exposes both ports, copies the config, and sets the command" do
    config = Container.new()
    built = ContainerBuilder.build(config)

    assert built.image == config.image
    assert {8090, nil} in built.exposed_ports
    assert {9091, nil} in built.exposed_ports
    assert built.cmd == ["start", "-c", "/config/mockly.yaml"]
    assert [%{"target" => "/config/mockly.yaml", "contents" => contents}] = built.copy_to
    assert contents == config.config_yaml
  end

  test "ContainerBuilder.build/1 waits on the management API's /api/protocols endpoint" do
    built = Container.new() |> ContainerBuilder.build()

    assert [%HttpWaitStrategy{endpoint: "/api/protocols", port: 9091, timeout: 60_000} = wait] =
             built.wait_strategies

    assert wait.status_code == 200
  end

  test "ContainerBuilder.build/1 propagates a custom wait_timeout to the wait strategy" do
    built = Container.new() |> Container.with_wait_timeout(5_000) |> ContainerBuilder.build()
    assert [%HttpWaitStrategy{timeout: 5_000}] = built.wait_strategies
  end

  test "ContainerBuilder.build/1 propagates the reuse flag" do
    built = Container.new() |> Container.with_reuse(true) |> ContainerBuilder.build()
    assert built.reuse == true
  end

  test "ContainerBuilder.build/1 rejects a clearly non-Mockly image" do
    config = Container.new() |> Container.with_image("redis:latest")

    assert_raise ArgumentError, fn ->
      ContainerBuilder.build(config)
    end
  end
end
