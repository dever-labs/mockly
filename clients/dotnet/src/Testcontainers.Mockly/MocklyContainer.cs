using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Testcontainers.Mockly;

/// <summary>
/// A started Mockly container. Use <see cref="MocklyBuilder"/> to create and start instances.
/// </summary>
public sealed class MocklyContainer : DockerContainer
{
    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
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
    public Task AddMockAsync(Mock mock, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/mocks/http", mock, cancellationToken);

    /// <summary>Removes the mock with the specified <paramref name="id"/>.</summary>
    public Task DeleteMockAsync(string id, CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), $"/api/mocks/http/{Uri.EscapeDataString(id)}", cancellationToken);

    /// <summary>Clears all dynamic mocks, deactivates all scenarios, and removes any fault configuration.</summary>
    public Task ResetAsync(CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/reset", null, cancellationToken);

    /// <summary>Activates the scenario with the given <paramref name="scenarioId"/>.</summary>
    public Task ActivateScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/activate", null, cancellationToken);

    /// <summary>Deactivates the scenario with the given <paramref name="scenarioId"/>.</summary>
    public Task DeactivateScenarioAsync(string scenarioId, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), $"/api/scenarios/{Uri.EscapeDataString(scenarioId)}/deactivate", null, cancellationToken);

    /// <summary>Applies a fault configuration to the HTTP mock server.</summary>
    public Task SetFaultAsync(FaultConfig config, CancellationToken cancellationToken = default)
        => PostAsync(GetOrCreateHttpClient(), "/api/fault", config, cancellationToken);

    /// <summary>Removes any active fault configuration.</summary>
    public Task ClearFaultAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/fault", cancellationToken);

    /// <summary>Returns recent request log entries as a JSON string.</summary>
    public Task<string> GetLogsAsync(CancellationToken cancellationToken = default)
        => GetStringAsync(GetOrCreateHttpClient(), "/api/logs", cancellationToken);

    /// <summary>Clears all stored request log entries.</summary>
    public Task ClearLogsAsync(CancellationToken cancellationToken = default)
        => DeleteAsync(GetOrCreateHttpClient(), "/api/logs", cancellationToken);

    internal static async Task PostAsync(HttpClient http, string path, object? body, CancellationToken cancellationToken = default)
    {
        using HttpContent content = body != null
            ? new StringContent(JsonSerializer.Serialize(body, JsonOpts), Encoding.UTF8, "application/json")
            : new StringContent(string.Empty, Encoding.UTF8, "application/json");

        using var response = await http.PostAsync(path, content, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, $"Mockly API {path} failed", cancellationToken).ConfigureAwait(false);
    }

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

    private HttpClient GetOrCreateHttpClient()
    {
        _http ??= new HttpClient
        {
            BaseAddress = new Uri(GetApiBaseAddress()),
        };

        return _http;
    }

    private static async Task EnsureSuccessAsync(HttpResponseMessage response, string errorPrefix, CancellationToken cancellationToken)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        var message = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
        throw new HttpRequestException($"{errorPrefix} ({(int)response.StatusCode}): {message}");
    }
}
