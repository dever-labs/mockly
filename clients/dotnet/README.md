# Mockly.Driver

**C# client for [Mockly](https://github.com/dever-labs/mockly)** — a single cross-platform binary that mocks HTTP, gRPC, GraphQL, WebSocket, Kafka, AMQP, MQTT, Redis, SMTP, and a dozen other protocols for integration tests, so you don't need a dozen Docker containers to test against a handful of external dependencies.

`Mockly.Driver` downloads the right native `mockly` binary for your OS/architecture automatically (no separate install step), starts it as a child process from your test, and gives you a fully-typed async API to register HTTP mocks, inspect recorded calls, run scenarios, and inject faults (latency, error rates, bad status codes).

Prefer Docker instead of a downloaded binary? Use [`Testcontainers.Mockly`](https://www.nuget.org/packages/Testcontainers.Mockly) — same API, container-based lifecycle.

## Supported frameworks

`net8.0`, `net9.0`, `net10.0` (and `net11.0` once released). No third-party runtime dependencies.

## Installation

```shell
dotnet add package Mockly.Driver
```

## Quick Start

```csharp
using Mockly.Driver;
using Mockly.Driver.Models;

public class MyIntegrationTests : IAsyncLifetime
{
    private MocklyServer _server = null!;

    public async Task InitializeAsync()
    {
        _server = await MocklyServer.CreateAsync();
    }

    public async Task DisposeAsync()
    {
        await _server.DisposeAsync();
    }

    [Fact]
    public async Task MyTest()
    {
        await _server.AddMockAsync(new Mock(
            Id: "get-users",
            Request: new MockRequest("GET", "/users"),
            Response: new MockResponse(200, Body: "[{\"id\":1}]",
                Headers: new() { ["Content-Type"] = "application/json" })
        ));

        using var http = new HttpClient { BaseAddress = new Uri(_server.HttpBase) };
        var resp = await http.GetAsync("/users");
        var body = await resp.Content.ReadAsStringAsync();
        Assert.Equal(200, (int)resp.StatusCode);
        Assert.Contains("\"id\":1", body);

        await _server.DeleteMockAsync("get-users");
    }
}
```

### With `await using`

```csharp
await using var server = await MocklyServer.CreateAsync();
await server.AddMockAsync(new Mock("ping", new MockRequest("GET", "/ping"), new MockResponse(200)));
```

`CreateAsync`/`EnsureAsync` install the binary (if needed), start the server, wait for readiness, and retry automatically up to 3 times on ephemeral port conflicts.

## Asserting on recorded calls

Every request matched by a mock is recorded. Use this to assert your code under test actually called the mock, and how many times.

```csharp
await server.AddMockAsync(new Mock("get-user", new MockRequest("GET", "/users/1"), new MockResponse(200)));

// ... exercise the code under test ...

var calls = await server.WaitForCallsAsync("get-user", count: 1, timeout: TimeSpan.FromSeconds(5));
Assert.Equal(1, calls.Count);
```

## Scenarios

Scenarios group a set of mock-response overrides ("patches") behind a single named switch, so a test can flip a dependency into a failure/edge-case mode without redefining mocks.

```csharp
var server = await MocklyServer.CreateAsync(new MocklyServerOptions(
    Scenarios: [
        new Scenario(
            Id: "payment-declined",
            Name: "Payment Declined",
            Patches: [
                new ScenarioPatch(MockId: "charge-card", Status: 402, Body: "{\"error\":\"declined\"}")
            ])
    ]));

await server.ActivateScenarioAsync("payment-declined");
// requests matching "charge-card" now return 402 until deactivated
await server.DeactivateScenarioAsync("payment-declined");
```

## Fault injection

Simulate latency, partial outages, or forced error rates on the HTTP mock server without changing individual mocks:

```csharp
await server.SetFaultAsync(new FaultConfig(Enabled: true, Delay: "200ms", ErrorRate: 0.5));
// ~50% of requests now get a 500 and/or a 200ms added delay
await server.ClearFaultAsync();
```

## API Reference

| Method | Description |
|--------|-------------|
| `MocklyServer.CreateAsync(opts?)` | Install binary (if needed), start server, wait for readiness |
| `MocklyServer.EnsureAsync(opts?)` | Like `CreateAsync`, then calls `ResetAsync()` |
| `server.AddMockAsync(mock)` | Register an HTTP mock (`POST /api/mocks/http`) |
| `server.ListMocksAsync()` | List all registered HTTP mocks |
| `server.UpdateMockAsync(id, mock)` | Replace a mock by ID |
| `server.PatchMockAsync(id, patch)` | Partially update a mock's response |
| `server.DeleteMockAsync(id)` | Remove a mock by ID (`DELETE /api/mocks/http/{id}`) |
| `server.ResetAsync()` | Clear all mocks, scenarios, and faults |
| `server.ListScenariosAsync()` / `CreateScenarioAsync(scenario)` / `GetScenarioAsync(id)` / `UpdateScenarioAsync(id, scenario)` / `DeleteScenarioAsync(id)` | Scenario CRUD |
| `server.ActivateScenarioAsync(id)` / `DeactivateScenarioAsync(id)` | Toggle a scenario |
| `server.ListActiveScenariosAsync()` | List currently active scenarios |
| `server.SetFaultAsync(config)` | Inject faults (`POST /api/fault/http`) |
| `server.ClearFaultAsync()` | Remove fault config (`DELETE /api/fault`) |
| `server.GetCallsAsync(mockId)` / `ClearCallsAsync(mockId)` / `ClearAllCallsAsync()` | Inspect/clear recorded calls |
| `server.WaitForCallsAsync(mockId, count, timeout)` | Poll until a mock has been called `count` times |
| `server.GetStateAsync()` / `SetStateAsync(kvMap)` / `DeleteStateAsync(key)` | Manage the server's key-value state store |
| `server.GetLogsAsync(matchedId?)` / `ClearLogsAsync()` / `GetLogsCountAsync(matchedId?)` | Request log retrieval and cleanup |
| `server.StopAsync()` | Stop and clean up |

### Properties

| Property | Description |
|----------|-------------|
| `server.HttpPort` | Port the mock HTTP server is listening on |
| `server.ApiPort` | Port the management API is listening on |
| `server.HttpBase` | `http://127.0.0.1:<HttpPort>` |
| `server.ApiBase` | `http://127.0.0.1:<ApiPort>` |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MOCKLY_BINARY_PATH` | Absolute path to a pre-staged Mockly binary (skips download) |
| `MOCKLY_VERSION` | Version to download, e.g. `v0.16.0`<!-- x-release-please-version --> |
| `MOCKLY_DOWNLOAD_BASE_URL` | Base URL override for binary downloads (Artifactory / mirror) |
| `MOCKLY_NO_INSTALL` | Set to any value to throw instead of downloading |
| `HTTPS_PROXY` / `HTTP_PROXY` | Standard proxy variables, honoured automatically by `HttpClient` |

## Air-Gap / Artifactory

Pre-stage the binary and set `MOCKLY_BINARY_PATH`:

```shell
# CI pipeline step
MOCKLY_BINARY_PATH=/opt/mockly/mockly
```

Or mirror the releases and set `MOCKLY_DOWNLOAD_BASE_URL`:

```shell
MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.corp.com/mockly/releases/download
```

## Proxy Support

`HttpClient` picks up `HTTPS_PROXY` / `HTTP_PROXY` automatically when using `HttpClientHandler`. No extra configuration needed.

## Links

- [Mockly server docs](https://github.com/dever-labs/mockly#readme) — full protocol list, config reference, scenarios, fault injection, CLI
- [.NET client docs](https://github.com/dever-labs/mockly/blob/main/docs/clients/dotnet.md) — extended usage guide
- [Changelog](https://github.com/dever-labs/mockly/blob/main/clients/dotnet/CHANGELOG.md)
- [Source](https://github.com/dever-labs/mockly/tree/main/clients/dotnet)
- [Report an issue](https://github.com/dever-labs/mockly/issues)

## License

MIT — see [LICENSE](https://github.com/dever-labs/mockly/blob/main/clients/dotnet/LICENSE).
