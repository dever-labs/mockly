using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record FaultConfig(
    [property: JsonPropertyName("enabled")] bool Enabled,
    [property: JsonPropertyName("delay")] string? Delay = null,
    [property: JsonPropertyName("status_override")] int? StatusOverride = null,
    [property: JsonPropertyName("error_rate")] double? ErrorRate = null);
