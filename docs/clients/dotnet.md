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
// Add a mock
await server.AddMockAsync(new Mock
{
    Id = "get-orders",
    Request = new MockRequest
    {
        Method = "GET",
        Path = "/orders",
        Headers = new() { ["Authorization"] = "Bearer *" },
    },
    Response = new MockResponse
    {
        Status = 200,
        Body = """[{"id":1}]""",
        Headers = new() { ["Content-Type"] = "application/json" },
        Delay = "100ms",
    },
});

// Remove a mock
await server.DeleteMockAsync("get-orders");
```

### Scenarios

```csharp
// Activate a pre-configured scenario
await server.ActivateScenarioAsync("payment-fail");

// Deactivate it
await server.DeactivateScenarioAsync("payment-fail");
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
