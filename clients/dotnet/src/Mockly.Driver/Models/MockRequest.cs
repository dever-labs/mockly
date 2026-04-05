using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record MockRequest(
    [property: JsonPropertyName("method")] string Method,
    [property: JsonPropertyName("path")] string Path,
    [property: JsonPropertyName("headers")] Dictionary<string, string>? Headers = null);
