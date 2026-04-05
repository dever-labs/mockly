using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record ScenarioPatch(
    [property: JsonPropertyName("mock_id")] string MockId,
    [property: JsonPropertyName("status")] int? Status = null,
    [property: JsonPropertyName("body")] string? Body = null,
    [property: JsonPropertyName("delay")] string? Delay = null);
