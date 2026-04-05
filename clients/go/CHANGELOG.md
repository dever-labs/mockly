# Changelog

All notable changes to the Mockly Go client will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0] — 2025-01-01

### Added

- `Create(opts Options) (*Server, error)` — start a Mockly server with automatically allocated ports; retries up to 3× on port conflict.
- `Ensure(opts Options, installOpts InstallOptions) (*Server, error)` — install binary then start server.
- `Install(opts InstallOptions) (string, error)` — download platform-specific Mockly binary from GitHub releases (or a custom mirror).
- `GetBinaryPath(binDir string) string` — locate an existing binary via `MOCKLY_BINARY_PATH`, a given directory, or `./bin`.
- `Server.Stop()` — kill the process.
- `Server.AddMock(Mock)` — register a mock via `POST /api/mocks/http`.
- `Server.DeleteMock(id)` — remove a mock via `DELETE /api/mocks/http/{id}`.
- `Server.Reset()` — reset all state via `POST /api/reset`.
- `Server.ActivateScenario(id)` / `Server.DeactivateScenario(id)` — scenario lifecycle.
- `Server.SetFault(FaultConfig)` / `Server.ClearFault()` — fault injection control.
- Environment variable support: `MOCKLY_BINARY_PATH`, `MOCKLY_NO_INSTALL`, `MOCKLY_VERSION`, `MOCKLY_DOWNLOAD_BASE_URL`.
- Automatic proxy support via Go's `net/http` (`HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`).
- Zero external dependencies (stdlib only).

[v0.1.0]: https://github.com/dever-labs/mockly/releases/tag/v0.1.0

