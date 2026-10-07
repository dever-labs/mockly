package asyncapi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// maxInlineDepth bounds how deep inlineRefs will chase nested $refs before
// giving up and substituting an empty object. Every recursive call strictly
// increments depth, so this always terminates even on a self-referencing
// (recursive) schema.
const maxInlineDepth = 12

// resolvePointer resolves a local JSON Pointer ref (e.g.
// "#/components/schemas/Pet") against the parsed document root.
//
// Only local pointers (starting with "#/") are ever resolved. A spec is
// often downloaded from an untrusted source, so refs to other files or
// URLs are rejected outright rather than fetched — the same rationale the
// OpenAPI generator uses to disallow external $refs (SSRF / local file
// disclosure).
func resolvePointer(root map[string]any, ref string) (any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("external $ref %q is not allowed (spec may be untrusted)", ref)
	}
	tokens := strings.Split(ref[2:], "/")
	var cur any = root
	for _, tok := range tokens {
		tok = strings.ReplaceAll(tok, "~1", "/")
		tok = strings.ReplaceAll(tok, "~0", "~")
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[tok]
			if !ok {
				return nil, fmt.Errorf("$ref %q: no such key %q", ref, tok)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(tok)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("$ref %q: invalid array index %q", ref, tok)
			}
			cur = v[idx]
		default:
			return nil, fmt.Errorf("$ref %q: cannot descend into non-container at %q", ref, tok)
		}
	}
	return cur, nil
}

// inlineRefs walks node, replacing any {"$ref": "..."} object with the
// (recursively inlined) value it points to. External refs and refs that
// can't be resolved are replaced with an empty object and recorded as a
// warning rather than aborting the whole generation.
func inlineRefs(root map[string]any, node any, depth int, warnings *[]string) any {
	if depth > maxInlineDepth {
		return map[string]any{}
	}
	switch v := node.(type) {
	case map[string]any:
		if refVal, ok := v["$ref"]; ok {
			if refStr, ok := refVal.(string); ok {
				target, err := resolvePointer(root, refStr)
				if err != nil {
					*warnings = append(*warnings, err.Error())
					return map[string]any{}
				}
				return inlineRefs(root, target, depth+1, warnings)
			}
		}
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[k] = inlineRefs(root, val, depth+1, warnings)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			out[i] = inlineRefs(root, val, depth+1, warnings)
		}
		return out
	default:
		return v
	}
}

// asMap type-asserts v as a YAML/JSON object.
func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

// sortedKeys returns m's keys sorted, for deterministic output order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
