# Changelog

All notable changes to the `mockly-driver-kotlin` package will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial release of `mockly-driver-kotlin`: trailing-lambda DSL builders for `MocklyConfig`/`mocklyServer`,
  `Mock`, `Scenario`/`ScenarioPatch`, and `FaultConfig`, plus optional coroutine `suspend` wrappers around
  `MocklyServer`'s blocking HTTP calls.
