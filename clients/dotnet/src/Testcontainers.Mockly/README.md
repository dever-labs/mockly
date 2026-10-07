# Testcontainers.Mockly

**[Testcontainers](https://dotnet.testcontainers.org/) module for [Mockly](https://github.com/dever-labs/mockly)** — a single cross-platform binary that mocks HTTP, gRPC, GraphQL, WebSocket, Kafka, AMQP, MQTT, Redis, SMTP, and a dozen other protocols for integration tests, so you don't need a dozen Docker containers to test against a handful of external dependencies.

`Testcontainers.Mockly` starts `ghcr.io/dever-labs/mockly:latest`, waits for the management API to become ready, and exposes the same mock/scenario/fault controls as [`Mockly.Driver`](https://www.nuget.org/packages/Mockly.Driver) — but managed by Docker instead of a downloaded local binary.

## Requirements

- `net8.0`, `net9.0`, or `net10.0` (no third-party runtime dependencies beyond `Testcontainers`)
- Docker

## Install

```sh
dotnet add package Testcontainers.Mockly
```

## Quickstart

```csharp
using System.Net.Http;
using Mockly.Driver.Models;
using Testcontainers.Mockly;

await using var container = new MocklyBuilder().Build();
await container.StartAsync();

await container.AddMockAsync(new Mock(
    "get-user",
    new MockRequest("GET", "/users/1"),
    new MockResponse(
        200,
        """{"id":1,"name":"Alice"}""",
        new Dictionary<string, string> { ["Content-Type"] = "application/json" })));

using var http = new HttpClient();
var response = await http.GetAsync($"{container.GetHttpBaseAddress()}/users/1");
var body = await response.Content.ReadAsStringAsync();

if (!response.IsSuccessStatusCode || body != """{"id":1,"name":"Alice"}""")
{
    throw new InvalidOperationException("Mockly did not return the expected response.");
}
```

## When to use the testcontainers module

Use `Testcontainers.Mockly` when you want Docker to manage Mockly's lifecycle for the test, you do not want to download a local binary, or you want the same containerized setup in local development and CI.

Use `Mockly.Driver` when you prefer running the native Mockly binary directly from the test process.

## Builder API

`MocklyBuilder` is a fluent builder that configures the container before startup.

| Method | Description |
|---|---|
| `new MocklyBuilder()` | Creates a builder with the default image, default config, random host port bindings, and readiness checks. |
| `WithInlineConfig(string yaml)` | Replaces the default `/config/mockly.yaml` with your own YAML. |
| `Build()` | Creates a `MocklyContainer`. Call `StartAsync()` before using it. |

### Custom YAML config

```csharp
var container = new MocklyBuilder()
    .WithInlineConfig("""
    mockly:
      api:
        port: 9091
    protocols:
      http:
        enabled: true
        port: 8090
    """)
    .Build();
```

## Container API

`MocklyContainer` inherits from `DockerContainer` and talks to Mockly through the management API.

### Base addresses

| Method | Description |
|---|---|
| `GetHttpBaseAddress()` | Base URL of the mock HTTP server, for example `http://localhost:32768`. |
| `GetApiBaseAddress()` | Base URL of the management API, for example `http://localhost:32769`. |

### Management methods

| Method | Description |
|---|---|
| `AddMockAsync(Mock mock)` | Register a dynamic HTTP mock. |
| `DeleteMockAsync(string id)` | Delete a mock by ID. |
| `ResetAsync()` | Remove dynamic mocks, deactivate scenarios, and clear faults. |
| `ActivateScenarioAsync(string scenarioId)` | Activate a configured scenario. |
| `DeactivateScenarioAsync(string scenarioId)` | Deactivate a configured scenario. |
| `SetFaultAsync(FaultConfig config)` | Apply a global HTTP fault. |
| `ClearFaultAsync()` | Remove the active fault. |
| `GetLogsAsync()` | Fetch request logs as JSON. |
| `ClearLogsAsync()` | Clear stored request logs. |

## Cleanup

`MocklyContainer` implements `IAsyncDisposable`, so prefer `await using` in tests.

```csharp
await using var container = new MocklyBuilder().Build();
await container.StartAsync();
```

## Links

- [Mockly server docs](https://github.com/dever-labs/mockly#readme) — full protocol list, config reference, scenarios, fault injection, CLI
- [Mockly.Driver](https://www.nuget.org/packages/Mockly.Driver) — companion package for binary-based (non-Docker) usage
- [Changelog](https://github.com/dever-labs/mockly/blob/main/clients/dotnet/CHANGELOG.md)
- [Source](https://github.com/dever-labs/mockly/tree/main/clients/dotnet)
- [Report an issue](https://github.com/dever-labs/mockly/issues)

## License

MIT — see [LICENSE](https://github.com/dever-labs/mockly/blob/main/clients/dotnet/LICENSE).

