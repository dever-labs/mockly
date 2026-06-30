using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record FaultConfig(
    [property: JsonPropertyName("enabled")] bool Enabled,
    [property: JsonPropertyName("delay")] string? Delay = null,
    [property: JsonPropertyName("status")] int? Status = null,
    [property: JsonPropertyName("error_rate")] double? ErrorRate = null);
