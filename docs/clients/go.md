# Mockly — Go Client

The Go client starts, controls, and stops a Mockly process from within your Go tests.

## Install

```sh
go get github.com/dever-labs/mockly/clients/go
```

## Quickstart

```go
package mypackage_test

import (
    "testing"

    mocklydriver "github.com/dever-labs/mockly/clients/go"
)

func TestMyService(t *testing.T) {
    // Install binary if needed, then start the server
    server, err := mocklydriver.Ensure(
        mocklydriver.Options{},
        mocklydriver.InstallOptions{},
    )
    if err != nil {
        t.Fatalf("start mockly: %v", err)
    }
    defer server.Stop()

    err = server.AddMock(mocklydriver.Mock{
        ID: "get-user",
        Request: mocklydriver.Request{Method: "GET", Path: "/users/1"},
        Response: mocklydriver.Response{
            Status: 200,
            Body:   `{"id":1,"name":"Alice"}`,
            Headers: map[string]string{"Content-Type": "application/json"},
        },
    })
    if err != nil {
        t.Fatalf("add mock: %v", err)
    }

    // Point your service under test at server.HTTPBase, then assert responses
    _ = server.HTTPBase // e.g. "http://127.0.0.1:45678"
}
```

## Factory methods

| Function | Description |
|---|---|
| `mocklydriver.Ensure(opts, installOpts)` | Downloads the binary if not present, then starts the server. **Recommended for most cases.** |
| `mocklydriver.Create(opts)` | Starts the server using an already-installed binary. Fails immediately if the binary cannot be found. |

Both retry up to 3 times on ephemeral port conflicts.

## Configuration

```go
// With pre-loaded scenarios
server, err := mocklydriver.Ensure(
    mocklydriver.Options{
        Scenarios: []mocklydriver.Scenario{
            {
                ID:   "payment-fail",
                Name: "Payment Failure",
                Patches: []mocklydriver.ScenarioPatch{
                    {MockID: "charge", Status: ptr(503), Body: ptr(`{"error":"unavailable"}`)},
                },
            },
        },
    },
    mocklydriver.InstallOptions{},
)
```

## API reference

### Mocks

```go
// Add a mock
server.AddMock(mocklydriver.Mock{
    ID: "get-orders",
    Request: mocklydriver.Request{
        Method: "GET",
        Path:   "/orders",
        Headers: map[string]string{"Authorization": "Bearer *"},
    },
    Response: mocklydriver.Response{
        Status:  200,
        Body:    `[{"id":1}]`,
        Headers: map[string]string{"Content-Type": "application/json"},
        Delay:   "100ms",
    },
})

// Remove a mock
server.DeleteMock("get-orders")
```

### Scenarios

```go
// Activate a pre-configured scenario
server.ActivateScenario("payment-fail")

// Deactivate it
server.DeactivateScenario("payment-fail")
```

### Fault injection

```go
// Add latency and override status codes on all requests
server.SetFault(mocklydriver.FaultConfig{
    Enabled:        true,
    Delay:          "500ms",
    StatusOverride: 503,
    ErrorRate:      0.5, // 50% of requests
})

// Remove the fault
server.ClearFault()
```

### Reset and stop

```go
// Reset all dynamic mocks, active scenarios, and faults; keeps startup config
server.Reset()

// Kill the process
server.Stop()
```

## Integration with `testing.T`

### TestMain (shared server for a package)

```go
package mypackage_test

import (
    "os"
    "testing"

    mocklydriver "github.com/dever-labs/mockly/clients/go"
)

var mockly *mocklydriver.Server

func TestMain(m *testing.M) {
    var err error
    mockly, err = mocklydriver.Ensure(mocklydriver.Options{}, mocklydriver.InstallOptions{})
    if err != nil {
        panic(err)
    }
    code := m.Run()
    mockly.Stop()
    os.Exit(code)
}

func TestGetUser(t *testing.T) {
    defer mockly.Reset()

    mockly.AddMock(mocklydriver.Mock{
        ID:       "get-user",
        Request:  mocklydriver.Request{Method: "GET", Path: "/users/1"},
        Response: mocklydriver.Response{Status: 200, Body: `{"id":1}`},
    })

    // ... test your service ...
}
```

### Per-test server

```go
func TestIsolated(t *testing.T) {
    server, err := mocklydriver.Ensure(mocklydriver.Options{}, mocklydriver.InstallOptions{})
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { server.Stop() })

    // ... test ...
}
```

## Server properties

| Field | Description |
|---|---|
| `server.HTTPBase` | Base URL of the mock HTTP server, e.g. `http://127.0.0.1:45123` |
| `server.APIBase` | Base URL of the management API, e.g. `http://127.0.0.1:45124` |
| `server.HTTPPort` | Numeric HTTP port |
| `server.APIPort` | Numeric API port |
