defmodule Mockly.Testcontainers.Container do
  @moduledoc """
  A `Testcontainers` module that runs Mockly (`ghcr.io/dever-labs/mockly:latest`)
  in a Docker container, waits for the management API to become ready, and
  exposes the host/port needed to reach both the mocked protocol port and the
  management API.

  Mirrors the `Testcontainers.RedisContainer`/`Testcontainers.MinioContainer`
  pattern from upstream `testcontainers-elixir`: build a config struct with
  `new/0` and `with_*/2` helpers, then pass it to `Testcontainers.start_container/1`.

  ## Example

      {:ok, _} = Testcontainers.start_link()

      config = Mockly.Testcontainers.Container.new()
      {:ok, container} = Testcontainers.start_container(config)

      http_base = Mockly.Testcontainers.Container.http_base_url(container)
      api_base = Mockly.Testcontainers.Container.api_base_url(container)

      Testcontainers.stop_container(container.container_id)
  """

  alias Testcontainers.Container
  alias Testcontainers.ContainerBuilder
  alias Testcontainers.HttpWaitStrategy
  alias Mockly.Testcontainers.Container, as: MocklyContainer

  @default_image "ghcr.io/dever-labs/mockly:latest"
  @http_port 8090
  @api_port 9091
  @default_wait_timeout 60_000

  @default_config """
  mockly:
    api:
      port: #{@api_port}
  protocols:
    http:
      enabled: true
      port: #{@http_port}
  """

  @type t :: %__MODULE__{}

  @enforce_keys [:image, :wait_timeout]
  defstruct [
    :image,
    :wait_timeout,
    config_yaml: @default_config,
    check_image: ~r/^ghcr\.io\/dever-labs\/mockly(:.+)?$/,
    reuse: false
  ]

  @doc "Creates a new config struct with Mockly's default image and config."
  def new,
    do: %__MODULE__{
      image: @default_image,
      wait_timeout: @default_wait_timeout
    }

  @doc "Overrides the default Mockly image."
  def with_image(%__MODULE__{} = config, image) when is_binary(image) do
    %{config | image: image}
  end

  @doc """
  Overrides the regular expression used to validate the image before
  starting the container (see `Testcontainers.RedisContainer.with_check_image/2`
  for the same pattern upstream). Useful when pointing at a locally-built or
  differently-tagged Mockly image that wouldn't otherwise match the default.
  """
  def with_check_image(%__MODULE__{} = config, %Regex{} = check_image) do
    %{config | check_image: check_image}
  end

  @doc "Replaces the inline `mockly.yaml` config written into the container."
  def with_inline_config(%__MODULE__{} = config, yaml) when is_binary(yaml) do
    %{config | config_yaml: yaml}
  end

  @doc "Overrides the wait timeout (ms) used while waiting for the management API."
  def with_wait_timeout(%__MODULE__{} = config, timeout) when is_integer(timeout) do
    %{config | wait_timeout: timeout}
  end

  @doc "Sets the reuse flag to reuse an already-running container."
  def with_reuse(%__MODULE__{} = config, reuse) when is_boolean(reuse) do
    %{config | reuse: reuse}
  end

  @doc "The mocked-protocol port exposed on the host for the running container."
  def http_port(%Container{} = container), do: Testcontainers.get_port(container, @http_port)

  @doc "The management API port exposed on the host for the running container."
  def api_port(%Container{} = container), do: Testcontainers.get_port(container, @api_port)

  @doc false
  def default_http_port, do: @http_port

  @doc false
  def default_api_port, do: @api_port

  @doc "Base URL (`http://host:port`) for the mocked HTTP protocol."
  def http_base_url(%Container{} = container) do
    "http://#{Testcontainers.get_host(container)}:#{MocklyContainer.http_port(container)}"
  end

  @doc "Base URL (`http://host:port`) for the management API."
  def api_base_url(%Container{} = container) do
    "http://#{Testcontainers.get_host(container)}:#{MocklyContainer.api_port(container)}"
  end

  defimpl ContainerBuilder do
    import Container

    @spec build(MocklyContainer.t()) :: Container.t()
    @impl true
    def build(%MocklyContainer{} = config) do
      new(config.image)
      |> with_exposed_ports([
        MocklyContainer.default_http_port(),
        MocklyContainer.default_api_port()
      ])
      |> with_copy_to("/config/mockly.yaml", config.config_yaml)
      |> with_cmd(["start", "-c", "/config/mockly.yaml"])
      |> with_reuse(config.reuse)
      |> with_check_image(config.check_image)
      |> with_waiting_strategy(
        HttpWaitStrategy.new("/api/protocols", MocklyContainer.default_api_port(),
          status_code: 200
        )
        |> Map.put(:timeout, config.wait_timeout)
      )
      |> valid_image!()
    end

    @impl true
    def after_start(_config, _container, _conn), do: :ok
  end
end
