# Changelog

All notable changes to Mockly.Driver will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.12.0] - 2026-06-30

### Added

- `ListMocksAsync()`, `UpdateMockAsync(id, mock)`, and `PatchMockAsync(id, patch)` for listing and updating HTTP mocks.
- `GetCallsAsync(mockId)`, `ClearCallsAsync(mockId)`, `ClearAllCallsAsync()`, and `WaitForCallsAsync(mockId, count, timeout)` for recorded-call inspection and polling.
- `GetStateAsync()`, `SetStateAsync(kvMap)`, and `DeleteStateAsync(key)` for server state management.
- `GetLogsAsync(matchedId)`, `ClearLogsAsync()`, and `GetLogsCountAsync(matchedId)` for log retrieval and cleanup.
- `ListScenariosAsync()`, `CreateScenarioAsync(scenario)`, `GetScenarioAsync(id)`, `UpdateScenarioAsync(id, scenario)`, `DeleteScenarioAsync(id)`, and `ListActiveScenariosAsync()` for scenario CRUD and active-scenario inspection.
- New record types: `CallEntry`, `CallSummary`, `MockResponsePatch`, and `ActiveScenariosResponse`.

### Fixed

- `DeleteMockAsync(id)` now correctly handles the management API's HTTP 200 response.

## [0.1.0] - 2025-01-01

### Added
- Initial release of `Mockly.Driver` for .NET 6+
- `MocklyServer` — start/stop Mockly binary, manage HTTP mocks
- `MocklyInstaller` — automatic binary download with env var overrides
- Models: `Mock`, `MockRequest`, `MockResponse`, `Scenario`, `ScenarioPatch`, `FaultConfig`
- Full async/await API with `IAsyncDisposable` support
- Zero runtime dependencies (BCL only: `System.Text.Json`, `HttpClient`, `System.Net.Sockets`)

