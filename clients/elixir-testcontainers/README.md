# Mockly.Testcontainers

> **SPIKE — see [dever-labs/mockly#371](https://github.com/dever-labs/mockly/issues/371).**
> This package validates feasibility only. It is **not published to Hex**,
> has **no CI workflow**, and is **not wired into this repo's release
> pipeline (release-please)**. See [Spike status](#spike-status) below
> before depending on it for anything beyond local experimentation.

Run Mockly in Docker-backed Elixir tests on top of upstream
[`testcontainers-elixir`](https://hex.pm/packages/testcontainers).

The package starts `ghcr.io/dever-labs/mockly:latest`, waits for the
management API to become ready, and provides a thin HTTP client for mocks,
scenarios, fault injection, and logs/calls.

## Requirements

- Elixir ~> 1.14 / Erlang-OTP with `:inets` and `:ssl` (both part of a
  standard OTP install)
- Docker

## Quickstart

```elixir
{:ok, _pid} = Testcontainers.start_link()

config = Mockly.Testcontainers.Container.new()
{:ok, container} = Testcontainers.start_container(config)
base_url = Mockly.Testcontainers.Container.api_base_url(container)

Mockly.Testcontainers.Client.add_mock(base_url, %{
  "id" => "get-user",
  "request" => %{"method" => "GET", "path" => "/users/1"},
  "response" => %{"status" => 200, "body" => ~s({"id":1})}
})

http_base_url = Mockly.Testcontainers.Container.http_base_url(container)
{:ok, {{_, 200, _}, _headers, body}} =
  :httpc.request(:get, {String.to_charlist("#{http_base_url}/users/1"), []}, [], [])

Testcontainers.stop_container(container.container_id)
```

## Design

Unlike `clients/rust-testcontainers` (which depends on a separate driver
crate), this follows the same self-contained pattern as
`clients/python-testcontainers`: one package, no separate Elixir "driver"
dependency.

- **`Mockly.Testcontainers.Container`** — a config struct + `Testcontainers.ContainerBuilder`
  implementation (the same pattern upstream's `RedisContainer`/`MinioContainer`
  use): `new/0`, `with_image/2`, `with_check_image/2`, `with_inline_config/2`,
  `with_wait_timeout/2`, `with_reuse/2`, and host/port/URL accessors.
- **`Mockly.Testcontainers.Client`** — functions for mocks, state, scenarios,
  fault injection, and logs/calls, built on Erlang's built-in `:httpc` and
  [`Jason`](https://hex.pm/packages/jason) rather than a third-party HTTP
  client library, to keep the dependency footprint small. Every function
  takes the management API's **base URL string** (not the
  `%Testcontainers.Container{}` struct) as its first argument — this
  decouples the client from `Testcontainers`' live registry/GenServer, so it
  can be unit-tested against a plain HTTP stub without Docker.
- Request/response bodies are plain maps mirroring Mockly's JSON wire format
  (see `docs/openapi.yaml` in the main repo), not dedicated structs — this
  keeps the spike's surface area small compared to the typed models in
  `clients/python-testcontainers/src/mockly_testcontainers/_types.py`.

## API

### `Mockly.Testcontainers.Container`

| Function | Description |
|---|---|
| `new/0` | Config struct with Mockly's default image, ports, and inline config. |
| `with_image/2` | Override the Mockly image. |
| `with_check_image/2` | Override the regex used to validate the image before starting. |
| `with_inline_config/2` | Replace the inline `mockly.yaml` written into the container. |
| `with_wait_timeout/2` | Override the readiness-wait timeout (ms). |
| `with_reuse/2` | Reuse an already-running container (testcontainers "reuse" mode). |
| `http_port/1`, `api_port/1` | Host-mapped ports for the started container. |
| `http_base_url/1`, `api_base_url/1` | Base URLs for the mocked protocol / management API. |

### `Mockly.Testcontainers.Client`

Mocks: `add_mock/2`, `list_mocks/1`, `update_mock/3`, `patch_mock/3`, `delete_mock/2`
State: `get_state/1`, `set_state/2`, `delete_state/2`, `reset/1`
Scenarios: `list_scenarios/1`, `create_scenario/2`, `get_scenario/2`, `activate_scenario/2`, `deactivate_scenario/2`, `list_active_scenarios/1`
Faults: `set_fault/2`, `clear_fault/1`
Logs & calls: `get_logs/2`, `clear_logs/1`, `get_calls/2`, `clear_calls/2`, `wait_for_calls/3`

## Testing this package

```sh
mix test                    # unit tests only (no Docker required)
mix test --include integration   # adds the Docker-backed integration test
```

Unit tests run `Client` against a small `:gen_tcp`-based HTTP stub
(`test/support/fake_api.ex`), mirroring `FakeHttpServer` in
`clients/php-testcontainers/tests`. The integration test starts a real
Mockly container via `Testcontainers.start_container/1`.

## Spike status

This package was built to answer: *is a Mockly testcontainers module for
Elixir feasible, and what would it look like?* It is deliberately scoped
down from full client parity:

**Done:**
- `Container` + `Client` modules, compiling and passing unit + integration
  tests against `ghcr.io/dever-labs/mockly:latest`.

**Explicitly deferred pending a decision to pursue full parity:**
- Publishing to [Hex](https://hex.pm) (`mix hex.publish`).
- A CI workflow (`.github/workflows/*`) running this package's test suite.
- `mkdocs` / `docs/` entries and a "driver" (non-Docker) Elixir package —
  see the issue for the open question of whether a standalone driver is
  wanted in addition to this testcontainers module.
- Wiring the version into release-please.

See [dever-labs/mockly#371](https://github.com/dever-labs/mockly/issues/371)
for the full discussion and recommendation.
