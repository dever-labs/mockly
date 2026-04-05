# mocklydriver — Go client for Mockly

`mocklydriver` is a zero-dependency Go package for starting and controlling a [Mockly](https://github.com/dever-labs/mockly) HTTP mock server from within tests.

## Installation

```bash
go get github.com/dever-labs/mockly/clients/go
```

## Quick start

```go
package mypackage_test

import (
    "testing"

    mocklydriver "github.com/dever-labs/mockly/clients/go"
)

var srv *mocklydriver.Server

func TestMain(m *testing.M) {
    var err error

    // Ensure downloads the binary if needed, then starts the server.
    srv, err = mocklydriver.Ensure(
        mocklydriver.Options{},
        mocklydriver.InstallOptions{BinDir: "./bin"},
    )
    if err != nil {
        panic(err)
    }
    defer srv.Stop()

    m.Run()
}

func TestGetUsers(t *testing.T) {
    if err := srv.AddMock(mocklydriver.Mock{
        ID: "get-users",
        Request:  mocklydriver.MockRequest{Method: "GET", Path: "/users"},
        Response: mocklydriver.MockResponse{Status: 200, Body: `[{"id":1}]`,
            Headers: map[string]string{"Content-Type": "application/json"}},
    }); err != nil {
        t.Fatal(err)
    }
    defer srv.DeleteMock("get-users")

    // Call srv.HTTPBase + "/users" from your HTTP client under test.
    _ = srv.HTTPBase
}
```

## Installing the binary

`mocklydriver.Install()` downloads the binary automatically unless `MOCKLY_NO_INSTALL` is set. You can also pre-stage the binary:

```bash
# Point at a pre-staged binary (skips all download logic)
export MOCKLY_BINARY_PATH=/usr/local/bin/mockly
```

Or download it manually from the [releases page](https://github.com/dever-labs/mockly/releases).

## API reference

### Top-level functions

| Function | Description |
|---|---|
| `Create(opts Options) (*Server, error)` | Find binary, allocate ports, start process. Retries up to 3× on port conflict. |
| `Ensure(opts Options, installOpts InstallOptions) (*Server, error)` | `Install()` then `Create()`. |
| `Install(opts InstallOptions) (string, error)` | Download binary for current platform. Returns path. |
| `GetBinaryPath(binDir string) string` | Locate existing binary. Returns `""` if not found. |

### `Server` methods

| Method | Description |
|---|---|
| `Stop() error` | Kill the process and wait for exit. |
| `AddMock(mock Mock) error` | `POST /api/mocks/http` — register a mock (201). |
| `DeleteMock(id string) error` | `DELETE /api/mocks/http/{id}` — remove a mock (204). |
| `Reset() error` | `POST /api/reset` — remove all dynamic mocks, deactivate scenarios, clear faults (200). |
| `ActivateScenario(id string) error` | `POST /api/scenarios/{id}/activate` (200). |
| `DeactivateScenario(id string) error` | `POST /api/scenarios/{id}/deactivate` (200). |
| `SetFault(cfg FaultConfig) error` | `POST /api/fault` — enable fault injection (200). |
| `ClearFault() error` | `DELETE /api/fault` — disable fault injection (200). |

### `Server` fields

| Field | Description |
|---|---|
| `HTTPPort int` | Port the mock HTTP server listens on. |
| `APIPort int` | Port the management API listens on. |
| `HTTPBase string` | `http://127.0.0.1:<HTTPPort>` — base URL for mock traffic. |
| `APIBase string` | `http://127.0.0.1:<APIPort>` — base URL for management calls. |

### Types

```go
type Mock struct {
    ID       string
    Request  MockRequest
    Response MockResponse
}

type MockRequest struct {
    Method  string
    Path    string
    Headers map[string]string
}

type MockResponse struct {
    Status  int
    Body    string
    Headers map[string]string
    Delay   string // e.g. "50ms"
}

type FaultConfig struct {
    Enabled        bool
    Delay          string
    StatusOverride *int
    ErrorRate      float64 // 0.0–1.0
}

type Options struct {
    Scenarios []Scenario
}

type InstallOptions struct {
    Version string // default: "v0.1.0"
    BaseURL string // default: GitHub releases
    BinDir  string // default: "./bin"
    Force   bool   // re-download even if binary exists
}
```

## Environment variables

| Variable | Description |
|---|---|
| `MOCKLY_BINARY_PATH` | Absolute path to a pre-staged binary. Skips all download logic. If set and the file is missing, an error is returned. |
| `MOCKLY_NO_INSTALL` | If set (any value), `Install()` returns an error instead of downloading. Use in air-gapped environments together with `MOCKLY_BINARY_PATH`. |
| `MOCKLY_VERSION` | Override the default binary version (`v0.1.0`). |
| `MOCKLY_DOWNLOAD_BASE_URL` | Override the download base URL. Useful for Artifactory or internal mirrors. |
| `HTTPS_PROXY` / `HTTP_PROXY` | Respected automatically by Go's `net/http`. |
| `NO_PROXY` | Respected automatically by Go's `net/http`. |

## Proxy / Artifactory / air-gap

### Corporate proxy

Set `HTTPS_PROXY` or `HTTP_PROXY` before running tests — Go's `net/http` honours these automatically:

```bash
HTTPS_PROXY=http://proxy.corp.example.com:3128 go test ./...
```

### Artifactory or custom mirror

Host the Mockly release assets in your artifact registry and override the base URL:

```bash
MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.corp.example.com/mockly/releases/download go test ./...
```

The final download URL is: `<MOCKLY_DOWNLOAD_BASE_URL>/<version>/<asset>`.

### Air-gapped environment

Pre-download the binary and point directly at it:

```bash
export MOCKLY_BINARY_PATH=/opt/tools/mockly
export MOCKLY_NO_INSTALL=1   # belt-and-suspenders: prevent any download attempts
go test ./...
```

## Supported platforms

| OS | Architecture | Asset |
|---|---|---|
| Linux | x64 | `mockly-linux-amd64` |
| Linux | arm64 | `mockly-linux-arm64` |
| macOS | x64 | `mockly-darwin-amd64` |
| macOS | arm64 (Apple Silicon) | `mockly-darwin-arm64` |
| Windows | x64 | `mockly-windows-amd64.exe` |

## License

Apache-2.0 — see [LICENSE](../../LICENSE).
