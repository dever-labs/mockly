# Changelog

All notable changes to Mockly.Driver will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-01-01

### Added
- Initial release of `Mockly.Driver` for .NET 6+
- `MocklyServer` — start/stop Mockly binary, manage HTTP mocks
- `MocklyInstaller` — automatic binary download with env var overrides
- Models: `Mock`, `MockRequest`, `MockResponse`, `Scenario`, `ScenarioPatch`, `FaultConfig`
- Full async/await API with `IAsyncDisposable` support
- Zero runtime dependencies (BCL only: `System.Text.Json`, `HttpClient`, `System.Net.Sockets`)

