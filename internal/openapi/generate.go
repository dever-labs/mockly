// Package openapi generates a starter set of HTTP mocks from an OpenAPI 3.x
// document, so a consumer can point `mockly generate openapi` at a real
// spec and get a runnable config instead of hand-writing every mock.
//
// For every operation (path + method) it picks a representative response
// (preferring 2xx status codes) and derives a response body: it uses the
// response's example/examples when present, falling back to a plausible
// value synthesised from the JSON schema (strings, numbers, booleans,
// objects and arrays, including resolved $ref, allOf/oneOf/anyOf, enum and
// format hints). Operations with no usable response are skipped with a
// warning rather than failing the whole generation.
package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/dever-labs/mockly/internal/config"
)

// maxSchemaDepth bounds recursive example generation so self-referencing or
// deeply nested schemas can't cause runaway recursion.
const maxSchemaDepth = 8

// httpMethods are the operations recognised on a PathItem, in the order
// they're emitted for a given path (for deterministic output).
var httpMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE"}

// Result is the outcome of generating mocks from an OpenAPI document.
type Result struct {
	Mocks    []config.HTTPMock
	Warnings []string
}

// Generate parses the OpenAPI 3.x document at specPath (YAML or JSON, local
// file) and returns one HTTP mock per operation it could derive a response
// for.
func Generate(specPath string) (*Result, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec %q: %w", specPath, err)
	}

	res := &Result{}
	if doc.Paths == nil {
		return res, nil
	}

	paths := doc.Paths.Map()
	sortedPaths := make([]string, 0, len(paths))
	for p := range paths {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	usedIDs := map[string]int{}

	for _, p := range sortedPaths {
		item := paths[p]
		if item == nil {
			continue
		}
		ops := item.Operations()
		for _, method := range httpMethods {
			op, ok := ops[method]
			if !ok || op == nil {
				continue
			}
			mock, warn := generateMock(method, p, op)
			if warn != "" {
				res.Warnings = append(res.Warnings, warn)
			}
			if mock == nil {
				continue
			}
			mock.ID = uniqueID(usedIDs, mock.ID)
			res.Mocks = append(res.Mocks, *mock)
		}
	}

	return res, nil
}

// uniqueID appends a numeric suffix to id if it was already used, so
// generated mocks never collide (e.g. two operations with the same
// sanitised operationId).
func uniqueID(used map[string]int, id string) string {
	n := used[id]
	used[id]++
	if n == 0 {
		return id
	}
	return fmt.Sprintf("%s-%d", id, n+1)
}

func generateMock(method, path string, op *openapi3.Operation) (*config.HTTPMock, string) {
	id := mockID(method, path, op)

	status, resp := pickResponse(op.Responses)
	if resp == nil {
		return nil, fmt.Sprintf("%s %s: no response defined, skipped", method, path)
	}

	headers := map[string]string{}
	var body string
	var warning string

	if ct, mt := pickMediaType(resp.Content); mt != nil {
		if isTextualContentType(ct) {
			value, err := exampleValue(mt)
			if err != nil {
				warning = fmt.Sprintf("%s %s: %v", method, path, err)
			} else if value != nil {
				encoded, err := json.MarshalIndent(value, "", "  ")
				if err != nil {
					warning = fmt.Sprintf("%s %s: encoding example body: %v", method, path, err)
				} else {
					body = string(encoded)
				}
			}
			headers["Content-Type"] = ct
		} else {
			warning = fmt.Sprintf("%s %s: response content-type %q isn't textual, body left empty", method, path, ct)
			headers["Content-Type"] = ct
		}
	}

	return &config.HTTPMock{
		ID:      id,
		Request: config.HTTPRequest{Method: method, Path: path},
		Response: config.HTTPResponse{
			Status:  status,
			Headers: headers,
			Body:    body,
		},
	}, warning
}

// mockID derives a stable, readable mock ID from the operation's
// operationId when present, otherwise from the method and path.
func mockID(method, path string, op *openapi3.Operation) string {
	base := op.OperationID
	if base == "" {
		base = method + " " + path
	}
	return slugify(base)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "mock"
	}
	return out
}

// pickResponse chooses the response to derive a mock from, preferring (in
// order) 200/201/202/204, any other 2xx, the "default" response, and
// finally whatever the lowest-sorted status code is.
func pickResponse(responses *openapi3.Responses) (int, *openapi3.Response) {
	if responses == nil {
		return 0, nil
	}
	m := responses.Map()
	if len(m) == 0 {
		return 0, nil
	}

	for _, preferred := range []string{"200", "201", "202", "204"} {
		if ref, ok := m[preferred]; ok && ref != nil && ref.Value != nil {
			code, _ := strconv.Atoi(preferred)
			return code, ref.Value
		}
	}

	var codes []string
	for k := range m {
		if len(k) == 3 && k[0] == '2' {
			codes = append(codes, k)
		}
	}
	sort.Strings(codes)
	if len(codes) > 0 {
		ref := m[codes[0]]
		if ref != nil && ref.Value != nil {
			code, _ := strconv.Atoi(codes[0])
			return code, ref.Value
		}
	}

	if ref, ok := m["default"]; ok && ref != nil && ref.Value != nil {
		return 200, ref.Value
	}

	var all []string
	for k := range m {
		all = append(all, k)
	}
	sort.Strings(all)
	ref := m[all[0]]
	if ref == nil || ref.Value == nil {
		return 0, nil
	}
	code, err := strconv.Atoi(all[0])
	if err != nil {
		code = 200
	}
	return code, ref.Value
}

// pickMediaType chooses application/json (or the first +json/json-like
// type), falling back to the first media type in the content map.
func pickMediaType(content openapi3.Content) (string, *openapi3.MediaType) {
	if len(content) == 0 {
		return "", nil
	}
	if mt, ok := content["application/json"]; ok {
		return "application/json", mt
	}
	var keys []string
	for k := range content {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.Contains(k, "json") {
			return k, content[k]
		}
	}
	return keys[0], content[keys[0]]
}

func isTextualContentType(ct string) bool {
	switch {
	case strings.Contains(ct, "json"),
		strings.Contains(ct, "xml"),
		strings.HasPrefix(ct, "text/"),
		strings.Contains(ct, "x-www-form-urlencoded"):
		return true
	default:
		return false
	}
}

// exampleValue returns the best example for a media type: an explicit
// example, the first of multiple named examples, or one synthesised from
// the schema.
func exampleValue(mt *openapi3.MediaType) (any, error) {
	if mt.Example != nil {
		return mt.Example, nil
	}
	if len(mt.Examples) > 0 {
		var keys []string
		for k := range mt.Examples {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if ex := mt.Examples[keys[0]]; ex != nil && ex.Value != nil {
			return ex.Value.Value, nil
		}
	}
	if mt.Schema != nil {
		return generateExample(mt.Schema, 0), nil
	}
	return nil, nil
}

// generateExample synthesises a plausible JSON value for a schema: it
// prefers an explicit example/default/enum value, then recurses into
// object/array shapes, and otherwise fills in a type-appropriate
// placeholder (format-aware for strings).
func generateExample(ref *openapi3.SchemaRef, depth int) any {
	if ref == nil || ref.Value == nil || depth > maxSchemaDepth {
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
			v := generateExample(sub, depth+1)
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
		return generateExample(schema.OneOf[0], depth+1)
	}
	if len(schema.AnyOf) > 0 {
		return generateExample(schema.AnyOf[0], depth+1)
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
			obj[k] = generateExample(schema.Properties[k], depth+1)
		}
		if len(obj) == 0 && schema.AdditionalProperties.Schema != nil {
			obj["key"] = generateExample(schema.AdditionalProperties.Schema, depth+1)
		}
		return obj
	case "array":
		item := generateExample(schema.Items, depth+1)
		return []any{item}
	case "string":
		return exampleString(schema.Format)
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

func exampleString(format string) string {
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
