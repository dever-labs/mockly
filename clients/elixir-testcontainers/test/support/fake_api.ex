defmodule Mockly.Testcontainers.Test.FakeApi do
  @moduledoc """
  A tiny `:gen_tcp`-based HTTP/1.1 stub standing in for Mockly's management
  API, so `Mockly.Testcontainers.Client` can be exercised without Docker.

  Mirrors the role of `FakeHttpServer` in `clients/php-testcontainers/tests`:
  one process accepts connections, parses the request line/headers/body,
  hands it to an `Agent` for later assertions, and replies with a canned
  (status, body) response configured via `set_response/3`.
  """

  defstruct status: 200, body: "", last_request: nil

  @reason_phrases %{
    200 => "OK",
    201 => "Created",
    204 => "No Content",
    404 => "Not Found",
    408 => "Request Timeout",
    500 => "Internal Server Error"
  }

  def start_link do
    {:ok, agent} = Agent.start(fn -> %__MODULE__{} end)

    {:ok, listen_socket} =
      :gen_tcp.listen(0, [:binary, packet: :raw, active: false, reuseaddr: true])

    {:ok, port} = :inet.port(listen_socket)

    acceptor =
      spawn(fn ->
        accept_loop(listen_socket, agent)
      end)

    %{agent: agent, port: port, listen_socket: listen_socket, acceptor: acceptor}
  end

  def stop(%{listen_socket: listen_socket, acceptor: acceptor, agent: agent}) do
    Process.exit(acceptor, :kill)
    :gen_tcp.close(listen_socket)
    Agent.stop(agent)
  end

  def base_url(%{port: port}), do: "http://127.0.0.1:#{port}"

  def set_response(%{agent: agent}, status, body) do
    Agent.update(agent, fn state -> %{state | status: status, body: body} end)
  end

  def last_request(%{agent: agent}), do: Agent.get(agent, & &1.last_request)

  defp accept_loop(listen_socket, agent) do
    case :gen_tcp.accept(listen_socket) do
      {:ok, socket} ->
        spawn(fn -> handle_connection(socket, agent) end)
        accept_loop(listen_socket, agent)

      {:error, :closed} ->
        :ok
    end
  end

  defp handle_connection(socket, agent) do
    request = read_request(socket, "")
    Agent.update(agent, fn state -> %{state | last_request: request} end)

    %{status: status, body: body} = Agent.get(agent, & &1)
    reason = Map.get(@reason_phrases, status, "Unknown")

    response =
      "HTTP/1.1 #{status} #{reason}\r\n" <>
        "content-type: application/json\r\n" <>
        "content-length: #{byte_size(body)}\r\n" <>
        "connection: close\r\n\r\n" <>
        body

    :gen_tcp.send(socket, response)
    :gen_tcp.close(socket)
  end

  # Reads until the full header block + any declared content-length body has
  # arrived, then parses out method/path/body for assertions.
  defp read_request(socket, acc) do
    case :gen_tcp.recv(socket, 0, 2_000) do
      {:ok, data} ->
        acc = acc <> data

        case parse(acc) do
          {:incomplete, _} -> read_request(socket, acc)
          parsed -> parsed
        end

      {:error, :closed} ->
        parse(acc)

      {:error, reason} ->
        raise "FakeApi: failed to read request: #{inspect(reason)}"
    end
  end

  defp parse(raw) do
    case String.split(raw, "\r\n\r\n", parts: 2) do
      [head, rest] ->
        [request_line | header_lines] = String.split(head, "\r\n")
        [method, path, _version] = String.split(request_line, " ")
        content_length = content_length(header_lines)

        if byte_size(rest) < content_length do
          {:incomplete, raw}
        else
          %{method: method, path: path, body: binary_part(rest, 0, content_length)}
        end

      _ ->
        {:incomplete, raw}
    end
  end

  defp content_length(header_lines) do
    Enum.find_value(header_lines, 0, fn line ->
      case String.split(line, ":", parts: 2) do
        [name, value] ->
          if String.downcase(String.trim(name)) == "content-length" do
            value |> String.trim() |> String.to_integer()
          end

        _ ->
          nil
      end
    end)
  end
end
