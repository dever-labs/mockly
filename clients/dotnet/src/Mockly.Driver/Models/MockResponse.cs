using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record MockResponse(
    [property: JsonPropertyName("status")] int Status,
    [property: JsonPropertyName("body")] string? Body = null,
    [property: JsonPropertyName("headers")] Dictionary<string, string>? Headers = null,
    [property: JsonPropertyName("delay")] string? Delay = null);
