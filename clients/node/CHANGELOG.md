# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-09-22

### Added

- `MocklyServer.create()` — start a Mockly process with automatically allocated ports.
- `MocklyServer.ensure()` — download the Mockly binary if needed, then start.
- `MocklyServer.stop()` — graceful shutdown.
- Management API helpers: `addMock()`, `deleteMock()`, `reset()`, `activateScenario()`, `deactivateScenario()`, `setFault()`, `clearFault()`.
- `install()` — programmatic binary download with Artifactory / proxy / air-gap support.
- `getBinaryPath()` — resolve binary location (env override, `bin/`, `node_modules/.bin/`).
- `npx mockly-install` CLI for CI setup steps.
- Retry logic in `create()` on port-conflict errors (up to 3 attempts).
- Stderr capture forwarded to startup error messages for easier debugging.

