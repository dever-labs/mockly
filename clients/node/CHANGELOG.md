# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.12.0] - 2026-06-30

### Added

- `listMocks()`, `updateMock(id, mock)`, and `patchMock(id, patch)` for listing and updating HTTP mocks.
- `getCalls(mockId)`, `clearCalls(mockId)`, `clearAllCalls()`, and `waitForCalls(mockId, count, timeout)` for recorded-call inspection and polling.
- `getState()`, `setState(kvMap)`, and `deleteState(key)` for server state management.
- `getLogs(matchedId?)`, `clearLogs()`, and `getLogsCount(matchedId?)` for log retrieval and cleanup.
- `listScenarios()`, `createScenario(scenario)`, `getScenario(id)`, `updateScenario(id, scenario)`, `deleteScenario(id)`, and `listActiveScenarios()` for scenario CRUD and active-scenario inspection.
- New types: `CallEntry`, `CallSummary`, `MockResponsePatch`, and `ActiveScenariosResponse`.

### Fixed

- `deleteMock(id)` now correctly handles the management API's HTTP 200 response.

## [0.8.1] - 2026-05-22

### Changed

- Package renamed from `mockly-driver` to `@dever-labs/mockly-driver`.
- The Mockly binary is now downloaded from GitHub releases at `npm install` time via a postinstall script, replacing the previous per-platform optional dependency packages (`@dever-labs/mockly-driver-{platform}`).
- `DEFAULT_MOCKLY_VERSION` now automatically matches the installed npm package version instead of being hardcoded.
- `npx mockly-install` replaces the old `npx mockly-driver-install` command.

### Removed

- Platform sub-packages (`@dever-labs/mockly-driver-linux-x64`, `-linux-arm64`, `-darwin-x64`, `-darwin-arm64`, `-win32-x64`) are no longer published or required.

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

