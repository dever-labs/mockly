# Changelog

All notable changes to **mockly-driver** (PHP) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- Initial PHP driver client: `MocklyServer::create()` / `::ensure()` / `stop()`.
- Mock CRUD: `addMock()`, `listMocks()`, `updateMock()`, `patchMock()`, `deleteMock()`.
- State management: `getState()`, `setState()`, `deleteState()`.
- Log retrieval: `getLogs()`, `clearLogs()`, `getLogsCount()`.
- Scenario CRUD: `listScenarios()`, `createScenario()`, `getScenario()`, `updateScenario()`,
  `deleteScenario()`, `listActiveScenarios()`, `activateScenario()`, `deactivateScenario()`.
- Fault injection: `setFault()`, `clearFault()`.
- Call inspection: `getCalls()`, `clearCalls()`, `clearAllCalls()`, `waitForCalls()`.
- `reset()` — remove all dynamic mocks, deactivate scenarios, clear faults.
- `Installer::install()` / `::getBinaryPath()` — platform-correct binary download with
  `MOCKLY_BINARY_PATH`, `MOCKLY_VERSION`, `MOCKLY_DOWNLOAD_BASE_URL`, `MOCKLY_NO_INSTALL`,
  `HTTPS_PROXY`/`HTTP_PROXY` support.
- PSR-4 autoloaded under `Mockly\Driver\`, requires PHP >=8.1, `ext-curl`, `ext-json`.
- PHPUnit unit tests covering config rendering, binary discovery/install guards, and every
  HTTP API call against a fake local management API.
