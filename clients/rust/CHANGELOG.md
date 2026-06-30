# Changelog

All notable changes to `mockly-driver` will be documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and this project adheres to [Semantic Versioning](https://semver.org/).

## [v0.12.0] - 2026-06-30

### Added

- `list_mocks()`, `update_mock(id, mock)`, and `patch_mock(id, patch)` for listing and updating HTTP mocks.
- `get_calls(mock_id)`, `clear_calls(mock_id)`, `clear_all_calls()`, and `wait_for_calls(mock_id, count, timeout_secs)` for recorded-call inspection and polling.
- `get_state()`, `set_state(state)`, and `delete_state(key)` for server state management.
- `get_logs(matched_id)`, `clear_logs()`, and `get_logs_count(matched_id)` for log retrieval and cleanup.
- `list_scenarios()`, `create_scenario(scenario)`, `get_scenario(id)`, `update_scenario(id, scenario)`, `delete_scenario(id)`, and `list_active_scenarios()` for scenario CRUD and active-scenario inspection.
- New types: `CallEntry`, `CallSummary`, `MockResponsePatch`, and `ActiveScenariosResponse`.

### Fixed

- `delete_mock(id)` now correctly handles the management API's HTTP 200 response.

## [0.1.0] - 2025-01-01

### Added

- `MocklyServer::create` — start a server with a pre-installed binary
- `MocklyServer::ensure` — install binary if needed, then start
- `MocklyServer::stop` and `Drop` implementation for automatic cleanup
- `add_mock`, `delete_mock`, `reset` management API methods
- `activate_scenario`, `deactivate_scenario` scenario methods
- `set_fault`, `clear_fault` fault injection methods
- `install` / `get_binary_path` helpers with env-var overrides
- `get_free_port` utility
- Retry on port conflict (up to 3 attempts)
- Readiness polling via `GET /api/protocols`



