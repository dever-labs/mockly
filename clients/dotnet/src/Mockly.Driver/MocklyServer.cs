using System.ComponentModel;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using Mockly.Driver.Models;

namespace Mockly.Driver;

public sealed class MocklyServer : IAsyncDisposable
{
    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly HttpClient _http;
    private readonly System.Diagnostics.Process? _process;

    public int HttpPort { get; }
    public int ApiPort { get; }
    public string HttpBase { get; }
    public string ApiBase { get; }

    private MocklyServer(System.Diagnostics.Process? process, int httpPort, int apiPort, HttpClient http)
    {
        _process = process;
        _http = http;
        HttpPort = httpPort;
        ApiPort = apiPort;
        HttpBase = $"http://127.0.0.1:{httpPort}";
        ApiBase = $"http://127.0.0.1:{apiPort}";
    }

    public static async Task<MocklyServer> CreateAsync(MocklyServerOptions? opts = null)
    {
        opts ??= new MocklyServerOptions();
        var binaryPath = await MocklyInstaller.InstallAsync(opts.InstallOptions);

        for (var attempt = 0; attempt < 3; attempt++)
        {
            var httpPort = await GetFreePortAsync();
            var apiPort = await GetFreePortAsync();

            var configPath = WriteConfig(apiPort, httpPort, opts.Scenarios);
            var stderr = new StringBuilder();

            var psi = new System.Diagnostics.ProcessStartInfo(binaryPath,
                $"start --config \"{configPath}\" --api-port {apiPort}")
            {
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true,
            };

            var process = new System.Diagnostics.Process { StartInfo = psi, EnableRaisingEvents = true };
            process.ErrorDataReceived += (_, e) => { if (e.Data != null) { lock (stderr) stderr.AppendLine(e.Data); } };
            process.Start();
            process.BeginErrorReadLine();

            var http = new HttpClient { BaseAddress = new Uri($"http://127.0.0.1:{apiPort}") };

            try
            {
                await WaitReadyAsync(http, TimeSpan.FromSeconds(10));
                return new MocklyServer(process, httpPort, apiPort, http);
            }
            catch (Exception ex) when (attempt < 2)
            {
                var stderrStr = stderr.ToString();
                if (stderrStr.Contains("address already in use", StringComparison.OrdinalIgnoreCase)
                    || stderrStr.Contains("EADDRINUSE", StringComparison.OrdinalIgnoreCase)
                    || ex.Message.Contains("address already in use", StringComparison.OrdinalIgnoreCase))
                {
                    http.Dispose();
                    try { process.Kill(); } catch (InvalidOperationException) { } catch (Win32Exception) { }
                    process.Dispose();
                    continue;
                }
                http.Dispose();
                try { process.Kill(); } catch (InvalidOperationException) { } catch (Win32Exception) { }
                process.Dispose();
                throw new InvalidOperationException(
                    $"Mockly failed to start: {ex.Message}\nStderr: {stderrStr}", ex);
            }
            catch (Exception ex)
            {
                var stderrStr = stderr.ToString();
                http.Dispose();
                try { process.Kill(); } catch (InvalidOperationException) { } catch (Win32Exception) { }
                process.Dispose();
                throw new InvalidOperationException(
                    $"Mockly failed to start after 3 attempts: {ex.Message}\nStderr: {stderrStr}", ex);
            }
        }

        throw new InvalidOperationException("Failed to start Mockly after 3 attempts (port conflict).");
    }

    public static async Task<MocklyServer> EnsureAsync(MocklyServerOptions? opts = null)
    {
        var server = await CreateAsync(opts);
        await server.ResetAsync();
        return server;
    }

    public async Task StopAsync()
    {
        _http.Dispose();
        if (_process is { HasExited: false })
        {
            try
            {
                _process.Kill();
                await _process.WaitForExitAsync();
            }
            catch (InvalidOperationException) { }
            catch (Win32Exception) { }
        }
        _process?.Dispose();
    }

    public ValueTask DisposeAsync() => new(StopAsync());

    public Task AddMockAsync(Mock mock)
        => PostAsync("/api/mocks/http", mock);

    public Task<IReadOnlyList<Mock>> ListMocksAsync()
        => GetAsync<IReadOnlyList<Mock>>("/api/mocks/http");

    public Task<Mock> UpdateMockAsync(string id, Mock mock)
        => PutAndReadAsync<Mock>($"/api/mocks/http/{Uri.EscapeDataString(id)}", mock);

    public Task<Mock> PatchMockAsync(string id, MockResponsePatch patch)
        => PatchAndReadAsync<Mock>($"/api/mocks/http/{Uri.EscapeDataString(id)}", patch);

    public Task DeleteMockAsync(string id)
        => DeleteAsync($"/api/mocks/http/{Uri.EscapeDataString(id)}");

    public Task<Dictionary<string, string>> GetStateAsync()
        => GetAsync<Dictionary<string, string>>("/api/state");

    public Task<Dictionary<string, string>> SetStateAsync(Dictionary<string, string> kvMap)
        => PostAndReadAsync<Dictionary<string, string>>("/api/state", kvMap);

    public Task DeleteStateAsync(string key)
        => DeleteAsync($"/api/state/{Uri.EscapeDataString(key)}");

    public Task<IReadOnlyList<CallEntry>> GetLogsAsync(string? matchedId = null)
        => GetAsync<IReadOnlyList<CallEntry>>(WithMatchedId("/api/logs", matchedId));

    public Task ClearLogsAsync()
        => DeleteAsync("/api/logs");

    public async Task<int> GetLogsCountAsync(string? matchedId = null)
        => (await GetAsync<CountResponse>(WithMatchedId("/api/logs/count", matchedId))).Count;

    public Task<IReadOnlyList<Scenario>> ListScenariosAsync()
        => GetAsync<IReadOnlyList<Scenario>>("/api/scenarios");

    public Task<Scenario> CreateScenarioAsync(Scenario scenario)
        => PostAndReadAsync<Scenario>("/api/scenarios", scenario);

    public Task<Scenario> GetScenarioAsync(string scenarioId)
        => GetAsync<Scenario>($"/api/scenarios/{Uri.EscapeDataString(scenarioId)}");

    public Task<Scenario> UpdateScenarioAsync(string scenarioId, Scenario scenario)
        => PutAndReadAsync<Scenario>($"/api/scenarios/{Uri.EscapeDataString(scenarioId)}", scenario);

    public Task DeleteScenarioAsync(string scenarioId)
        => DeleteAsync($"/api/scenarios/{Uri.EscapeDataString(scenarioId)}");

    public Task<ActiveScenariosResponse> ListActiveScenariosAsync()
        => GetAsync<ActiveScenariosResponse>("/api/scenarios/active");

    public Task ResetAsync()
        => PostAsync("/api/reset", null);

    public Task ActivateScenarioAsync(string scenarioId)
        => PostAsync($"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/activate", null);

    public Task DeactivateScenarioAsync(string scenarioId)
        => PostAsync($"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/deactivate", null);

    public Task SetFaultAsync(FaultConfig config)
        => PostAsync("/api/fault/http", config);

    public Task ClearFaultAsync()
        => DeleteAsync("/api/fault");

    public Task<CallSummary> GetCallsAsync(string mockId)
        => GetAsync<CallSummary>($"/api/calls/http/{Uri.EscapeDataString(mockId)}");

    public Task ClearCallsAsync(string mockId)
        => DeleteAsync($"/api/calls/http/{Uri.EscapeDataString(mockId)}");

    public Task ClearAllCallsAsync()
        => DeleteAsync("/api/calls/http");

    public Task<CallSummary> WaitForCallsAsync(string mockId, int count = 1, TimeSpan? timeout = null)
    {
        var t = timeout ?? TimeSpan.FromSeconds(10);
        var body = new { count, timeout = $"{(long)t.TotalMilliseconds}ms" };
        return PostAndReadAsync<CallSummary>($"/api/calls/http/{Uri.EscapeDataString(mockId)}/wait", body);
    }

    private Task PostAsync(string path, object? body)
        => SendWithoutReadingAsync(HttpMethod.Post, path, body);

    private Task<T> PostAndReadAsync<T>(string path, object? body)
        => SendAndReadAsync<T>(HttpMethod.Post, path, body);

    private Task<T> PutAndReadAsync<T>(string path, object? body)
        => SendAndReadAsync<T>(HttpMethod.Put, path, body);

    private Task<T> PatchAndReadAsync<T>(string path, object? body)
        => SendAndReadAsync<T>(HttpMethod.Patch, path, body);

    private async Task<T> GetAsync<T>(string path)
    {
        var response = await _http.GetAsync(path);
        if (!response.IsSuccessStatusCode)
        {
            var msg = await response.Content.ReadAsStringAsync();
            throw new HttpRequestException($"Mockly API GET {path} failed ({(int)response.StatusCode}): {msg}");
        }
        var json = await response.Content.ReadAsStringAsync();
        return JsonSerializer.Deserialize<T>(json, JsonOpts)!;
    }

    private async Task DeleteAsync(string path)
    {
        var response = await _http.DeleteAsync(path);
        if (!response.IsSuccessStatusCode)
        {
            var msg = await response.Content.ReadAsStringAsync();
            throw new HttpRequestException($"Mockly API DELETE {path} failed ({(int)response.StatusCode}): {msg}");
        }
    }

    private async Task SendWithoutReadingAsync(HttpMethod method, string path, object? body)
    {
        using var request = new HttpRequestMessage(method, path)
        {
            Content = CreateContent(body),
        };

        var response = await _http.SendAsync(request);
        if (!response.IsSuccessStatusCode)
        {
            var msg = await response.Content.ReadAsStringAsync();
            throw new HttpRequestException($"Mockly API {method.Method} {path} failed ({(int)response.StatusCode}): {msg}");
        }
    }

    private async Task<T> SendAndReadAsync<T>(HttpMethod method, string path, object? body)
    {
        using var request = new HttpRequestMessage(method, path)
        {
            Content = CreateContent(body),
        };

        var response = await _http.SendAsync(request);
        if (!response.IsSuccessStatusCode)
        {
            var msg = await response.Content.ReadAsStringAsync();
            throw new HttpRequestException($"Mockly API {method.Method} {path} failed ({(int)response.StatusCode}): {msg}");
        }
        var json = await response.Content.ReadAsStringAsync();
        return JsonSerializer.Deserialize<T>(json, JsonOpts)!;
    }

    private static HttpContent CreateContent(object? body)
        => body != null
            ? new StringContent(JsonSerializer.Serialize(body, JsonOpts), Encoding.UTF8, "application/json")
            : new StringContent(string.Empty, Encoding.UTF8, "application/json");

    private static string WithMatchedId(string path, string? matchedId)
        => string.IsNullOrEmpty(matchedId)
            ? path
            : $"{path}?matched_id={Uri.EscapeDataString(matchedId)}";

    private static Task<int> GetFreePortAsync()
    {
        using var socket = new Socket(AddressFamily.InterNetwork, SocketType.Stream, ProtocolType.Tcp);
        socket.Bind(new IPEndPoint(IPAddress.Loopback, 0));
        return Task.FromResult(((IPEndPoint)socket.LocalEndPoint!).Port);
    }

    private static string WriteConfig(int apiPort, int httpPort, IReadOnlyList<Scenario>? scenarios)
    {
        var scenariosYaml = new StringBuilder();
        if (scenarios != null && scenarios.Count > 0)
        {
            scenariosYaml.AppendLine("scenarios:");
            foreach (var scenario in scenarios)
            {
                scenariosYaml.AppendLine($"  - id: \"{scenario.Id}\"");
                scenariosYaml.AppendLine($"    name: \"{scenario.Name}\"");
                if (scenario.Description is not null) scenariosYaml.AppendLine($"    description: \"{scenario.Description}\"");
                if (scenario.Patches.Count > 0)
                {
                    scenariosYaml.AppendLine("    patches:");
                    foreach (var patch in scenario.Patches)
                    {
                        scenariosYaml.AppendLine($"      - mock_id: \"{patch.MockId}\"");
                        if (patch.Status.HasValue) scenariosYaml.AppendLine($"        status: {patch.Status}");
                        if (patch.Body != null) scenariosYaml.AppendLine($"        body: \"{patch.Body}\"");
                        if (patch.Headers is { Count: > 0 })
                        {
                            scenariosYaml.AppendLine("        headers:");
                            foreach (var header in patch.Headers)
                            {
                                scenariosYaml.AppendLine($"          \"{header.Key}\": \"{header.Value}\"");
                            }
                        }
                        if (patch.Delay != null) scenariosYaml.AppendLine($"        delay: \"{patch.Delay}\"");
                        if (patch.Disabled.HasValue) scenariosYaml.AppendLine($"        disabled: {patch.Disabled.Value.ToString().ToLowerInvariant()}");
                    }
                }
            }
        }

        var yaml = $@"mockly:
  api:
    port: {apiPort}
protocols:
  http:
    enabled: true
    port: {httpPort}
{scenariosYaml}";

        var tmpPath = Path.Join(Path.GetTempPath(), $"mockly-config-{Guid.NewGuid():N}.yaml");
        File.WriteAllText(tmpPath, yaml);
        return tmpPath;
    }

    private static async Task WaitReadyAsync(HttpClient http, TimeSpan maxWait)
    {
        var deadline = DateTime.UtcNow + maxWait;
        while (DateTime.UtcNow < deadline)
        {
            try
            {
                var resp = await http.GetAsync("/api/protocols");
                if (resp.IsSuccessStatusCode) return;
            }
            catch (HttpRequestException) { }
            catch (TaskCanceledException) { }
            await Task.Delay(50);
        }
        throw new TimeoutException($"Mockly did not become ready within {maxWait.TotalSeconds}s");
    }

    private sealed record CountResponse([property: JsonPropertyName("count")] int Count);
}

public record MocklyServerOptions(
    IReadOnlyList<Scenario>? Scenarios = null,
    InstallOptions? InstallOptions = null);
