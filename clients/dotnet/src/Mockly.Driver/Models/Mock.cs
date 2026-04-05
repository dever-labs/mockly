using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record Mock(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("request")] MockRequest Request,
    [property: JsonPropertyName("response")] MockResponse Response);
