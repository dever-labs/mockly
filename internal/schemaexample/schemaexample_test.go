package schemaexample

import (
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func ref(s *openapi3.Schema) *openapi3.SchemaRef {
	return openapi3.NewSchemaRef("", s)
}

func TestGenerate_NilAndDepthGuards(t *testing.T) {
	if got := Generate(nil, 0); got != nil {
		t.Errorf("Generate(nil, 0) = %v, want nil", got)
	}
	if got := Generate(&openapi3.SchemaRef{}, 0); got != nil {
		t.Errorf("Generate(&SchemaRef{}, 0) = %v, want nil (nil Value)", got)
	}
	if got := Generate(ref(openapi3.NewStringSchema()), MaxDepth+1); got != nil {
		t.Errorf("Generate at depth > MaxDepth = %v, want nil", got)
	}
}

func TestGenerate_PrefersExampleOverEnumOverDefault(t *testing.T) {
	s := openapi3.NewStringSchema()
	s.Example = "explicit-example"
	s.WithEnum("a", "b")
	s.WithDefault("fallback")
	if got := Generate(ref(s), 0); got != "explicit-example" {
		t.Errorf("Generate = %v, want explicit example to take priority", got)
	}

	s2 := openapi3.NewStringSchema()
	s2.WithEnum("first", "second")
	s2.WithDefault("fallback")
	if got := Generate(ref(s2), 0); got != "first" {
		t.Errorf("Generate = %v, want first enum value", got)
	}

	s3 := openapi3.NewStringSchema()
	s3.WithDefault("the-default")
	if got := Generate(ref(s3), 0); got != "the-default" {
		t.Errorf("Generate = %v, want default value", got)
	}
}

func TestGenerate_AllOfMergesObjectBranches(t *testing.T) {
	base := openapi3.NewObjectSchema().WithProperty("id", openapi3.NewStringSchema())
	extra := openapi3.NewObjectSchema().WithProperty("name", openapi3.NewStringSchema())
	merged := openapi3.NewAllOfSchema(base, extra)

	got, ok := Generate(ref(merged), 0).(map[string]any)
	if !ok {
		t.Fatalf("Generate = %T, want map[string]any", Generate(ref(merged), 0))
	}
	if _, ok := got["id"]; !ok {
		t.Errorf("merged object missing 'id' key: %+v", got)
	}
	if _, ok := got["name"]; !ok {
		t.Errorf("merged object missing 'name' key: %+v", got)
	}
}

func TestGenerate_AllOfFallsBackToNonObjectBranch(t *testing.T) {
	allOf := openapi3.NewAllOfSchema(openapi3.NewStringSchema())
	if got := Generate(ref(allOf), 0); got != "string" {
		t.Errorf("Generate = %v, want fallback non-object branch", got)
	}
}

func TestGenerate_OneOfUsesFirstBranch(t *testing.T) {
	s := openapi3.NewOneOfSchema(openapi3.NewIntegerSchema(), openapi3.NewStringSchema())
	if got := Generate(ref(s), 0); got != 0 {
		t.Errorf("Generate = %v, want first oneOf branch (integer 0)", got)
	}
}

func TestGenerate_AnyOfUsesFirstBranch(t *testing.T) {
	s := openapi3.NewAnyOfSchema(openapi3.NewBoolSchema(), openapi3.NewStringSchema())
	if got := Generate(ref(s), 0); got != true {
		t.Errorf("Generate = %v, want first anyOf branch (bool true)", got)
	}
}

func TestGenerate_ObjectWithPropertiesSortedKeys(t *testing.T) {
	s := openapi3.NewObjectSchema().
		WithProperty("zeta", openapi3.NewStringSchema()).
		WithProperty("alpha", openapi3.NewIntegerSchema())

	got, ok := Generate(ref(s), 0).(map[string]any)
	if !ok {
		t.Fatalf("Generate = %T, want map[string]any", Generate(ref(s), 0))
	}
	if got["alpha"] != 0 {
		t.Errorf("got['alpha'] = %v, want 0", got["alpha"])
	}
	if got["zeta"] != "string" {
		t.Errorf("got['zeta'] = %v, want \"string\"", got["zeta"])
	}
}

func TestGenerate_ObjectWithAdditionalPropertiesOnly(t *testing.T) {
	s := openapi3.NewObjectSchema().WithAdditionalProperties(openapi3.NewStringSchema())

	got, ok := Generate(ref(s), 0).(map[string]any)
	if !ok {
		t.Fatalf("Generate = %T, want map[string]any", Generate(ref(s), 0))
	}
	if got["key"] != "string" {
		t.Errorf("got['key'] = %v, want \"string\" placeholder", got["key"])
	}
}

func TestGenerate_ArrayType(t *testing.T) {
	s := openapi3.NewArraySchema().WithItems(openapi3.NewIntegerSchema())
	got := Generate(ref(s), 0)
	want := []any{0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Generate = %#v, want %#v", got, want)
	}
}

func TestGenerate_ScalarTypes(t *testing.T) {
	cases := []struct {
		name   string
		schema *openapi3.Schema
		want   any
	}{
		{"integer", openapi3.NewIntegerSchema(), 0},
		{"number", openapi3.NewFloat64Schema(), 0.0},
		{"boolean", openapi3.NewBoolSchema(), true},
		{"string-default", openapi3.NewStringSchema(), "string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Generate(ref(tc.schema), 0); got != tc.want {
				t.Errorf("Generate(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestGenerate_InferredObjectAndArrayWithoutExplicitType(t *testing.T) {
	// Schema.Type is nil/empty but Properties is set: should be treated as "object".
	obj := &openapi3.Schema{Properties: openapi3.Schemas{"a": ref(openapi3.NewStringSchema())}}
	got, ok := Generate(ref(obj), 0).(map[string]any)
	if !ok {
		t.Fatalf("Generate = %T, want map[string]any", Generate(ref(obj), 0))
	}
	if got["a"] != "string" {
		t.Errorf("got['a'] = %v, want \"string\"", got["a"])
	}

	// Schema.Type is nil/empty but Items is set: should be treated as "array".
	arr := &openapi3.Schema{Items: ref(openapi3.NewIntegerSchema())}
	gotArr := Generate(ref(arr), 0)
	if !reflect.DeepEqual(gotArr, []any{0}) {
		t.Errorf("Generate = %#v, want []any{0}", gotArr)
	}
}

func TestGenerate_UnknownTypeReturnsNil(t *testing.T) {
	// No Type, no Properties, no Items -> falls through to the default case.
	s := &openapi3.Schema{}
	if got := Generate(ref(s), 0); got != nil {
		t.Errorf("Generate = %v, want nil for an empty/unrecognised schema", got)
	}
}

func TestExampleString(t *testing.T) {
	cases := map[string]string{
		"date-time": "2024-01-01T00:00:00Z",
		"date":      "2024-01-01",
		"email":     "user@example.com",
		"uuid":      "00000000-0000-0000-0000-000000000000",
		"uri":       "https://example.com",
		"url":       "https://example.com",
		"hostname":  "https://example.com",
		"byte":      "ZXhhbXBsZQ==",
		"password":  "********",
		"":          "string",
		"unknown":   "string",
	}
	for format, want := range cases {
		if got := ExampleString(format); got != want {
			t.Errorf("ExampleString(%q) = %q, want %q", format, got, want)
		}
	}
}

func TestGenerate_StringUsesFormatAwarePlaceholder(t *testing.T) {
	s := openapi3.NewStringSchema().WithFormat("uuid")
	if got := Generate(ref(s), 0); got != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("Generate = %v, want format-aware uuid placeholder", got)
	}
}
