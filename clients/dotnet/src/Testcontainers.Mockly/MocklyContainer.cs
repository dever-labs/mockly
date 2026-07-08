using System.Text.Json;
using System.Text.Json.Serialization;

namespace Testcontainers.Mockly;

/// <summary>
/// A started Mockly container. Use <see cref="MocklyBuilder"/> to create and start instances.
/// </summary>
public sealed class MocklyContainer : DockerContainer, IMocklyServer
{
    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        PropertyNameCaseInsensitive = true,
    };

    private HttpClient? _http;

    /// <summary>Initialises the container from the given configuration.</summary>
    public MocklyContainer(MocklyConfiguration configuration)
        : base(configuration)
    {
    }

    /// <summary>Returns the base URL of the HTTP mock server, e.g. <c>http://localhost:32768</c>.</summary>
    public string GetHttpBaseAddress()
        => new UriBuilder(Uri.UriSchemeHttp, Hostname, GetMappedPublicPort(MocklyBuilder.HttpPort)).ToString().TrimEnd('/');

    /// <summary>Returns the base URL of the Mockly management API, e.g. <c>http://localhost:32769</c>.</summary>
    public string GetApiBaseAddress()
        => new UriBuilder(Uri.UriSchemeHttp, Hostname, GetMappedPublicPort(MocklyBuilder.ApiPort)).ToString().TrimEnd('/');

    /// <summary>Registers a new HTTP mock.</summary>
    public Task AddMockAsync(Mock mock)
        => AddMockAsync(mock, default);

    /// <summary>Registers a new HTTP mock.</summary>
    public Task AddMockAsync(Mock mock, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/mocks/http", mock, cancellationToken);

    /// <summary>Returns all registered HTTP mocks.</summary>
    public Task<IReadOnlyList<Mock>> ListMocksAsync()
        => ListMocksAsync(default);

    /// <summary>Returns all registered HTTP mocks.</summary>
    public Task<IReadOnlyList<Mock>> ListMocksAsync(CancellationToken cancellationToken = default)
        => GetAsync<IReadOnlyList<Mock>>(GetOrCreateHttpClient(), "/api/mocks/http", cancellationToken);

    /// <summary>Replaces the mock with the specified <paramref name="id"/>.</summary>
    public Task<Mock> UpdateMockAsync(string id, Mock mock)
        => UpdateMockAsync(id, mock, default);

    /// <summary>Replaces the mock with the specified <paramref name="id"/>.</summary>
    public Task<Mock> UpdateMockAsync(string id, Mock mock, CancellationToken cancellationToken = default)
        => PutAndReadAsync<Mock>(GetOrCreateHttpClient(), $"/api/mocks/http/{Uri.EscapeDataString(id)}", mock, cancellationToken);

    /// <summary>Partially updates the response of the mock with the specified <paramref name="id"/>.</summary>
    public Task<Mock> PatchMockAsync(string id, MockResponsePatch patch)
        => PatchMockAsync(id, patch, default);

    /// <summary>Partially updates the response of the mock with the specified <paramref name="id"/>.</summary>
    public Task<Mock> PatchMockAsync(string id, MockResponsePatch patch, CancellationToken cancellationToken = default)
        => PatchAndReadAsync<Mock>(GetOrCreateHttpClient(), $"/api/mocks/http/{Uri.EscapeDataString(id)}", patch, cancellationToken);

    /// <summary>Removes the mock with the specified <paramref name="id"/>.</summary>
    public Task DeleteMockAsync(string id)
        => DeleteMockAsync(id, default);

    /// <summary>Removes the mock with the specified <paramref name="id"/>.</summary>
    public Task DeleteMockAsync(string id, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/mocks/http/{Uri.EscapeDataString(id)}", cancellationToken);

    /// <summary>Returns all key-value entries in the state store.</summary>
    public Task<Dictionary<string, string>> GetStateAsync()
        => GetStateAsync(default);

    /// <summary>Returns all key-value entries in the state store.</summary>
    public Task<Dictionary<string, string>> GetStateAsync(CancellationToken cancellationToken = default)
        => GetAsync<Dictionary<string, string>>(GetOrCreateHttpClient(), "/api/state", cancellationToken);

    /// <summary>Merges <paramref name="kvMap"/> into the state store, adding or overwriting the given keys.</summary>
    public Task<Dictionary<string, string>> SetStateAsync(Dictionary<string, string> kvMap)
        => SetStateAsync(kvMap, default);

    /// <summary>Merges <paramref name="kvMap"/> into the state store, adding or overwriting the given keys.</summary>
    public Task<Dictionary<string, string>> SetStateAsync(Dictionary<string, string> kvMap, CancellationToken cancellationToken = default)
        => PostAndReadAsync<Dictionary<string, string>>(GetOrCreateHttpClient(), "/api/state", kvMap, cancellationToken);

    /// <summary>Removes the state entry with the specified <paramref name="key"/>.</summary>
    public Task DeleteStateAsync(string key)
        => DeleteStateAsync(key, default);

    /// <summary>Removes the state entry with the specified <paramref name="key"/>.</summary>
    public Task DeleteStateAsync(string key, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/state/{Uri.EscapeDataString(key)}", cancellationToken);

    /// <summary>Returns recent request log entries, optionally filtered by <paramref name="matchedId"/>.</summary>
    public Task<IReadOnlyList<CallEntry>> GetLogsAsync(string? matchedId = null)
        => GetLogsAsync(matchedId, default);

    /// <summary>Returns recent request log entries.</summary>
    public Task<IReadOnlyList<CallEntry>> GetLogsAsync(string? matchedId, CancellationToken cancellationToken)
        => GetAsync<IReadOnlyList<CallEntry>>(GetOrCreateHttpClient(), WithMatchedId("/api/logs", matchedId), cancellationToken);

    /// <summary>Clears all stored request log entries.</summary>
    public Task ClearLogsAsync()
        => ClearLogsAsync(default);

    /// <summary>Clears all stored request log entries.</summary>
    public Task ClearLogsAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/logs", cancellationToken);

    /// <summary>Returns the number of logged requests, optionally filtered by <paramref name="matchedId"/>.</summary>
    public Task<int> GetLogsCountAsync(string? matchedId = null)
        => GetLogsCountAsync(matchedId, default);

    /// <summary>Returns the number of logged requests, optionally filtered by <paramref name="matchedId"/>.</summary>
    public async Task<int> GetLogsCountAsync(string? matchedId, CancellationToken cancellationToken)
        => (await GetAsync<CountResponse>(GetOrCreateHttpClient(), WithMatchedId("/api/logs/count", matchedId), cancellationToken).ConfigureAwait(false)).Count;

    /// <summary>Returns all registered scenarios.</summary>
    public Task<IReadOnlyList<Scenario>> ListScenariosAsync()
        => ListScenariosAsync(default);

    /// <summary>Returns all registered scenarios.</summary>
    public Task<IReadOnlyList<Scenario>> ListScenariosAsync(CancellationToken cancellationToken = default)
        => GetAsync<IReadOnlyList<Scenario>>(GetOrCreateHttpClient(), "/api/scenarios", cancellationToken);

    /// <summary>Creates a new scenario.</summary>
    public Task<Scenario> CreateScenarioAsync(Scenario scenario)
        => CreateScenarioAsync(scenario, default);

    /// <summary>Creates a new scenario.</summary>
    public Task<Scenario> CreateScenarioAsync(Scenario scenario, CancellationToken cancellationToken = default)
        => PostAndReadAsync<Scenario>(GetOrCreateHttpClient(), "/api/scenarios", scenario, cancellationToken);

    /// <summary>Returns the scenario with the specified <paramref name="scenarioId"/>.</summary>
    public Task<Scenario> GetScenarioAsync(string scenarioId)
        => GetScenarioAsync(scenarioId, default);

    /// <summary>Returns the scenario with the specified <paramref name="scenarioId"/>.</summary>
    public Task<Scenario> GetScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => GetAsync<Scenario>(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}", cancellationToken);

    /// <summary>Replaces the scenario with the specified <paramref name="scenarioId"/>.</summary>
    public Task<Scenario> UpdateScenarioAsync(string scenarioId, Scenario scenario)
        => UpdateScenarioAsync(scenarioId, scenario, default);

    /// <summary>Replaces the scenario with the specified <paramref name="scenarioId"/>.</summary>
    public Task<Scenario> UpdateScenarioAsync(string scenarioId, Scenario scenario, CancellationToken cancellationToken = default)
        => PutAndReadAsync<Scenario>(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}", scenario, cancellationToken);

    /// <summary>Removes the scenario with the specified <paramref name="scenarioId"/>.</summary>
    public Task DeleteScenarioAsync(string scenarioId)
        => DeleteScenarioAsync(scenarioId, default);

    /// <summary>Removes the scenario with the specified <paramref name="scenarioId"/>.</summary>
    public Task DeleteScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}", cancellationToken);

    /// <summary>Returns all currently active scenarios.</summary>
    public Task<ActiveScenariosResponse> ListActiveScenariosAsync()
        => ListActiveScenariosAsync(default);

    /// <summary>Returns all currently active scenarios.</summary>
    public Task<ActiveScenariosResponse> ListActiveScenariosAsync(CancellationToken cancellationToken = default)
        => GetAsync<ActiveScenariosResponse>(GetOrCreateHttpClient(), "/api/scenarios/active", cancellationToken);

    /// <summary>Clears all dynamic mocks, deactivates all scenarios, and removes any fault configuration.</summary>
    public Task ResetAsync()
        => ResetAsync(default);

    /// <summary>Clears all dynamic mocks, deactivates all scenarios, and removes any fault configuration.</summary>
    public Task ResetAsync(CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/reset", null, cancellationToken);

    /// <summary>Activates the scenario with the given <paramref name="scenarioId"/>.</summary>
    public Task ActivateScenarioAsync(string scenarioId)
        => ActivateScenarioAsync(scenarioId, default);

    /// <summary>Activates the scenario with the given <paramref name="scenarioId"/>.</summary>
    public Task ActivateScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/activate", null, cancellationToken);

    /// <summary>Deactivates the scenario with the given <paramref name="scenarioId"/>.</summary>
    public Task DeactivateScenarioAsync(string scenarioId)
        => DeactivateScenarioAsync(scenarioId, default);

    /// <summary>Deactivates the scenario with the given <paramref name="scenarioId"/>.</summary>
    public Task DeactivateScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/deactivate", null, cancellationToken);

    /// <summary>Applies a fault configuration to the HTTP mock server.</summary>
    public Task SetFaultAsync(FaultConfig config)
        => SetFaultAsync(config, default);

    /// <summary>Applies a fault configuration to the HTTP mock server.</summary>
    public Task SetFaultAsync(FaultConfig config, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/fault/http", config, cancellationToken);

    /// <summary>Removes any active fault configuration.</summary>
    public Task ClearFaultAsync()
        => ClearFaultAsync(default);

    /// <summary>Removes any active fault configuration.</summary>
    public Task ClearFaultAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/fault", cancellationToken);

    /// <summary>Returns the call history for the mock with the specified <paramref name="mockId"/>.</summary>
    public Task<CallSummary> GetCallsAsync(string mockId)
        => GetCallsAsync(mockId, default);

    /// <summary>Returns the call history for the mock with the specified <paramref name="mockId"/>.</summary>
    public Task<CallSummary> GetCallsAsync(string mockId, CancellationToken cancellationToken = default)
        => GetAsync<CallSummary>(GetOrCreateHttpClient(), $"/api/calls/http/{Uri.EscapeDataString(mockId)}", cancellationToken);

    /// <summary>Clears the call history for the mock with the specified <paramref name="mockId"/>.</summary>
    public Task ClearCallsAsync(string mockId)
        => ClearCallsAsync(mockId, default);

    /// <summary>Clears the call history for the mock with the specified <paramref name="mockId"/>.</summary>
    public Task ClearCallsAsync(string mockId, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/calls/http/{Uri.EscapeDataString(mockId)}", cancellationToken);

    /// <summary>Clears the call history for all mocks.</summary>
    public Task ClearAllCallsAsync()
        => ClearAllCallsAsync(default);

    /// <summary>Clears the call history for all mocks.</summary>
    public Task ClearAllCallsAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/calls/http", cancellationToken);

    /// <summary>Blocks until the mock with <paramref name="mockId"/> has been called at least <paramref name="count"/> times, or until <paramref name="timeout"/> expires.</summary>
    public Task<CallSummary> WaitForCallsAsync(string mockId, int count = 1, TimeSpan? timeout = null)
        => WaitForCallsAsync(mockId, count, timeout, default);

    /// <summary>Blocks until the mock with <paramref name="mockId"/> has been called at least <paramref name="count"/> times, or until <paramref name="timeout"/> expires.</summary>
    public Task<CallSummary> WaitForCallsAsync(string mockId, int count, TimeSpan? timeout, CancellationToken cancellationToken)
    {
        var t = timeout ?? TimeSpan.FromSeconds(10);
        var body = new { count, timeout = $"{(long)t.TotalMilliseconds}ms" };
        return PostAndReadAsync<CallSummary>(GetOrCreateHttpClient(), $"/api/calls/http/{Uri.EscapeDataString(mockId)}/wait", body, cancellationToken);
    }

    /// <summary>Disposes the HTTP client and the underlying container.</summary>
    public new async ValueTask DisposeAsync()
    {
        _http?.Dispose();
        await base.DisposeAsync().ConfigureAwait(false);
    }

    internal static async Task PostAsync(HttpClient http, string path, object? body, CancellationToken cancellationToken = default)
    {
        using HttpContent content = CreateContent(body);
        using var response = await http.PostAsync(path, content, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, $"Mockly API POST {path} failed", cancellationToken).ConfigureAwait(false);
    }

    internal static Task<T> PostAndReadAsync<T>(HttpClient http, string path, object? body, CancellationToken cancellationToken = default)
        => SendAndReadAsync<T>(http, HttpMethod.Post, path, body, cancellationToken);

    internal static Task<T> PutAndReadAsync<T>(HttpClient http, string path, object? body, CancellationToken cancellationToken = default)
        => SendAndReadAsync<T>(http, HttpMethod.Put, path, body, cancellationToken);

    internal static Task<T> PatchAndReadAsync<T>(HttpClient http, string path, object? body, CancellationToken cancellationToken = default)
        => SendAndReadAsync<T>(http, HttpMethod.Patch, path, body, cancellationToken);

    internal static async Task DeleteAsync(HttpClient http, string path, CancellationToken cancellationToken = default)
    {
        using var response = await http.DeleteAsync(path, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, $"Mockly API DELETE {path} failed", cancellationToken).ConfigureAwait(false);
    }

    internal static async Task<string> GetStringAsync(HttpClient http, string path, CancellationToken cancellationToken = default)
    {
        using var response = await http.GetAsync(path, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, $"Mockly API GET {path} failed", cancellationToken).ConfigureAwait(false);
        return await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
    }

    internal static async Task<T> GetAsync<T>(HttpClient http, string path, CancellationToken cancellationToken = default)
    {
        var json = await GetStringAsync(http, path, cancellationToken).ConfigureAwait(false);
        return JsonSerializer.Deserialize<T>(json, JsonOpts)
            ?? throw new JsonException($"Mockly API GET {path} returned an empty payload.");
    }

    private static async Task<T> SendAndReadAsync<T>(HttpClient http, HttpMethod method, string path, object? body, CancellationToken cancellationToken)
    {
        using var request = new HttpRequestMessage(method, path)
        {
            Content = CreateContent(body),
        };
        using var response = await http.SendAsync(request, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, $"Mockly API {method.Method} {path} failed", cancellationToken).ConfigureAwait(false);

        var json = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
        return JsonSerializer.Deserialize<T>(json, JsonOpts)
            ?? throw new JsonException($"Mockly API {method.Method} {path} returned an empty payload.");
    }

    private static HttpContent CreateContent(object? body)
        => body != null
            ? new StringContent(JsonSerializer.Serialize(body, JsonOpts), Encoding.UTF8, "application/json")
            : new StringContent(string.Empty, Encoding.UTF8, "application/json");

    private HttpClient GetOrCreateHttpClient()
    {
        _http ??= new HttpClient
        {
            BaseAddress = new Uri(GetApiBaseAddress()),
        };

        return _http;
    }

    private static string WithMatchedId(string path, string? matchedId)
        => string.IsNullOrEmpty(matchedId)
            ? path
            : $"{path}?matched_id={Uri.EscapeDataString(matchedId)}";

    private static async Task EnsureSuccessAsync(HttpResponseMessage response, string errorPrefix, CancellationToken cancellationToken)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        var message = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
        throw new HttpRequestException($"{errorPrefix} ({(int)response.StatusCode}): {message}");
    }

    private sealed record CountResponse([property: JsonPropertyName("count")] int Count);
}
