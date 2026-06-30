using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Text;
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
        public string ResponseBody { get; set; } = string.Empty;

        protected override async Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var body = request.Content != null
                ? await request.Content.ReadAsStringAsync(cancellationToken)
                : null;
            Requests.Add((request.Method.Method, request.RequestUri!.PathAndQuery, body));
            return new HttpResponseMessage(NextStatusCode)
            {
                Content = new StringContent(ResponseBody, Encoding.UTF8, "application/json")
            };
        }
    }



    private static Mock SampleMock(string id = "m1")
        => new(id, new MockRequest("GET", "/ping"), new MockResponse(200, "ok"));

    private static Scenario SampleScenario(string name = "Test")
        => new("s1", name, Array.Empty<ScenarioPatch>());

    private static string MockJson(string id = "m1", int status = 200, string body = "ok")
        => JsonSerializer.Serialize(new
        {
            id,
            request = new { method = "GET", path = "/ping" },
            response = new { status, body }
        });

    private static string ScenarioJson(string name = "Test")
        => JsonSerializer.Serialize(new { id = "s1", name, patches = Array.Empty<object>() });

    private static string CallEntryJson()
        => JsonSerializer.Serialize(new
        {
            id = "c1",
            timestamp = "2026-01-01T00:00:00Z",
            protocol = "http",
            method = "GET",
            path = "/ping",
            status = 200,
            duration_ms = 5,
            matched_id = "m1"
        });

    private static string CallSummaryJson()
        => JsonSerializer.Serialize(new
        {
            mock_id = "m1",
            count = 2,
            calls = new[] { new
            {
                id = "c1",
                timestamp = "2026-01-01T00:00:00Z",
                protocol = "http",
                method = "GET",
                path = "/ping",
                status = 200,
                duration_ms = 5,
                matched_id = "m1"
            } }
        });

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
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.OK };
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
        Assert.Equal("/api/fault/http", handler.Requests[0].PathAndQuery);
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

    [Fact]
    public async Task ListMocksAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = "[" + MockJson() + "]" };
        await using var server = CreateTestServer(handler);

        var result = await server.ListMocksAsync();

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/mocks/http", handler.Requests[0].PathAndQuery);
        Assert.Equal("m1", result[0].Id);
    }

    [Fact]
    public async Task ListMocksAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.ListMocksAsync());
    }

    [Fact]
    public async Task UpdateMockAsync_PutsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = MockJson(status: 201, body: "updated") };
        await using var server = CreateTestServer(handler);

        var result = await server.UpdateMockAsync("m1", SampleMock());

        Assert.Equal("PUT", handler.Requests[0].Method);
        Assert.Equal("/api/mocks/http/m1", handler.Requests[0].PathAndQuery);
        Assert.Contains("\"id\":\"m1\"", handler.Requests[0].Body!);
        Assert.Equal(201, result.Response.Status);
    }

    [Fact]
    public async Task UpdateMockAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.UpdateMockAsync("m1", SampleMock()));
    }

    [Fact]
    public async Task PatchMockAsync_PatchesCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = MockJson(status: 201, body: "patched") };
        await using var server = CreateTestServer(handler);

        var result = await server.PatchMockAsync("m1", new MockResponsePatch(201, "patched"));

        Assert.Equal("PATCH", handler.Requests[0].Method);
        Assert.Equal("/api/mocks/http/m1", handler.Requests[0].PathAndQuery);
        Assert.Contains("\"status\":201", handler.Requests[0].Body!);
        Assert.Equal("patched", result.Response.Body);
    }

    [Fact]
    public async Task PatchMockAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.PatchMockAsync("m1", new MockResponsePatch(201)));
    }

    [Fact]
    public async Task GetStateAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = JsonSerializer.Serialize(new { key1 = "val1" }) };
        await using var server = CreateTestServer(handler);

        var result = await server.GetStateAsync();

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/state", handler.Requests[0].PathAndQuery);
        Assert.Equal("val1", result["key1"]);
    }

    [Fact]
    public async Task GetStateAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.GetStateAsync());
    }

    [Fact]
    public async Task SetStateAsync_PostsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = JsonSerializer.Serialize(new { key1 = "val1" }) };
        await using var server = CreateTestServer(handler);

        var result = await server.SetStateAsync(new Dictionary<string, string> { ["key1"] = "val1" });

        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Equal("/api/state", handler.Requests[0].PathAndQuery);
        Assert.Contains("key1", handler.Requests[0].Body!);
        Assert.Equal("val1", result["key1"]);
    }

    [Fact]
    public async Task SetStateAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.SetStateAsync(new Dictionary<string, string> { ["key1"] = "val1" }));
    }

    [Fact]
    public async Task DeleteStateAsync_DeletesCorrectEndpoint()
    {
        var handler = new FakeHttpHandler();
        await using var server = CreateTestServer(handler);

        await server.DeleteStateAsync("key1");

        Assert.Equal("DELETE", handler.Requests[0].Method);
        Assert.Equal("/api/state/key1", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task DeleteStateAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.DeleteStateAsync("key1"));
    }

    [Fact]
    public async Task GetLogsAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = "[" + CallEntryJson() + "]" };
        await using var server = CreateTestServer(handler);

        var result = await server.GetLogsAsync("m1");

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/logs?matched_id=m1", handler.Requests[0].PathAndQuery);
        Assert.Equal("m1", result[0].MatchedId);
    }

    [Fact]
    public async Task GetLogsAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.GetLogsAsync());
    }

    [Fact]
    public async Task ClearLogsAsync_DeletesCorrectEndpoint()
    {
        var handler = new FakeHttpHandler();
        await using var server = CreateTestServer(handler);

        await server.ClearLogsAsync();

        Assert.Equal("DELETE", handler.Requests[0].Method);
        Assert.Equal("/api/logs", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task ClearLogsAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.ClearLogsAsync());
    }

    [Fact]
    public async Task GetLogsCountAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = JsonSerializer.Serialize(new { count = 5 }) };
        await using var server = CreateTestServer(handler);

        var result = await server.GetLogsCountAsync("m1");

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/logs/count?matched_id=m1", handler.Requests[0].PathAndQuery);
        Assert.Equal(5, result);
    }

    [Fact]
    public async Task GetLogsCountAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.GetLogsCountAsync());
    }

    [Fact]
    public async Task ListScenariosAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = "[" + ScenarioJson() + "]" };
        await using var server = CreateTestServer(handler);

        var result = await server.ListScenariosAsync();

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/scenarios", handler.Requests[0].PathAndQuery);
        Assert.Equal("s1", result[0].Id);
    }

    [Fact]
    public async Task ListScenariosAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.ListScenariosAsync());
    }

    [Fact]
    public async Task CreateScenarioAsync_PostsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.Created, ResponseBody = ScenarioJson() };
        await using var server = CreateTestServer(handler);

        var result = await server.CreateScenarioAsync(SampleScenario());

        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Equal("/api/scenarios", handler.Requests[0].PathAndQuery);
        Assert.Contains("\"id\":\"s1\"", handler.Requests[0].Body!);
        Assert.Equal("Test", result.Name);
    }

    [Fact]
    public async Task CreateScenarioAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.CreateScenarioAsync(SampleScenario()));
    }

    [Fact]
    public async Task GetScenarioAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = ScenarioJson() };
        await using var server = CreateTestServer(handler);

        var result = await server.GetScenarioAsync("s1");

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/scenarios/s1", handler.Requests[0].PathAndQuery);
        Assert.Equal("s1", result.Id);
    }

    [Fact]
    public async Task GetScenarioAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.GetScenarioAsync("s1"));
    }

    [Fact]
    public async Task UpdateScenarioAsync_PutsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = ScenarioJson("Updated") };
        await using var server = CreateTestServer(handler);

        var result = await server.UpdateScenarioAsync("s1", SampleScenario("Updated"));

        Assert.Equal("PUT", handler.Requests[0].Method);
        Assert.Equal("/api/scenarios/s1", handler.Requests[0].PathAndQuery);
        Assert.Contains("Updated", handler.Requests[0].Body!);
        Assert.Equal("Updated", result.Name);
    }

    [Fact]
    public async Task UpdateScenarioAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.UpdateScenarioAsync("s1", SampleScenario("Updated")));
    }

    [Fact]
    public async Task DeleteScenarioAsync_DeletesCorrectEndpoint()
    {
        var handler = new FakeHttpHandler();
        await using var server = CreateTestServer(handler);

        await server.DeleteScenarioAsync("s1");

        Assert.Equal("DELETE", handler.Requests[0].Method);
        Assert.Equal("/api/scenarios/s1", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task DeleteScenarioAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.DeleteScenarioAsync("s1"));
    }

    [Fact]
    public async Task ListActiveScenariosAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = JsonSerializer.Serialize(new { active = new[] { "s1" }, scenarios = new[] { new { id = "s1", name = "Test", patches = Array.Empty<object>() } } }) };
        await using var server = CreateTestServer(handler);

        var result = await server.ListActiveScenariosAsync();

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/scenarios/active", handler.Requests[0].PathAndQuery);
        Assert.Equal("s1", result.Active[0]);
        Assert.Equal("s1", result.Scenarios[0].Id);
    }

    [Fact]
    public async Task ListActiveScenariosAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.ListActiveScenariosAsync());
    }

    [Fact]
    public async Task GetCallsAsync_GetsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = CallSummaryJson() };
        await using var server = CreateTestServer(handler);

        var result = await server.GetCallsAsync("m1");

        Assert.Equal("GET", handler.Requests[0].Method);
        Assert.Equal("/api/calls/http/m1", handler.Requests[0].PathAndQuery);
        Assert.Equal(2, result.Count);
        Assert.Equal("c1", result.Calls[0].Id);
    }

    [Fact]
    public async Task GetCallsAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.GetCallsAsync("m1"));
    }

    [Fact]
    public async Task ClearCallsAsync_DeletesCorrectEndpoint()
    {
        var handler = new FakeHttpHandler();
        await using var server = CreateTestServer(handler);

        await server.ClearCallsAsync("m1");

        Assert.Equal("DELETE", handler.Requests[0].Method);
        Assert.Equal("/api/calls/http/m1", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task ClearCallsAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.ClearCallsAsync("m1"));
    }

    [Fact]
    public async Task ClearAllCallsAsync_DeletesCorrectEndpoint()
    {
        var handler = new FakeHttpHandler();
        await using var server = CreateTestServer(handler);

        await server.ClearAllCallsAsync();

        Assert.Equal("DELETE", handler.Requests[0].Method);
        Assert.Equal("/api/calls/http", handler.Requests[0].PathAndQuery);
    }

    [Fact]
    public async Task ClearAllCallsAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.InternalServerError, ResponseBody = JsonSerializer.Serialize(new { error = "boom" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.ClearAllCallsAsync());
    }

    [Fact]
    public async Task WaitForCallsAsync_PostsCorrectEndpointAndParsesResponse()
    {
        var handler = new FakeHttpHandler { ResponseBody = CallSummaryJson() };
        await using var server = CreateTestServer(handler);

        var result = await server.WaitForCallsAsync("m1", 2, TimeSpan.FromSeconds(5));

        Assert.Equal("POST", handler.Requests[0].Method);
        Assert.Equal("/api/calls/http/m1/wait", handler.Requests[0].PathAndQuery);
        Assert.Contains("5000ms", handler.Requests[0].Body!);
        Assert.Equal(2, result.Count);
        Assert.Equal("c1", result.Calls[0].Id);
    }

    [Fact]
    public async Task WaitForCallsAsync_ThrowsOnErrorStatus()
    {
        var handler = new FakeHttpHandler { NextStatusCode = HttpStatusCode.RequestTimeout, ResponseBody = JsonSerializer.Serialize(new { error = "timeout" }) };
        await using var server = CreateTestServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.WaitForCallsAsync("m1", 2, TimeSpan.FromSeconds(5)));
    }

}
