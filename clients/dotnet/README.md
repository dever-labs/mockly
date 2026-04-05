# Mockly.Driver

A zero-dependency C# / .NET 6+ client for [Mockly](https://github.com/dever-labs/mockly) — start, configure, and tear down Mockly HTTP mock servers in your integration tests.

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

## API Reference

| Method | Description |
|--------|-------------|
| `MocklyServer.CreateAsync(opts?)` | Install binary (if needed), start server, wait for readiness |
| `MocklyServer.EnsureAsync(opts?)` | Like `CreateAsync`, then calls `ResetAsync()` |
| `server.AddMockAsync(mock)` | Register an HTTP mock (`POST /api/mocks/http`) |
| `server.DeleteMockAsync(id)` | Remove a mock by ID (`DELETE /api/mocks/http/{id}`) |
| `server.ResetAsync()` | Clear all mocks and state (`POST /api/reset`) |
| `server.ActivateScenarioAsync(id)` | Activate a scenario (`POST /api/scenarios/{id}/activate`) |
| `server.DeactivateScenarioAsync(id)` | Deactivate a scenario |
| `server.SetFaultAsync(config)` | Inject faults (`POST /api/fault`) |
| `server.ClearFaultAsync()` | Remove fault config (`DELETE /api/fault`) |
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
| `MOCKLY_VERSION` | Version to download, e.g. `v0.1.0` |
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

## License

MIT — see [LICENSE](LICENSE).
