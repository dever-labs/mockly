using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

/// <summary>A single recorded HTTP request captured by the Mockly server.</summary>
public record CallEntry(
    [property: JsonPropertyName("id")]          string Id,
    [property: JsonPropertyName("timestamp")]   string Timestamp,
    [property: JsonPropertyName("protocol")]    string Protocol,
    [property: JsonPropertyName("path")]        string Path,
    [property: JsonPropertyName("duration_ms")] long   DurationMs)
{
    [JsonPropertyName("method")]      public string?                     Method     { get; init; }
    [JsonPropertyName("status")]      public int?                        Status     { get; init; }
    [JsonPropertyName("headers")]     public Dictionary<string, string>? Headers    { get; init; }
    [JsonPropertyName("body")]        public string?                     Body       { get; init; }
    [JsonPropertyName("matched_id")]  public string?                     MatchedId  { get; init; }
    [JsonPropertyName("path_params")] public Dictionary<string, string>? PathParams { get; init; }
}
