# Mockly — .NET Client

The .NET client starts, controls, and stops a Mockly process from your C# tests.

## Install

```sh
dotnet add package Mockly.Driver
```

The package bundles the native binary for your platform — no separate install step required.

## Quickstart

```csharp
using Mockly.Driver;
using Mockly.Driver.Models;

await using var server = await MocklyServer.CreateAsync();

await server.AddMockAsync(new Mock
{
    Id = "get-user",
    Request = new MockRequest { Method = "GET", Path = "/users/1" },
    Response = new MockResponse
    {
        Status = 200,
        Body = """{"id":1,"name":"Alice"}""",
        Headers = new() { ["Content-Type"] = "application/json" },
    },
});

// Point your service under test at server.HttpBase
// e.g. "http://127.0.0.1:45678"
```

## Factory methods

| Method | Description |
|---|---|
| `MocklyServer.CreateAsync(opts?)` | Starts the server. Installs the binary automatically via the bundled MSBuild targets. **Recommended for most cases.** |
| `MocklyServer.EnsureAsync(opts?)` | Like `CreateAsync`, but also calls `ResetAsync()` immediately — useful for test setup. |

Both implement `IAsyncDisposable` — use `await using` for automatic cleanup.

Both retry up to 3 times on ephemeral port conflicts.

## Configuration

```csharp
var server = await MocklyServer.CreateAsync(new MocklyServerOptions
{
    Scenarios =
    [
        new Scenario
        {
            Id = "payment-fail",
            Name = "Payment Failure",
            Patches =
            [
                new ScenarioPatch
                {
                    MockId = "charge",
                    Status = 503,
                    Body = """{"error":"unavailable"}""",
                },
            ],
        },
    ],
});
```

## API reference

### Mocks

```csharp
using System.Collections.Generic;
using Mockly.Driver.Models;

// Add a mock
await server.AddMockAsync(new Mock(
    "get-orders",
    new MockRequest(
        "GET",
        "/orders",
        new Dictionary<string, string> { ["Authorization"] = "Bearer *" }),
    new MockResponse(
        200,
        """[{"id":1}]""",
        new Dictionary<string, string> { ["Content-Type"] = "application/json" },
        "100ms")));

// Inspect the currently registered mocks
var mocks = await server.ListMocksAsync();

// Replace a mock definition
var updated = await server.UpdateMockAsync("get-orders", new Mock(
    "get-orders",
    new MockRequest("GET", "/orders"),
    new MockResponse(
        200,
        """[{"id":1},{"id":2}]""",
        new Dictionary<string, string> { ["Content-Type"] = "application/json" })));

// Patch only the response fields you want to change
var patched = await server.PatchMockAsync(
    "get-orders",
    new MockResponsePatch(Status: 201, Body: "[]", Headers: new Dictionary<string, string>
    {
        ["X-Mock-Version"] = "v2",
    }, Delay: "250ms"));

// Remove a mock
await server.DeleteMockAsync("get-orders");
```

### Scenarios

```csharp
using System;
using System.Collections.Generic;
using System.Linq;
using Mockly.Driver.Models;

var createdScenario = await server.CreateScenarioAsync(new Scenario(
    "slow-checkout",
    "Slow checkout",
    new[]
    {
        new ScenarioPatch("charge", Status: 503, Delay: "750ms"),
    },
    "Used for retry-path tests"));

var scenarios = await server.ListScenariosAsync();
var loadedScenario = await server.GetScenarioAsync("slow-checkout");

var updatedScenario = await server.UpdateScenarioAsync(
    "slow-checkout",
    loadedScenario with { Name = "Slow checkout v2" });

// Activate a scenario before exercising your service
await server.ActivateScenarioAsync("slow-checkout");
var activeScenarios = await server.ListActiveScenariosAsync();
Console.WriteLine(string.Join(", ", activeScenarios.Active));

// Deactivate or delete it when you're done
await server.DeactivateScenarioAsync("slow-checkout");
await server.DeleteScenarioAsync("slow-checkout");
```

### Call verification

```csharp
using System;

var summary = await server.WaitForCallsAsync(
    "get-orders",
    count: 2,
    timeout: TimeSpan.FromSeconds(5));

if (summary.Count != 2)
{
    throw new InvalidOperationException($"Expected 2 calls, got {summary.Count}");
}

var latestCalls = await server.GetCallsAsync("get-orders");
Console.WriteLine(latestCalls.Calls[0].Path);

await server.ClearCallsAsync("get-orders");
await server.ClearAllCallsAsync();
```

### State

```csharp
using System.Collections.Generic;

var state = await server.GetStateAsync();
Console.WriteLine(state.GetValueOrDefault("order-status"));

var updatedState = await server.SetStateAsync(new Dictionary<string, string>
{
    ["order-status"] = "pending",
    ["retry-count"] = "1",
});
Console.WriteLine(updatedState["retry-count"]);

await server.DeleteStateAsync("retry-count");
```

### Logs

```csharp
using System.Linq;

var allLogs = await server.GetLogsAsync();
var matchedLogs = await server.GetLogsAsync("get-orders");

var totalLogs = await server.GetLogsCountAsync();
var matchedCount = await server.GetLogsCountAsync("get-orders");
Console.WriteLine($"{totalLogs} total / {matchedCount} matched");
Console.WriteLine(allLogs.FirstOrDefault()?.Path);
Console.WriteLine(matchedLogs.FirstOrDefault()?.MatchedId);

await server.ClearLogsAsync();
```

### Fault injection

```csharp
// Add latency and override status codes on all requests
await server.SetFaultAsync(new FaultConfig
{
    Enabled = true,
    Delay = "500ms",
    StatusOverride = 503,
    ErrorRate = 0.5f, // 50% of requests
});

// Remove the fault
await server.ClearFaultAsync();
```

### Reset and stop

```csharp
// Reset all dynamic mocks, active scenarios, and faults; keeps startup config
await server.ResetAsync();

// Explicit stop (or use IAsyncDisposable / await using)
await server.StopAsync();
```

## Integration with xUnit

```csharp
// SharedMocklyFixture.cs
using Mockly.Driver;

public sealed class MocklyFixture : IAsyncLifetime
{
    public MocklyServer Server { get; private set; } = null!;

    public async Task InitializeAsync()
        => Server = await MocklyServer.CreateAsync();

    public async Task DisposeAsync()
        => await Server.StopAsync();
}

// PaymentServiceTests.cs
public class PaymentServiceTests : IClassFixture<MocklyFixture>, IAsyncLifetime
{
    private readonly MocklyServer _server;

    public PaymentServiceTests(MocklyFixture fixture)
        => _server = fixture.Server;

    public Task InitializeAsync() => _server.ResetAsync(); // isolate each test

    public Task DisposeAsync() => Task.CompletedTask;

    [Fact]
    public async Task ReturnsUserFromMock()
    {
        await _server.AddMockAsync(new Mock
        {
            Id = "get-user",
            Request  = new MockRequest { Method = "GET", Path = "/users/1" },
            Response = new MockResponse { Status = 200, Body = """{"id":1}""" },
        });

        // ... call your service using _server.HttpBase, assert response ...
    }

    [Fact]
    public async Task Handles503ViaScenario()
    {
        await _server.AddMockAsync(new Mock
        {
            Id = "charge",
            Request  = new MockRequest { Method = "POST", Path = "/charge" },
            Response = new MockResponse { Status = 200, Body = """{"ok":true}""" },
        });

        await _server.ActivateScenarioAsync("payment-fail");

        // ... assert your service handles 503 gracefully ...
    }
}
```

## Integration with NUnit

```csharp
[TestFixture]
public class OrderServiceTests
{
    private MocklyServer _server = null!;

    [OneTimeSetUp]
    public async Task StartMockly()
        => _server = await MocklyServer.CreateAsync();

    [OneTimeTearDown]
    public async Task StopMockly()
        => await _server.StopAsync();

    [SetUp]
    public Task Reset() => _server.ResetAsync();

    [Test]
    public async Task ReturnsOrders()
    {
        await _server.AddMockAsync(/* ... */);
        // ...
    }
}
```

## Server properties

| Property | Description |
|---|---|
| `server.HttpBase` | Base URL of the mock HTTP server, e.g. `http://127.0.0.1:45123` |
| `server.ApiBase` | Base URL of the management API, e.g. `http://127.0.0.1:45124` |
| `server.HttpPort` | Numeric HTTP port |
| `server.ApiPort` | Numeric API port |

## Testcontainers

Mockly also ships a Docker-backed testcontainers module for .NET: `Testcontainers.Mockly`.

Use it instead of the driver when you want Docker-managed lifecycle, no local binary download, and the same container image in local tests and CI.

### Install

```sh
dotnet add package Testcontainers.Mockly
```

### Example

```csharp
using System.Net.Http;
using Mockly.Driver.Models;
using Testcontainers.Mockly;
using Xunit;

public class PaymentTests
{
    [Fact]
    public async Task ReturnsUserFromContainer()
    {
        await using var container = new MocklyBuilder().Build();
        await container.StartAsync();

        await container.AddMockAsync(new Mock(
            "get-user",
            new MockRequest("GET", "/users/1"),
            new MockResponse(200, """{"id":1}""")));

        using var http = new HttpClient();
        var body = await http.GetStringAsync($"{container.GetHttpBaseAddress()}/users/1");

        Assert.Equal("""{"id":1}""", body);
    }
}
```

### Key API

- `MocklyBuilder.WithInlineConfig(yaml)` to replace the container config
- `MocklyContainer.GetHttpBaseAddress()` / `GetApiBaseAddress()` for base URLs
- `AddMockAsync`, `DeleteMockAsync`, `ResetAsync`
- `ActivateScenarioAsync`, `DeactivateScenarioAsync`
- `SetFaultAsync`, `ClearFaultAsync`

### Requirements

- .NET 8+
- Docker

See `clients/dotnet/src/Testcontainers.Mockly/README.md` for the full module reference.
