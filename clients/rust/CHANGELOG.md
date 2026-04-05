# Changelog

All notable changes to `mockly-driver` will be documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and this project adheres to [Semantic Versioning](https://semver.org/).

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

