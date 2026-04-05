using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record Scenario(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("patches")] IReadOnlyList<ScenarioPatch> Patches);
