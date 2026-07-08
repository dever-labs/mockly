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
    /// <inheritdoc/>
    public Task AddMockAsync(Mock mock)
        => AddMockAsync(mock, default);

    /// <inheritdoc cref="IMocklyServer.AddMockAsync"/>
    public Task AddMockAsync(Mock mock, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/mocks/http", mock, cancellationToken);

    /// <inheritdoc/>
    public Task<IReadOnlyList<Mock>> ListMocksAsync()
        => ListMocksAsync(default);

    /// <inheritdoc cref="IMocklyServer.ListMocksAsync"/>
    public Task<IReadOnlyList<Mock>> ListMocksAsync(CancellationToken cancellationToken = default)
        => GetAsync<IReadOnlyList<Mock>>(GetOrCreateHttpClient(), "/api/mocks/http", cancellationToken);

    /// <inheritdoc/>
    public Task<Mock> UpdateMockAsync(string id, Mock mock)
        => UpdateMockAsync(id, mock, default);

    /// <inheritdoc cref="IMocklyServer.UpdateMockAsync"/>
    public Task<Mock> UpdateMockAsync(string id, Mock mock, CancellationToken cancellationToken = default)
        => PutAndReadAsync<Mock>(GetOrCreateHttpClient(), $"/api/mocks/http/{Uri.EscapeDataString(id)}", mock, cancellationToken);

    /// <inheritdoc/>
    public Task<Mock> PatchMockAsync(string id, MockResponsePatch patch)
        => PatchMockAsync(id, patch, default);

    /// <inheritdoc cref="IMocklyServer.PatchMockAsync"/>
    public Task<Mock> PatchMockAsync(string id, MockResponsePatch patch, CancellationToken cancellationToken = default)
        => PatchAndReadAsync<Mock>(GetOrCreateHttpClient(), $"/api/mocks/http/{Uri.EscapeDataString(id)}", patch, cancellationToken);

    /// <inheritdoc/>
    public Task DeleteMockAsync(string id)
        => DeleteMockAsync(id, default);

    /// <inheritdoc cref="IMocklyServer.DeleteMockAsync"/>
    public Task DeleteMockAsync(string id, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/mocks/http/{Uri.EscapeDataString(id)}", cancellationToken);

    /// <inheritdoc/>
    public Task<Dictionary<string, string>> GetStateAsync()
        => GetStateAsync(default);

    /// <inheritdoc cref="IMocklyServer.GetStateAsync"/>
    public Task<Dictionary<string, string>> GetStateAsync(CancellationToken cancellationToken = default)
        => GetAsync<Dictionary<string, string>>(GetOrCreateHttpClient(), "/api/state", cancellationToken);

    /// <inheritdoc/>
    public Task<Dictionary<string, string>> SetStateAsync(Dictionary<string, string> kvMap)
        => SetStateAsync(kvMap, default);

    /// <inheritdoc cref="IMocklyServer.SetStateAsync"/>
    public Task<Dictionary<string, string>> SetStateAsync(Dictionary<string, string> kvMap, CancellationToken cancellationToken = default)
        => PostAndReadAsync<Dictionary<string, string>>(GetOrCreateHttpClient(), "/api/state", kvMap, cancellationToken);

    /// <inheritdoc/>
    public Task DeleteStateAsync(string key)
        => DeleteStateAsync(key, default);

    /// <inheritdoc cref="IMocklyServer.DeleteStateAsync"/>
    public Task DeleteStateAsync(string key, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/state/{Uri.EscapeDataString(key)}", cancellationToken);

    /// <inheritdoc/>
    public Task<IReadOnlyList<CallEntry>> GetLogsAsync(string? matchedId = null)
        => GetLogsAsync(matchedId, default);

    /// <inheritdoc cref="IMocklyServer.GetLogsAsync"/>
    public Task<IReadOnlyList<CallEntry>> GetLogsAsync(string? matchedId, CancellationToken cancellationToken)
        => GetAsync<IReadOnlyList<CallEntry>>(GetOrCreateHttpClient(), WithMatchedId("/api/logs", matchedId), cancellationToken);

    /// <inheritdoc/>
    public Task ClearLogsAsync()
        => ClearLogsAsync(default);

    /// <inheritdoc cref="IMocklyServer.ClearLogsAsync"/>
    public Task ClearLogsAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/logs", cancellationToken);

    /// <inheritdoc/>
    public Task<int> GetLogsCountAsync(string? matchedId = null)
        => GetLogsCountAsync(matchedId, default);

    /// <inheritdoc cref="IMocklyServer.GetLogsCountAsync"/>
    public async Task<int> GetLogsCountAsync(string? matchedId, CancellationToken cancellationToken)
        => (await GetAsync<CountResponse>(GetOrCreateHttpClient(), WithMatchedId("/api/logs/count", matchedId), cancellationToken).ConfigureAwait(false)).Count;

    /// <inheritdoc/>
    public Task<IReadOnlyList<Scenario>> ListScenariosAsync()
        => ListScenariosAsync(default);

    /// <inheritdoc cref="IMocklyServer.ListScenariosAsync"/>
    public Task<IReadOnlyList<Scenario>> ListScenariosAsync(CancellationToken cancellationToken = default)
        => GetAsync<IReadOnlyList<Scenario>>(GetOrCreateHttpClient(), "/api/scenarios", cancellationToken);

    /// <inheritdoc/>
    public Task<Scenario> CreateScenarioAsync(Scenario scenario)
        => CreateScenarioAsync(scenario, default);

    /// <inheritdoc cref="IMocklyServer.CreateScenarioAsync"/>
    public Task<Scenario> CreateScenarioAsync(Scenario scenario, CancellationToken cancellationToken = default)
        => PostAndReadAsync<Scenario>(GetOrCreateHttpClient(), "/api/scenarios", scenario, cancellationToken);

    /// <inheritdoc/>
    public Task<Scenario> GetScenarioAsync(string scenarioId)
        => GetScenarioAsync(scenarioId, default);

    /// <inheritdoc cref="IMocklyServer.GetScenarioAsync"/>
    public Task<Scenario> GetScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => GetAsync<Scenario>(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}", cancellationToken);

    /// <inheritdoc/>
    public Task<Scenario> UpdateScenarioAsync(string scenarioId, Scenario scenario)
        => UpdateScenarioAsync(scenarioId, scenario, default);

    /// <inheritdoc cref="IMocklyServer.UpdateScenarioAsync"/>
    public Task<Scenario> UpdateScenarioAsync(string scenarioId, Scenario scenario, CancellationToken cancellationToken = default)
        => PutAndReadAsync<Scenario>(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}", scenario, cancellationToken);

    /// <inheritdoc/>
    public Task DeleteScenarioAsync(string scenarioId)
        => DeleteScenarioAsync(scenarioId, default);

    /// <inheritdoc cref="IMocklyServer.DeleteScenarioAsync"/>
    public Task DeleteScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}", cancellationToken);

    /// <inheritdoc/>
    public Task<ActiveScenariosResponse> ListActiveScenariosAsync()
        => ListActiveScenariosAsync(default);

    /// <inheritdoc cref="IMocklyServer.ListActiveScenariosAsync"/>
    public Task<ActiveScenariosResponse> ListActiveScenariosAsync(CancellationToken cancellationToken = default)
        => GetAsync<ActiveScenariosResponse>(GetOrCreateHttpClient(), "/api/scenarios/active", cancellationToken);

    /// <inheritdoc/>
    public Task ResetAsync()
        => ResetAsync(default);

    /// <inheritdoc cref="IMocklyServer.ResetAsync"/>
    public Task ResetAsync(CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/reset", null, cancellationToken);

    /// <inheritdoc/>
    public Task ActivateScenarioAsync(string scenarioId)
        => ActivateScenarioAsync(scenarioId, default);

    /// <inheritdoc cref="IMocklyServer.ActivateScenarioAsync"/>
    public Task ActivateScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/activate", null, cancellationToken);

    /// <inheritdoc/>
    public Task DeactivateScenarioAsync(string scenarioId)
        => DeactivateScenarioAsync(scenarioId, default);

    /// <inheritdoc cref="IMocklyServer.DeactivateScenarioAsync"/>
    public Task DeactivateScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/deactivate", null, cancellationToken);

    /// <inheritdoc/>
    public Task SetFaultAsync(FaultConfig config)
        => SetFaultAsync(config, default);

    /// <inheritdoc cref="IMocklyServer.SetFaultAsync"/>
    public Task SetFaultAsync(FaultConfig config, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/fault/http", config, cancellationToken);

    /// <inheritdoc/>
    public Task ClearFaultAsync()
        => ClearFaultAsync(default);

    /// <inheritdoc cref="IMocklyServer.ClearFaultAsync"/>
    public Task ClearFaultAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/fault", cancellationToken);

    /// <inheritdoc/>
    public Task<CallSummary> GetCallsAsync(string mockId)
        => GetCallsAsync(mockId, default);

    /// <inheritdoc cref="IMocklyServer.GetCallsAsync"/>
    public Task<CallSummary> GetCallsAsync(string mockId, CancellationToken cancellationToken = default)
        => GetAsync<CallSummary>(GetOrCreateHttpClient(), $"/api/calls/http/{Uri.EscapeDataString(mockId)}", cancellationToken);

    /// <inheritdoc/>
    public Task ClearCallsAsync(string mockId)
        => ClearCallsAsync(mockId, default);

    /// <inheritdoc cref="IMocklyServer.ClearCallsAsync"/>
    public Task ClearCallsAsync(string mockId, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/calls/http/{Uri.EscapeDataString(mockId)}", cancellationToken);

    /// <inheritdoc/>
    public Task ClearAllCallsAsync()
        => ClearAllCallsAsync(default);

    /// <inheritdoc cref="IMocklyServer.ClearAllCallsAsync"/>
    public Task ClearAllCallsAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/calls/http", cancellationToken);

    /// <inheritdoc/>
    public Task<CallSummary> WaitForCallsAsync(string mockId, int count = 1, TimeSpan? timeout = null)
        => WaitForCallsAsync(mockId, count, timeout, default);

    /// <inheritdoc cref="IMocklyServer.WaitForCallsAsync"/>
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
