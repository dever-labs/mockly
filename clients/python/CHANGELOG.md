# Changelog

All notable changes to **mockly-driver** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Fixed

- `set_fault` now correctly calls `POST /api/fault/http` (protocol-scoped endpoint); the old `/api/fault` path does not accept POST.
- `clear_fault` correctly handles the server's `204 No Content` response.

## [v0.12.0] - 2026-06-30

### Added

- `list_mocks()`, `update_mock(mock_id, mock)`, and `patch_mock(mock_id, patch)` for listing and updating HTTP mocks.
- `get_calls(mock_id)`, `clear_calls(mock_id)`, `clear_all_calls()`, and `wait_for_calls(mock_id, count, timeout_seconds)` for recorded-call inspection and polling.
- `get_state()`, `set_state(kv_map)`, and `delete_state(key)` for server state management.
- `get_logs(matched_id=None)`, `clear_logs()`, and `get_logs_count(matched_id=None)` for log retrieval and cleanup.
- `list_scenarios()`, `create_scenario(scenario)`, `get_scenario(scenario_id)`, `update_scenario(scenario_id, scenario)`, `delete_scenario(scenario_id)`, and `list_active_scenarios()` for scenario CRUD and active-scenario inspection.
- New dataclasses: `CallEntry`, `CallSummary`, `MockResponsePatch`, and `ActiveScenariosResponse`.

### Fixed

- `delete_mock(mock_id)` now correctly handles the management API's HTTP 200 response.

## [0.1.0] - 2024-01-01

### Added

- `MocklyServer.create()` — start a Mockly server using an already-installed binary; retries up to 3× on port conflicts.
- `MocklyServer.ensure()` — download binary if needed, then start the server.
- `MocklyServer.stop()` — kill the Mockly process and clean up the temp config file.
- `MocklyServer.add_mock()` — register a dynamic HTTP mock at runtime.
- `MocklyServer.delete_mock()` — remove a mock by ID.
- `MocklyServer.reset()` — remove all dynamic mocks, deactivate scenarios, and clear faults.
- `MocklyServer.activate_scenario()` / `deactivate_scenario()` — toggle pre-configured scenarios.
- `MocklyServer.set_fault()` / `clear_fault()` — inject latency, status overrides, and error rates.
- `install()` — download the platform-correct Mockly binary with proxy and mirror support.
- `get_binary_path()` — locate an existing binary via `MOCKLY_BINARY_PATH`, explicit dir, or `./bin/`.
- `mockly-install` CLI entry point for one-step binary setup.
- Dataclasses: `Mock`, `MockRequest`, `MockResponse`, `Scenario`, `ScenarioPatch`, `FaultConfig`.
- Environment variable support: `MOCKLY_BINARY_PATH`, `MOCKLY_VERSION`, `MOCKLY_DOWNLOAD_BASE_URL`, `MOCKLY_NO_INSTALL`, `HTTPS_PROXY`, `HTTP_PROXY`.
- Stdlib-only runtime — no third-party dependencies.
- pytest unit tests covering port allocation, binary discovery, and install guards.


