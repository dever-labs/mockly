# Changelog

All notable changes to **mockly-testcontainers** (PHP) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- Initial PHP testcontainers module: `MocklyContainer` wrapping
  `testcontainers/testcontainers`'s `GenericContainer`, running
  `ghcr.io/dever-labs/mockly:latest`.
- `start()` / `stop()`, `withInlineConfig()`, `withScenarios()`.
- Full management API parity with the `mockly-driver` package: mock CRUD, state,
  logs, scenario CRUD, fault injection, call inspection.
- PHPUnit integration tests (tagged `#[Group('integration')]`, excluded from
  `composer test`) covering container startup, mock serving, scenario activation,
  and fault injection against a real Docker daemon.
