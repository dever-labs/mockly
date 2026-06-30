using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

public record ActiveScenariosResponse(
    [property: JsonPropertyName("active")] IReadOnlyList<string> Active,
    [property: JsonPropertyName("scenarios")] IReadOnlyList<Scenario> Scenarios);
