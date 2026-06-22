using System.Text.Json;
using Mockly.Driver.Models;
using Testcontainers.Mockly;
using Xunit;

namespace Testcontainers.Mockly.Tests;

/// <summary>
/// Integration tests that spin up a real Mockly Docker container.
/// Requires a running Docker daemon.
/// </summary>
/// <remarks>
/// Run with: <c>dotnet test --filter "Category=Integration"</c>
/// Excluded from the default run (no filter) automatically because they are
/// slow and need Docker. CI can opt in with the filter above.
/// </remarks>
[Trait("Category", "Integration")]
public sealed class MocklyContainerIntegrationTests : IAsyncLifetime
{
    private readonly MocklyContainer _container = new MocklyBuilder().Build();

    public async Task InitializeAsync()
    {
        await _container.StartAsync().ConfigureAwait(false);
    }

    public async Task DisposeAsync()
    {
        await _container.DisposeAsync().ConfigureAwait(false);
    }

    [Fact]
    public async Task Container_ApiIsReachable()
    {
        using var http = new HttpClient();
        var response = await http.GetAsync(_container.GetApiBaseAddress() + "/api/protocols");

        Assert.True(response.IsSuccessStatusCode,
            $"GET /api/protocols returned {(int)response.StatusCode}");
    }

    [Fact]
    public async Task AddMock_ThenHitEndpoint_ReturnsExpectedResponse()
    {
        var mock = new Mock(
            "hello-mock",
            new MockRequest("GET", "/hello"),
            new MockResponse(200, "world"));

        await _container.AddMockAsync(mock);

        using var http = new HttpClient();
        var response = await http.GetAsync(_container.GetHttpBaseAddress() + "/hello");
        var body = await response.Content.ReadAsStringAsync();

        Assert.Equal(200, (int)response.StatusCode);
        Assert.Equal("world", body);
    }

    [Fact]
    public async Task Reset_ClearsMocks()
    {
        var mock = new Mock(
            "reset-mock",
            new MockRequest("GET", "/reset-me"),
            new MockResponse(200, "before reset"));
        await _container.AddMockAsync(mock);
        await _container.ResetAsync();

        using var http = new HttpClient();
        var response = await http.GetAsync(_container.GetHttpBaseAddress() + "/reset-me");

        Assert.NotEqual(200, (int)response.StatusCode);
    }

    [Fact]
    public async Task GetLogs_ReturnsEntriesAfterRequest()
    {
        using var http = new HttpClient();
        await http.GetAsync(_container.GetHttpBaseAddress() + "/log-probe");

        var logs = await _container.GetLogsAsync();

        Assert.False(string.IsNullOrWhiteSpace(logs), "GetLogs should return non-empty JSON");
        // Verify it parses as JSON
        using var doc = JsonDocument.Parse(logs);
        Assert.NotNull(doc);
    }
}
