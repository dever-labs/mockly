using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record MockResponsePatch(
    [property: JsonPropertyName("status")] int? Status = null,
    [property: JsonPropertyName("body")] string? Body = null,
    [property: JsonPropertyName("headers")] Dictionary<string, string>? Headers = null,
    [property: JsonPropertyName("delay")] string? Delay = null);
