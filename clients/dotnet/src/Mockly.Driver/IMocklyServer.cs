using Mockly.Driver.Models;

namespace Mockly.Driver;

/// <summary>Contract for interacting with a running Mockly server.</summary>
public interface IMocklyServer
{
    /// <summary>Registers a new HTTP mock.</summary>
    Task AddMockAsync(Mock mock);

    /// <summary>Returns all registered HTTP mocks.</summary>
    Task<IReadOnlyList<Mock>> ListMocksAsync();

    /// <summary>Replaces the mock with the specified <paramref name="id"/>.</summary>
    Task<Mock> UpdateMockAsync(string id, Mock mock);

    /// <summary>Partially updates the response of the mock with the specified <paramref name="id"/>.</summary>
    Task<Mock> PatchMockAsync(string id, MockResponsePatch patch);

    /// <summary>Removes the mock with the specified <paramref name="id"/>.</summary>
    Task DeleteMockAsync(string id);

    /// <summary>Returns all key-value entries in the state store.</summary>
    Task<Dictionary<string, string>> GetStateAsync();

    /// <summary>Merges <paramref name="kvMap"/> into the state store, adding or overwriting the given keys.</summary>
    Task<Dictionary<string, string>> SetStateAsync(Dictionary<string, string> kvMap);

    /// <summary>Removes the state entry with the specified <paramref name="key"/>.</summary>
    Task DeleteStateAsync(string key);

    /// <summary>Returns recent request log entries, optionally filtered by <paramref name="matchedId"/>.</summary>
    Task<IReadOnlyList<CallEntry>> GetLogsAsync(string? matchedId = null);

    /// <summary>Clears all stored request log entries.</summary>
    Task ClearLogsAsync();

    /// <summary>Returns the number of logged requests, optionally filtered by <paramref name="matchedId"/>.</summary>
    Task<int> GetLogsCountAsync(string? matchedId = null);

    /// <summary>Returns all registered scenarios.</summary>
    Task<IReadOnlyList<Scenario>> ListScenariosAsync();

    /// <summary>Creates a new scenario.</summary>
    Task<Scenario> CreateScenarioAsync(Scenario scenario);

    /// <summary>Returns the scenario with the specified <paramref name="scenarioId"/>.</summary>
    Task<Scenario> GetScenarioAsync(string scenarioId);

    /// <summary>Replaces the scenario with the specified <paramref name="scenarioId"/>.</summary>
    Task<Scenario> UpdateScenarioAsync(string scenarioId, Scenario scenario);

    /// <summary>Removes the scenario with the specified <paramref name="scenarioId"/>.</summary>
    Task DeleteScenarioAsync(string scenarioId);

    /// <summary>Returns all currently active scenarios.</summary>
    Task<ActiveScenariosResponse> ListActiveScenariosAsync();

    /// <summary>Clears all dynamic mocks, deactivates all scenarios, and removes any fault configuration.</summary>
    Task ResetAsync();

    /// <summary>Activates the scenario with the given <paramref name="scenarioId"/>.</summary>
    Task ActivateScenarioAsync(string scenarioId);

    /// <summary>Deactivates the scenario with the given <paramref name="scenarioId"/>.</summary>
    Task DeactivateScenarioAsync(string scenarioId);

    /// <summary>Applies a fault configuration to the HTTP mock server.</summary>
    Task SetFaultAsync(FaultConfig config);

    /// <summary>Removes any active fault configuration.</summary>
    Task ClearFaultAsync();

    /// <summary>Returns the call history for the mock with the specified <paramref name="mockId"/>.</summary>
    Task<CallSummary> GetCallsAsync(string mockId);

    /// <summary>Clears the call history for the mock with the specified <paramref name="mockId"/>.</summary>
    Task ClearCallsAsync(string mockId);

    /// <summary>Clears the call history for all mocks.</summary>
    Task ClearAllCallsAsync();

    /// <summary>Blocks until the mock with <paramref name="mockId"/> has been called at least <paramref name="count"/> times, or until <paramref name="timeout"/> expires.</summary>
    Task<CallSummary> WaitForCallsAsync(string mockId, int count = 1, TimeSpan? timeout = null);
}
