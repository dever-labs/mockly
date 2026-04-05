# Changelog

All notable changes to the `mockly-driver` Java package will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2024-01-01

### Added
- Initial release of the `mockly-driver` Java client.
- `MocklyServer` — start/stop a Mockly server process and call the Management REST API.
- `MocklyInstaller` — download and locate the Mockly binary.
- `MocklyConfig` — builder-style configuration for server startup.
- Model classes: `Mock`, `MockRequest`, `MockResponse`, `Scenario`, `ScenarioPatch`, `FaultConfig`.
- Zero runtime dependencies — uses only the Java 11 standard library.
- JUnit 5 test suite.

