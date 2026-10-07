// Package schemaexample synthesises a plausible example value from a JSON
// Schema (as parsed by kin-openapi's openapi3.Schema), shared by every spec
// generator (OpenAPI, AsyncAPI, ...) that needs to turn "here's a schema"
// into "here's a runnable mock body".
package schemaexample

import (
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// MaxDepth bounds recursive example generation so self-referencing or
// deeply nested schemas can't cause runaway recursion.
const MaxDepth = 8

// Generate synthesises a plausible JSON value for a schema: it prefers an
// explicit example/default/enum value, then recurses into object/array
// shapes, and otherwise fills in a type-appropriate placeholder
// (format-aware for strings).
func Generate(ref *openapi3.SchemaRef, depth int) any {
	if ref == nil || ref.Value == nil || depth > MaxDepth {
		return nil
	}
	schema := ref.Value

	if schema.Example != nil {
		return schema.Example
	}
	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}
	if schema.Default != nil {
		return schema.Default
	}

	if len(schema.AllOf) > 0 {
		merged := map[string]any{}
		hasObject := false
		var fallback any
		for _, sub := range schema.AllOf {
			v := Generate(sub, depth+1)
			if m, ok := v.(map[string]any); ok {
				for k, val := range m {
					merged[k] = val
				}
				hasObject = true
			} else if fallback == nil {
				fallback = v
			}
		}
		if hasObject {
			return merged
		}
		return fallback
	}
	if len(schema.OneOf) > 0 {
		return Generate(schema.OneOf[0], depth+1)
	}
	if len(schema.AnyOf) > 0 {
		return Generate(schema.AnyOf[0], depth+1)
	}

	typ := ""
	if schema.Type != nil && !schema.Type.IsEmpty() {
		typ = schema.Type.Slice()[0]
	} else if len(schema.Properties) > 0 {
		typ = "object"
	} else if schema.Items != nil {
		typ = "array"
	}

	switch typ {
	case "object":
		obj := map[string]any{}
		keys := make([]string, 0, len(schema.Properties))
		for k := range schema.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			obj[k] = Generate(schema.Properties[k], depth+1)
		}
		if len(obj) == 0 && schema.AdditionalProperties.Schema != nil {
			obj["key"] = Generate(schema.AdditionalProperties.Schema, depth+1)
		}
		return obj
	case "array":
		item := Generate(schema.Items, depth+1)
		return []any{item}
	case "string":
		return ExampleString(schema.Format)
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return true
	default:
		return nil
	}
}

// ExampleString returns a plausible placeholder string for a JSON Schema
// "format" hint (date-time, email, uuid, ...), or the literal "string" when
// the format isn't recognised.
func ExampleString(format string) string {
	switch format {
	case "date-time":
		return "2024-01-01T00:00:00Z"
	case "date":
		return "2024-01-01"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "uri", "url", "hostname":
		return "https://example.com"
	case "byte":
		return "ZXhhbXBsZQ=="
	case "password":
		return "********" //nolint:gosec // placeholder text, not a real secret
	default:
		return "string"
	}
}
