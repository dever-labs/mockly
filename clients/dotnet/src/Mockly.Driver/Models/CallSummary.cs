using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace Mockly.Driver.Models;

/// <summary>Summary of recorded calls for a specific HTTP mock.</summary>
public record CallSummary(
    [property: JsonPropertyName("mock_id")] string MockId,
    [property: JsonPropertyName("count")]   long   Count,
    [property: JsonPropertyName("calls")]   List<CallEntry> Calls);
