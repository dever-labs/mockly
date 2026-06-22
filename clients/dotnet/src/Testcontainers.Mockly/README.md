# Testcontainers.Mockly

Run Mockly in Docker-backed .NET tests with [testcontainers-dotnet](https://dotnet.testcontainers.org/).

`Testcontainers.Mockly` starts `ghcr.io/dever-labs/mockly:latest`, waits for the management API to become ready, and exposes the same mock/scenario/fault controls as the driver client.

## Requirements

- .NET 8+
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
