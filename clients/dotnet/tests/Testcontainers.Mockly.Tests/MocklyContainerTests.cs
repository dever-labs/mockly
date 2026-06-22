using System.Reflection;
using System.Text;
using System.Text.Json;
using Mockly.Driver.Models;
using Testcontainers.Mockly;
using Xunit;

namespace Testcontainers.Mockly.Tests;

public sealed class MocklyContainerTests
{
    [Fact]
    public void MocklyBuilder_DefaultImage_IsCorrect()
    {
        Assert.Equal("ghcr.io/dever-labs/mockly:latest", MocklyBuilder.DefaultImage);
    }

    [Fact]
    public void MocklyBuilder_DefaultPorts_AreCorrect()
    {
        Assert.Equal((ushort)8090, MocklyBuilder.HttpPort);
        Assert.Equal((ushort)9091, MocklyBuilder.ApiPort);
    }

    [Fact]
    public async Task MocklyBuilder_WithInlineConfig_SetsConfig()
    {
        const string yaml = "mockly:\n  api:\n    port: 9091";

        var builder = new MocklyBuilder()
            .WithInlineConfig(yaml);

        var configuration = (MocklyConfiguration)typeof(MocklyBuilder)
            .GetProperty("DockerResourceConfiguration", BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.DeclaredOnly)!
            .GetValue(builder)!;

        Assert.Equal(yaml, configuration.InlineConfig);

        var resourceMappings = configuration.ResourceMappings
            .Where(resourceMapping => resourceMapping.Target == MocklyBuilder.ContainerConfigPath)
            .ToArray();

        Assert.NotEmpty(resourceMappings);
        // The last mapping for the config path wins at container startup
        var resourceMapping = resourceMappings[^1];
        var bytes = await resourceMapping.GetAllBytesAsync();

        Assert.Equal(yaml, Encoding.UTF8.GetString(bytes));
    }

    [Fact]
    public void MocklyConfiguration_Merge_PrefersNewInlineConfig()
    {
        var merged = new MocklyConfiguration(
            new MocklyConfiguration("old"),
            new MocklyConfiguration("new"));

        Assert.Equal("new", merged.InlineConfig);
    }

    [Fact]
    public async Task MocklyContainer_PostAsync_OmitsNullJsonValues()
    {
        var handler = new FakeHttpHandler();
        using var http = new HttpClient(handler) { BaseAddress = new Uri("http://127.0.0.1:9091") };

        var mock = new Mock(
            "m1",
            new MockRequest("GET", "/hello"),
            new MockResponse(200, "ok"));

        await MocklyContainer.PostAsync(http, "/api/mocks/http", mock);

        Assert.Single(handler.Requests);
        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Equal("/api/mocks/http", handler.Requests[0].PathAndQuery);

        using var json = JsonDocument.Parse(handler.Requests[0].Body!);
        var response = json.RootElement.GetProperty("response");
        Assert.False(response.TryGetProperty("headers", out _));
        Assert.False(response.TryGetProperty("delay", out _));
    }
    private sealed class FakeHttpHandler : HttpMessageHandler
    {
        public List<(string Method, string PathAndQuery, string? Body)> Requests { get; } = new();

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var body = request.Content != null
                ? await request.Content.ReadAsStringAsync(cancellationToken)
                : null;

            Requests.Add((request.Method.Method, request.RequestUri!.PathAndQuery, body));
            return new HttpResponseMessage(System.Net.HttpStatusCode.OK);
        }
    }
}
