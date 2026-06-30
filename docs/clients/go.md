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
statusCreated := 201
emptyBody := `[]`
retryDelay := "250ms"

// Add a mock
err = server.AddMock(mocklydriver.Mock{
    ID: "get-orders",
    Request: mocklydriver.MockRequest{
        Method:  "GET",
        Path:    "/orders",
        Headers: map[string]string{"Authorization": "Bearer *"},
    },
    Response: mocklydriver.MockResponse{
        Status:  200,
        Body:    `[{"id":1}]`,
        Headers: map[string]string{"Content-Type": "application/json"},
        Delay:   "100ms",
    },
})

// Inspect the currently registered mocks
mocks, err := server.ListMocks()
if err != nil {
    t.Fatalf("list mocks: %v", err)
}
_ = mocks

// Replace a mock definition
updated, err := server.UpdateMock("get-orders", mocklydriver.Mock{
    ID: "get-orders",
    Request: mocklydriver.MockRequest{Method: "GET", Path: "/orders"},
    Response: mocklydriver.MockResponse{
        Status:  200,
        Body:    `[{"id":1},{"id":2}]`,
        Headers: map[string]string{"Content-Type": "application/json"},
    },
})
if err != nil {
    t.Fatalf("update mock: %v", err)
}
_ = updated

// Patch only the response fields you want to change
patched, err := server.PatchMock("get-orders", mocklydriver.MockResponsePatch{
    Status: &statusCreated,
    Body:   &emptyBody,
    Headers: map[string]string{
        "X-Mock-Version": "v2",
    },
    Delay: &retryDelay,
})
if err != nil {
    t.Fatalf("patch mock: %v", err)
}
_ = patched

// Remove a mock
err = server.DeleteMock("get-orders")
```

### Scenarios

```go
scenarioStatus := 503
scenarioDelay := "750ms"

createdScenario, err := server.CreateScenario(mocklydriver.Scenario{
    ID:          "slow-checkout",
    Name:        "Slow checkout",
    Description: "Used for retry-path tests",
    Patches: []mocklydriver.ScenarioPatch{
        {
            MockID: "charge",
            Status: &scenarioStatus,
            Delay:  &scenarioDelay,
        },
    },
})
if err != nil {
    t.Fatalf("create scenario: %v", err)
}
_ = createdScenario

scenarios, err := server.ListScenarios()
if err != nil {
    t.Fatalf("list scenarios: %v", err)
}
_ = scenarios

loadedScenario, err := server.GetScenario("slow-checkout")
if err != nil {
    t.Fatalf("get scenario: %v", err)
}

loadedScenario.Name = "Slow checkout v2"
updatedScenario, err := server.UpdateScenario("slow-checkout", *loadedScenario)
if err != nil {
    t.Fatalf("update scenario: %v", err)
}
_ = updatedScenario

// Activate a scenario before exercising your service
err = server.ActivateScenario("slow-checkout")
if err != nil {
    t.Fatalf("activate scenario: %v", err)
}

activeScenarios, err := server.ListActiveScenarios()
if err != nil {
    t.Fatalf("list active scenarios: %v", err)
}
_ = activeScenarios.Active

// Deactivate or delete it when you're done
err = server.DeactivateScenario("slow-checkout")
if err != nil {
    t.Fatalf("deactivate scenario: %v", err)
}
err = server.DeleteScenario("slow-checkout")
```

### Call verification

```go
summary, err := server.WaitForCalls("get-orders", 2, 5)
if err != nil {
    t.Fatalf("wait for calls: %v", err)
}
if summary.Count != 2 {
    t.Fatalf("expected 2 calls, got %d", summary.Count)
}

latestCalls, err := server.GetCalls("get-orders")
if err != nil {
    t.Fatalf("get calls: %v", err)
}
_ = latestCalls.Calls

err = server.ClearCalls("get-orders")
if err != nil {
    t.Fatalf("clear calls: %v", err)
}
err = server.ClearAllCalls()
```

### State

```go
state, err := server.GetState()
if err != nil {
    t.Fatalf("get state: %v", err)
}
_ = state["order-status"]

updatedState, err := server.SetState(map[string]string{
    "order-status": "pending",
    "retry-count":  "1",
})
if err != nil {
    t.Fatalf("set state: %v", err)
}
_ = updatedState["retry-count"]

err = server.DeleteState("retry-count")
```

### Logs

```go
allLogs, err := server.GetLogs("")
if err != nil {
    t.Fatalf("get logs: %v", err)
}
matchedLogs, err := server.GetLogs("get-orders")
if err != nil {
    t.Fatalf("get filtered logs: %v", err)
}
_ = allLogs
_ = matchedLogs

totalLogs, err := server.GetLogsCount("")
if err != nil {
    t.Fatalf("count logs: %v", err)
}
matchedCount, err := server.GetLogsCount("get-orders")
if err != nil {
    t.Fatalf("count filtered logs: %v", err)
}
_ = totalLogs
_ = matchedCount

err = server.ClearLogs()
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

## Testcontainers

Mockly also ships a Docker-backed Go testcontainers module in the same repository: `github.com/dever-labs/mockly/clients/go/testcontainers`.

Use it instead of the driver when you want Docker-managed lifecycle, no local binary download, and the same container image in local tests and CI.

### Install

```sh
go get github.com/dever-labs/mockly/clients/go/testcontainers
go get github.com/testcontainers/testcontainers-go
```

### Example

```go
package mypackage_test

import (
    "context"
    "io"
    "net/http"
    "testing"

    mocklydriver "github.com/dever-labs/mockly/clients/go"
    testcontainersmockly "github.com/dever-labs/mockly/clients/go/testcontainers"
)

func TestReturnsUserFromContainer(t *testing.T) {
    ctx := context.Background()
    container, err := testcontainersmockly.Run(ctx)
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = container.Terminate(context.Background()) })

    if err := container.AddMock(ctx, mocklydriver.Mock{
        ID: "get-user",
        Request: mocklydriver.MockRequest{Method: http.MethodGet, Path: "/users/1"},
        Response: mocklydriver.MockResponse{Status: 200, Body: `{"id":1}`},
    }); err != nil {
        t.Fatal(err)
    }

    httpBase, _ := container.HTTPBase(ctx)
    resp, err := http.Get(httpBase + "/users/1")
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)

    if got := string(body); resp.StatusCode != 200 || got != `{"id":1}` {
        t.Fatalf("unexpected response: status=%d body=%s", resp.StatusCode, got)
    }
}
```

### Key API

- `Run(ctx, opts...)` to start the container
- `WithImage(image)` and `WithInlineConfig(yaml)` startup options
- `HTTPBase(ctx)` / `APIBase(ctx)`
- `AddMock`, `DeleteMock`, `Reset`
- `ActivateScenario`, `DeactivateScenario`
- `SetFault`, `ClearFault`

### Requirements

- Go 1.21+
- Docker
