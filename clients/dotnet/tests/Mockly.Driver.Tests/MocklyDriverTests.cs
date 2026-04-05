using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Text.Json;
using Mockly.Driver;
using Mockly.Driver.Models;
using Xunit;

namespace Mockly.Driver.Tests;

public class MocklyDriverTests
{
    [Fact]
    public void GetFreePort_ReturnsValidPort()
    {
        using var socket = new Socket(AddressFamily.InterNetwork, SocketType.Stream, ProtocolType.Tcp);
        socket.Bind(new IPEndPoint(IPAddress.Loopback, 0));
        var port = ((IPEndPoint)socket.LocalEndPoint!).Port;

        Assert.InRange(port, 1024, 65535);
    }

    [Fact]
    public void GetBinaryPath_ReturnsNull_WhenMissing()
    {
        var env = new Dictionary<string, string>(); // no MOCKLY_BINARY_PATH
        var result = MocklyInstaller.GetBinaryPath(
            Path.Join(Path.GetTempPath(), "mockly-nonexistent-" + Guid.NewGuid()),
            env);
        Assert.Null(result);
    }

    [Fact]
    public void GetBinaryPath_FindsBinaryInBinDir()
    {
        var dir = Path.Join(Path.GetTempPath(), "mockly-test-" + Guid.NewGuid());
        Directory.CreateDirectory(dir);
        var exeName = System.Runtime.InteropServices.RuntimeInformation.IsOSPlatform(
            System.Runtime.InteropServices.OSPlatform.Windows) ? "mockly.exe" : "mockly";
        var binaryPath = Path.Join(dir, exeName);
        File.WriteAllText(binaryPath, "fake");

        try
        {
            var env = new Dictionary<string, string>();
            var result = MocklyInstaller.GetBinaryPath(dir, env);
            Assert.Equal(binaryPath, result);
        }
        finally
        {
            Directory.Delete(dir, true);
        }
    }

    [Fact]
    public async Task Install_Throws_WhenNoInstallSet()
    {
        var env = new Dictionary<string, string>
        {
            ["MOCKLY_NO_INSTALL"] = "1"
        };
        var opts = new InstallOptions(BinDir: Path.Join(Path.GetTempPath(), "mockly-nope-" + Guid.NewGuid()));
        var ex = await Assert.ThrowsAsync<InvalidOperationException>(
            () => MocklyInstaller.InstallAsync(opts, env));
        Assert.Contains("MOCKLY_NO_INSTALL", ex.Message);
    }

    [Fact]
    public async Task Install_ReturnsStagedBinary_WhenPathSet()
    {
        var tmpFile = Path.Join(Path.GetTempPath(), "fake-mockly-" + Guid.NewGuid());
        File.WriteAllText(tmpFile, "fake binary");
        try
        {
            var env = new Dictionary<string, string>
            {
                ["MOCKLY_BINARY_PATH"] = tmpFile
            };
            var result = await MocklyInstaller.InstallAsync(null, env);
            Assert.Equal(tmpFile, result);
        }
        finally
        {
            File.Delete(tmpFile);
        }
    }

    // -----------------------------------------------------------------------
    // JSON serialization tests
    // -----------------------------------------------------------------------

    [Fact]
    public void Mock_SerializesToExpectedJson()
    {
        var mock = new Mock(
            "test-id",
            new MockRequest("GET", "/users", new Dictionary<string, string> { ["Authorization"] = "Bearer token" }),
            new MockResponse(200, "[{\"id\":1}]", null, "50ms"));

        var json = JsonSerializer.Serialize(mock);

        Assert.Contains("\"id\":\"test-id\"", json);
        Assert.Contains("\"method\":\"GET\"", json);
        Assert.Contains("\"path\":\"/users\"", json);
        Assert.Contains("\"status\":200", json);
        Assert.Contains("\"delay\":\"50ms\"", json);
        Assert.Contains("\"Authorization\"", json);
        Assert.Contains("\"Bearer token\"", json);
    }

    [Fact]
    public void Mock_MinimalFields_SerializesCorrectly()
    {
        var mock = new Mock(
            "min-id",
            new MockRequest("POST", "/items"),
            new MockResponse(204));

        var json = JsonSerializer.Serialize(mock);

        Assert.Contains("\"id\":\"min-id\"", json);
        Assert.Contains("\"method\":\"POST\"", json);
        Assert.Contains("\"path\":\"/items\"", json);
        Assert.Contains("\"status\":204", json);
    }

    [Fact]
    public void MockRequest_SerializesToExpectedJson()
    {
        var request = new MockRequest("DELETE", "/resource/1", new Dictionary<string, string> { ["X-Api-Key"] = "secret" });

        var json = JsonSerializer.Serialize(request);

        Assert.Contains("\"method\":\"DELETE\"", json);
        Assert.Contains("\"path\":\"/resource/1\"", json);
        Assert.Contains("\"X-Api-Key\"", json);
        Assert.Contains("\"secret\"", json);
    }

    [Fact]
    public void MockResponse_SerializesToExpectedJson()
    {
        var response = new MockResponse(
            201,
            "{\"created\":true}",
            new Dictionary<string, string> { ["Content-Type"] = "application/json" },
            "100ms");

        var json = JsonSerializer.Serialize(response);

        Assert.Contains("\"status\":201", json);
        Assert.Contains("\"body\"", json);
        Assert.Contains("\"delay\":\"100ms\"", json);
        Assert.Contains("\"Content-Type\"", json);
    }

    [Fact]
    public void FaultConfig_SerializesToExpectedJson()
    {
        var fault = new FaultConfig(true, "200ms", 503, 0.5);

        var json = JsonSerializer.Serialize(fault);

        Assert.Contains("\"enabled\":true", json);
        Assert.Contains("\"delay\":\"200ms\"", json);
        Assert.Contains("\"status_override\":503", json);
        Assert.Contains("\"error_rate\":0.5", json);
    }

    [Fact]
    public void FaultConfig_MinimalFields_SerializesCorrectly()
    {
        var fault = new FaultConfig(false);

        var json = JsonSerializer.Serialize(fault);

        Assert.Contains("\"enabled\":false", json);
    }

    [Fact]
    public void Mock_DeserializationRoundTrip()
    {
        var original = new Mock(
            "round-trip",
            new MockRequest("PUT", "/data", new Dictionary<string, string> { ["Accept"] = "application/json" }),
            new MockResponse(200, "ok", null, "10ms"));

        var json = JsonSerializer.Serialize(original);
        var deserialized = JsonSerializer.Deserialize<Mock>(json);

        Assert.NotNull(deserialized);
        Assert.Equal(original.Id, deserialized!.Id);
        Assert.Equal(original.Request.Method, deserialized.Request.Method);
        Assert.Equal(original.Request.Path, deserialized.Request.Path);
        Assert.Equal("application/json", deserialized.Request.Headers?["Accept"]);
        Assert.Equal(original.Response.Status, deserialized.Response.Status);
        Assert.Equal(original.Response.Delay, deserialized.Response.Delay);
    }

    // -----------------------------------------------------------------------
    // WriteConfig tests (via reflection — it's private static)
    // -----------------------------------------------------------------------

    [Fact]
    public void WriteConfig_CreatesFileWithExpectedContent()
    {
        var writeConfig = typeof(MocklyServer).GetMethod(
            "WriteConfig", BindingFlags.NonPublic | BindingFlags.Static)!;

        var configPath = (string)writeConfig.Invoke(null, new object?[] { 8080, 8081, null })!;
        try
        {
            Assert.True(File.Exists(configPath));
            var content = File.ReadAllText(configPath);
            Assert.Contains("mockly:", content);
            Assert.Contains("http:", content);
            Assert.Contains("port:", content);
            Assert.Contains("8081", content);  // httpPort
            Assert.Contains("8080", content);  // apiPort
        }
        finally
        {
            if (File.Exists(configPath)) File.Delete(configPath);
        }
    }

    [Fact]
    public void WriteConfig_WithScenarios_IncludesScenarioData()
    {
        var writeConfig = typeof(MocklyServer).GetMethod(
            "WriteConfig", BindingFlags.NonPublic | BindingFlags.Static)!;

        var scenarios = new List<Scenario>
        {
            new Scenario("sc1", "Error Scenario", new List<ScenarioPatch>
            {
                new ScenarioPatch("mock-1", 503, "error", "0ms")
            })
        };

        var configPath = (string)writeConfig.Invoke(null, new object?[] { 9090, 9091, scenarios })!;
        try
        {
            Assert.True(File.Exists(configPath));
            var content = File.ReadAllText(configPath);
            Assert.Contains("sc1", content);
            Assert.Contains("Error Scenario", content);
            Assert.Contains("mock-1", content);
            Assert.Contains("503", content);
        }
        finally
        {
            if (File.Exists(configPath)) File.Delete(configPath);
        }
    }

    // -----------------------------------------------------------------------
    // HTTP API method tests using FakeHttpHandler
    // -----------------------------------------------------------------------

    private sealed class FakeHttpHandler : HttpMessageHandler
    {
        public List<(string Method, string PathAndQuery, string? Body)> Requests { get; } = new();
        public HttpStatusCode NextStatusCode { get; set; } = HttpStatusCode.OK;

        protected override async Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var body = request.Content != null
                ? await request.Content.ReadAsStringAsync(cancellationToken)
                : null;
            Requests.Add((request.Method.Method, request.RequestUri!.PathAndQuery, body));
            return new HttpResponseMessage(NextStatusCode);
        }
    }

    private static MocklyServer CreateTestServer(FakeHttpHandler handler, int apiPort = 9999)
    {
        var client = new HttpClient(handler) { BaseAddress = new Uri($"http://127.0.0.1:{apiPort}") };
        var ctor = typeof(MocklyServer)
            .GetConstructors(BindingFlags.NonPublic | BindingFlags.Instance)
            .First();
        // Constructor: MocklyServer(Process process, int httpPort, int apiPort, HttpClient http)
        return (MocklyServer)ctor.Invoke(new object?[] { null, 8080, apiPort, client });
    }

    [Fact]
    public async Task AddMockAsync_PostsToCorrectEndpoint()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.Created };
        await using var server = CreateTestServer(handler);
        var mock = new Mock("m1", new MockRequest("GET", "/test"), new MockResponse(200));

        await server.AddMockAsync(mock);

        Assert.Single(handler.Requests);
        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Equal("/api/mocks/http", handler.Requests[0].PathAndQuery);
        Assert.Contains("\"id\":\"m1\"", handler.Requests[0].Body!);
    }

    [Fact]
    public async Task AddMockAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError };
        await using var server = CreateTestServer(handler);
        var mock = new Mock("m2", new MockRequest("GET", "/fail"), new MockResponse(200));

        await Assert.ThrowsAsync<HttpRequestException>(() => server.AddMockAsync(mock));
    }

    [Fact]
    public async Task DeleteMockAsync_DeletesCorrectEndpoint()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.NoContent };
        await using var server = CreateTestServer(handler);

        await server.DeleteMockAsync("test-id");

        Assert.Single(handler.Requests);
        Assert.Equal("DELETE", handler.Requests[0].Method);
        Assert.Contains("test-id", handler.Requests[0].PathAndQuery);
        Assert.Contains("/api/mocks/http/", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task DeleteMockAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.NotFound };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.DeleteMockAsync("no-such-id"));
    }

    [Fact]
    public async Task ResetAsync_PostsToResetEndpoint()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.OK };
        await using var server = CreateTestServer(handler);

        await server.ResetAsync();

        Assert.Single(handler.Requests);
        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Equal("/api/reset", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task ActivateScenarioAsync_PostsToCorrectEndpoint()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.OK };
        await using var server = CreateTestServer(handler);

        await server.ActivateScenarioAsync("sc1");

        Assert.Single(handler.Requests);
        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Contains("sc1", handler.Requests[0].PathAndQuery);
        Assert.Contains("activate", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task DeactivateScenarioAsync_PostsToCorrectEndpoint()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.OK };
        await using var server = CreateTestServer(handler);

        await server.DeactivateScenarioAsync("sc1");

        Assert.Single(handler.Requests);
        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Contains("sc1", handler.Requests[0].PathAndQuery);
        Assert.Contains("deactivate", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task SetFaultAsync_PostsToFaultEndpoint()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.OK };
        await using var server = CreateTestServer(handler);
        var fault = new FaultConfig(true, "100ms", null, 0.3);

        await server.SetFaultAsync(fault);

        Assert.Single(handler.Requests);
        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Equal("/api/fault", handler.Requests[0].PathAndQuery);
        Assert.Contains("\"enabled\":true", handler.Requests[0].Body!);
        Assert.Contains("\"delay\":\"100ms\"", handler.Requests[0].Body!);
    }

    [Fact]
    public async Task ClearFaultAsync_DeletesFaultEndpoint()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.OK };
        await using var server = CreateTestServer(handler);

        await server.ClearFaultAsync();

        Assert.Single(handler.Requests);
        Assert.Equal("DELETE", handler.Requests[0].Method);
        Assert.Equal("/api/fault", handler.Requests[0].PathAndQuery);
    }

    // -----------------------------------------------------------------------
    // Model optional-field serialization tests
    // -----------------------------------------------------------------------

    [Fact]
    public void MockResponse_RequiredFieldAlwaysPresent()
    {
        var response = new MockResponse(404);
        var json = JsonSerializer.Serialize(response);
        Assert.Contains("\"status\":404", json);
    }

    [Fact]
    public void ScenarioPatch_SerializesWithJsonPropertyNames()
    {
        var patch = new ScenarioPatch("mock-abc", 500, "error body", "0ms");
        var json = JsonSerializer.Serialize(patch);

        Assert.Contains("\"mock_id\":\"mock-abc\"", json);
        Assert.Contains("\"status\":500", json);
        Assert.Contains("\"body\":\"error body\"", json);
        Assert.Contains("\"delay\":\"0ms\"", json);
    }

    [Fact]
    public void Scenario_SerializesWithJsonPropertyNames()
    {
        var scenario = new Scenario("s1", "My Scenario", new List<ScenarioPatch>
        {
            new ScenarioPatch("m1", 503)
        });

        var json = JsonSerializer.Serialize(scenario);

        Assert.Contains("\"id\":\"s1\"", json);
        Assert.Contains("\"name\":\"My Scenario\"", json);
        Assert.Contains("\"patches\"", json);
        Assert.Contains("\"mock_id\":\"m1\"", json);
    }
}
