defmodule Mockly.Testcontainers.Client do
  @moduledoc """
  A thin HTTP client for Mockly's management API.

  Every function takes the management API's base URL (e.g.
  `Mockly.Testcontainers.Container.api_base_url(container)` for a container
  started via `Testcontainers.start_container/1`) as its first argument,
  rather than the `%Testcontainers.Container{}` struct itself — this keeps
  the client fully decoupled from `Testcontainers`' live registry/GenServer,
  so it can be unit-tested against a plain HTTP stub without Docker or a
  running `Testcontainers` process.

  Request/response bodies are plain maps mirroring Mockly's JSON wire format
  (see `docs/openapi.yaml` in the main repo) rather than dedicated structs —
  this keeps the spike's surface area small; see README.md for scope notes.
  """

  @type base_url :: String.t()

  # --- Mocks -----------------------------------------------------------

  @spec add_mock(base_url(), map()) :: {:ok, map()} | {:error, term()}
  def add_mock(base_url, mock), do: post(base_url, "/api/mocks/http", mock)

  @spec list_mocks(base_url()) :: {:ok, [map()]} | {:error, term()}
  def list_mocks(base_url), do: get(base_url, "/api/mocks/http")

  @spec update_mock(base_url(), String.t(), map()) :: {:ok, map()} | {:error, term()}
  def update_mock(base_url, mock_id, mock),
    do: put(base_url, "/api/mocks/http/#{URI.encode(mock_id)}", mock)

  @spec patch_mock(base_url(), String.t(), map()) :: {:ok, map()} | {:error, term()}
  def patch_mock(base_url, mock_id, patch),
    do: patch(base_url, "/api/mocks/http/#{URI.encode(mock_id)}", patch)

  @spec delete_mock(base_url(), String.t()) :: :ok | {:error, term()}
  def delete_mock(base_url, mock_id),
    do: delete(base_url, "/api/mocks/http/#{URI.encode(mock_id)}") |> as_ok()

  # --- State -------------------------------------------------------------

  @spec get_state(base_url()) :: {:ok, map()} | {:error, term()}
  def get_state(base_url), do: get(base_url, "/api/state")

  @spec set_state(base_url(), map()) :: {:ok, map()} | {:error, term()}
  def set_state(base_url, kv_map), do: post(base_url, "/api/state", kv_map)

  @spec delete_state(base_url(), String.t()) :: :ok | {:error, term()}
  def delete_state(base_url, key),
    do: delete(base_url, "/api/state/#{URI.encode(key)}") |> as_ok()

  @spec reset(base_url()) :: :ok | {:error, term()}
  def reset(base_url), do: post(base_url, "/api/reset", nil) |> as_ok()

  # --- Scenarios -----------------------------------------------------------

  @spec list_scenarios(base_url()) :: {:ok, [map()]} | {:error, term()}
  def list_scenarios(base_url), do: get(base_url, "/api/scenarios")

  @spec create_scenario(base_url(), map()) :: {:ok, map()} | {:error, term()}
  def create_scenario(base_url, scenario), do: post(base_url, "/api/scenarios", scenario)

  @spec get_scenario(base_url(), String.t()) :: {:ok, map()} | {:error, term()}
  def get_scenario(base_url, scenario_id),
    do: get(base_url, "/api/scenarios/#{URI.encode(scenario_id)}")

  @spec activate_scenario(base_url(), String.t()) :: :ok | {:error, term()}
  def activate_scenario(base_url, scenario_id),
    do: post(base_url, "/api/scenarios/#{URI.encode(scenario_id)}/activate", nil) |> as_ok()

  @spec deactivate_scenario(base_url(), String.t()) :: :ok | {:error, term()}
  def deactivate_scenario(base_url, scenario_id),
    do: post(base_url, "/api/scenarios/#{URI.encode(scenario_id)}/deactivate", nil) |> as_ok()

  @spec list_active_scenarios(base_url()) :: {:ok, map()} | {:error, term()}
  def list_active_scenarios(base_url), do: get(base_url, "/api/scenarios/active")

  # --- Fault injection -----------------------------------------------------

  @spec set_fault(base_url(), map()) :: :ok | {:error, term()}
  def set_fault(base_url, config), do: post(base_url, "/api/fault/http", config) |> as_ok()

  @spec clear_fault(base_url()) :: :ok | {:error, term()}
  def clear_fault(base_url), do: delete(base_url, "/api/fault") |> as_ok()

  # --- Logs & calls --------------------------------------------------------

  @spec get_logs(base_url(), String.t() | nil) :: {:ok, [map()]} | {:error, term()}
  def get_logs(base_url, matched_id \\ nil),
    do: get(base_url, with_optional_matched_id("/api/logs", matched_id))

  @spec clear_logs(base_url()) :: :ok | {:error, term()}
  def clear_logs(base_url), do: delete(base_url, "/api/logs") |> as_ok()

  @spec get_calls(base_url(), String.t()) :: {:ok, map()} | {:error, term()}
  def get_calls(base_url, mock_id), do: get(base_url, "/api/calls/http/#{URI.encode(mock_id)}")

  @spec clear_calls(base_url(), String.t()) :: :ok | {:error, term()}
  def clear_calls(base_url, mock_id),
    do: delete(base_url, "/api/calls/http/#{URI.encode(mock_id)}") |> as_ok()

  @spec wait_for_calls(base_url(), String.t(), count: integer(), timeout_seconds: integer()) ::
          {:ok, map()} | {:error, term()}
  def wait_for_calls(base_url, mock_id, opts \\ []) do
    body = %{
      "count" => Keyword.get(opts, :count, 1),
      "timeout" => "#{Keyword.get(opts, :timeout_seconds, 10)}s"
    }

    post(base_url, "/api/calls/http/#{URI.encode(mock_id)}/wait", body)
  end

  # --- Internal request helpers --------------------------------------------

  defp get(base_url, path), do: request(base_url, :get, path, nil)
  defp post(base_url, path, body), do: request(base_url, :post, path, body)
  defp put(base_url, path, body), do: request(base_url, :put, path, body)
  defp patch(base_url, path, body), do: request(base_url, :patch, path, body)
  defp delete(base_url, path), do: request(base_url, :delete, path, nil)

  # Uses the built-in `:httpc`/`Jason` rather than pulling in an HTTP client
  # library, keeping this spike's dependency footprint to just
  # `testcontainers` + `jason`.
  # Methods that carry a request body always need `:httpc`'s 4-tuple request
  # form (`{url, headers, content_type, body}`), even with an empty body —
  # passing the 2-tuple form (valid for GET/DELETE) for these returns
  # `{:error, :invalid_request}`.
  @body_methods [:post, :put, :patch]

  defp request(base_url, method, path, body) do
    url = String.to_charlist(base_url <> path)
    content_type = ~c"application/json"

    http_request =
      if method in @body_methods do
        encoded_body = if body, do: Jason.encode!(body), else: ""
        {url, [], content_type, encoded_body}
      else
        {url, []}
      end

    case :httpc.request(method, http_request, [], body_format: :binary) do
      {:ok, {{_http_version, status, _reason}, _headers, resp_body}} when status in 200..299 ->
        {:ok, decode_body(resp_body)}

      {:ok, {{_http_version, status, _reason}, _headers, resp_body}} ->
        {:error, {:unexpected_status, status, decode_body(resp_body)}}

      {:error, reason} ->
        {:error, reason}
    end
  end

  defp decode_body(""), do: nil

  defp decode_body(raw) do
    case Jason.decode(raw) do
      {:ok, decoded} -> decoded
      {:error, _} -> raw
    end
  end

  defp as_ok({:ok, _body}), do: :ok
  defp as_ok({:error, _} = error), do: error

  defp with_optional_matched_id(path, nil), do: path

  defp with_optional_matched_id(path, matched_id),
    do: "#{path}?matched_id=#{URI.encode(matched_id)}"
end
