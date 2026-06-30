# Changelog

All notable changes to the `mockly-driver` Java package will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `setFault` now correctly calls `POST /api/fault/http` (protocol-scoped endpoint); the old `/api/fault` path does not accept POST.
- `clearFault` correctly handles the server's `204 No Content` response.

## [v0.12.0] - 2026-06-30

### Added

- `listMocks()`, `updateMock(id, mock)`, and `patchMock(id, patch)` for listing and updating HTTP mocks.
- `getCalls(mockId)`, `clearCalls(mockId)`, `clearAllCalls()`, and `waitForCalls(mockId, count, timeout)` for recorded-call inspection and polling.
- `getState()`, `setState(kvMap)`, and `deleteState(key)` for server state management.
- `getLogs()`, `getLogs(matchedId)`, `clearLogs()`, `getLogsCount()`, and `getLogsCount(matchedId)` for log retrieval and cleanup.
- `listScenarios()`, `createScenario(scenario)`, `getScenario(id)`, `updateScenario(id, scenario)`, `deleteScenario(id)`, and `listActiveScenarios()` for scenario CRUD and active-scenario inspection.
- New model types: `CallEntry`, `CallSummary`, `MockResponsePatch`, and `ActiveScenariosResponse`.

### Fixed

- `deleteMock(id)` now correctly handles the management API's HTTP 200 response.

## [0.1.0] - 2024-01-01

### Added
- Initial release of the `mockly-driver` Java client.
- `MocklyServer` — start/stop a Mockly server process and call the Management REST API.
- `MocklyInstaller` — download and locate the Mockly binary.
- `MocklyConfig` — builder-style configuration for server startup.
- Model classes: `Mock`, `MockRequest`, `MockResponse`, `Scenario`, `ScenarioPatch`, `FaultConfig`.
- Zero runtime dependencies — uses only the Java 11 standard library.
- JUnit 5 test suite.

