using Mockly.Driver.Models;

namespace Mockly.Driver;

public interface IMocklyServer
{
    Task AddMockAsync(Mock mock);
    Task<IReadOnlyList<Mock>> ListMocksAsync();
    Task<Mock> UpdateMockAsync(string id, Mock mock);
    Task<Mock> PatchMockAsync(string id, MockResponsePatch patch);
    Task DeleteMockAsync(string id);
    Task<Dictionary<string, string>> GetStateAsync();
    Task<Dictionary<string, string>> SetStateAsync(Dictionary<string, string> kvMap);
    Task DeleteStateAsync(string key);
    Task<IReadOnlyList<CallEntry>> GetLogsAsync(string? matchedId = null);
    Task ClearLogsAsync();
    Task<int> GetLogsCountAsync(string? matchedId = null);
    Task<IReadOnlyList<Scenario>> ListScenariosAsync();
    Task<Scenario> CreateScenarioAsync(Scenario scenario);
    Task<Scenario> GetScenarioAsync(string scenarioId);
    Task<Scenario> UpdateScenarioAsync(string scenarioId, Scenario scenario);
    Task DeleteScenarioAsync(string scenarioId);
    Task<ActiveScenariosResponse> ListActiveScenariosAsync();
    Task ResetAsync();
    Task ActivateScenarioAsync(string scenarioId);
    Task DeactivateScenarioAsync(string scenarioId);
    Task SetFaultAsync(FaultConfig config);
    Task ClearFaultAsync();
    Task<CallSummary> GetCallsAsync(string mockId);
    Task ClearCallsAsync(string mockId);
    Task ClearAllCallsAsync();
    Task<CallSummary> WaitForCallsAsync(string mockId, int count = 1, TimeSpan? timeout = null);
}
