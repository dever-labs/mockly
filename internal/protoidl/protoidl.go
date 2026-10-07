// Package protoidl parses a local Protobuf (.proto) service definition and
// derives a ready-to-run Mockly gRPC config from it — one mock per unary RPC
// method, with the response body synthesised from the method's output
// message type.
//
// Parsing uses github.com/bufbuild/protocompile, a pure-Go proto2/proto3
// compiler (no protoc binary required). Imports are resolved only from the
// spec file's own directory (no absolute paths, no "../" traversal out of
// it) plus the standard google/protobuf/*.proto well-known types, which
// protocompile bundles; anything else is a hard error, same policy as the
// OpenAPI generator's handling of external $refs.
//
// Mockly's gRPC mock server is schema-agnostic at runtime (it matches calls
// by method name only and replies with freeform JSON — see
// internal/protocols/grpcserver), so the .proto file itself is never needed
// once a config has been generated; it's used here purely to synthesise
// plausible example responses.
//
// Only unary methods are mocked: Mockly's gRPC server reads at most one
// request message and writes at most one response message per call, so
// client-streaming, server-streaming and bidirectional-streaming methods
// have no usable Mockly equivalent and are skipped with a warning. Likewise,
// if two services in the same file declare a method with the same name, only
// the first is kept — Mockly matches gRPC calls by method name alone (not
// service), so a later duplicate would just shadow the first and never be
// reachable.
package protoidl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/schemaexample"
)

// maxMessageDepth bounds recursive example generation so a self-referencing
// message type (e.g. a Tree with repeated Tree children) can't cause runaway
// recursion.
const maxMessageDepth = 8

// Result holds the gRPC mocks derived from a .proto file, plus any
// diagnostics about RPCs/fields that couldn't be faithfully represented.
type Result struct {
	Mocks    []config.GRPCMock
	Warnings []string
}

// Empty reports whether no mocks could be derived at all.
func (r Result) Empty() bool { return len(r.Mocks) == 0 }

var syntaxRE = regexp.MustCompile(`(?m)^\s*syntax\s*=\s*"proto[23]"\s*;`)

// IsProtoIDL reports whether the file at specPath looks like a Protobuf IDL
// document (as opposed to an OpenAPI/AsyncAPI YAML/JSON spec), so the
// unified `mockly generate` CLI command can pick the right generator
// automatically. It trusts the ".proto" extension outright, and otherwise
// sniffs the first few KB of the file for a `syntax = "proto2|3";`
// declaration.
func IsProtoIDL(specPath string) (bool, error) {
	if strings.EqualFold(filepath.Ext(specPath), ".proto") {
		return true, nil
	}
	data, err := os.ReadFile(specPath) //nolint:gosec // local CLI arg, not a server-side path
	if err != nil {
		return false, fmt.Errorf("reading %q: %w", specPath, err)
	}
	if len(data) > 4096 {
		data = data[:4096]
	}
	return syntaxRE.Match(data), nil
}

// Generate parses the .proto file at specPath and derives one GRPCMock per
// unary RPC method declared by its service(s).
func Generate(specPath string) (Result, error) {
	root := filepath.Dir(specPath)
	filename := filepath.Base(specPath)

	resolver := protocompile.WithStandardImports(protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
		clean := filepath.Clean(path)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, string(filepath.Separator)+"..") {
			return protocompile.SearchResult{}, fmt.Errorf("protoidl: rejected import %q: only imports local to the spec file's own directory are supported", path)
		}
		data, err := os.ReadFile(filepath.Join(root, clean)) //nolint:gosec // local CLI arg, not a server-side path
		if err != nil {
			return protocompile.SearchResult{}, err
		}
		return protocompile.SearchResult{Source: strings.NewReader(string(data))}, nil
	}))

	compiler := protocompile.Compiler{
		Resolver:       resolver,
		SourceInfoMode: protocompile.SourceInfoNone,
	}
	files, err := compiler.Compile(context.Background(), filename)
	if err != nil {
		return Result{}, fmt.Errorf("parsing %q: %w", specPath, err)
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("parsing %q: no file compiled", specPath)
	}
	fd := files[0]

	var res Result
	seenMethod := map[string]bool{}
	usedIDs := map[string]bool{}
	svcs := fd.Services()
	for i := 0; i < svcs.Len(); i++ {
		svc := svcs.Get(i)
		methods := svc.Methods()
		for j := 0; j < methods.Len(); j++ {
			method := methods.Get(j)
			name := string(method.Name())

			if method.IsStreamingClient() || method.IsStreamingServer() {
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"%s.%s: streaming RPC has no Mockly equivalent (gRPC mocks only support unary request/response), skipped",
					svc.FullName(), name))
				continue
			}
			if seenMethod[name] {
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"%s.%s: method name %q already used by another service in this spec; Mockly matches gRPC calls by method name only (not service), so this duplicate would never be reached, skipped",
					svc.FullName(), name, name))
				continue
			}
			seenMethod[name] = true

			response := buildMessageFieldsExample(method.Output(), 0, &res.Warnings)
			res.Mocks = append(res.Mocks, config.GRPCMock{
				ID:       uniqueID(usedIDs, slugify(string(svc.Name())+"-"+name)),
				Method:   name,
				Response: response,
			})
		}
	}

	return res, nil
}

// buildFieldExample synthesises a plausible JSON-compatible example value
// for a single field, honouring repeated/map shape.
func buildFieldExample(f protoreflect.FieldDescriptor, depth int, warnings *[]string) any {
	switch {
	case f.IsMap():
		return map[string]any{"key1": buildScalarExample(f.MapValue(), depth+1, warnings)}
	case f.IsList():
		return []any{buildScalarExample(f, depth, warnings)}
	default:
		return buildScalarExample(f, depth, warnings)
	}
}

// buildScalarExample synthesises an example for a single (non-repeated,
// non-map) field value, per the standard proto3 JSON mapping: 64-bit integer
// kinds are represented as strings (to avoid precision loss in JSON
// numbers), everything else maps to its natural JSON type.
func buildScalarExample(f protoreflect.FieldDescriptor, depth int, warnings *[]string) any {
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return buildMessageOrWellKnownExample(f.Message(), depth, warnings)
	case protoreflect.EnumKind:
		vals := f.Enum().Values()
		if vals.Len() == 0 {
			return ""
		}
		return string(vals.Get(0).Name())
	case protoreflect.BoolKind:
		return true
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return schemaexample.ExampleString("byte")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "0"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return 0.0
	default: // Int32Kind, Sint32Kind, Sfixed32Kind, Uint32Kind, Fixed32Kind
		return 0
	}
}

// buildMessageOrWellKnownExample returns a scalar placeholder for the
// handful of google.protobuf well-known types whose JSON mapping isn't a
// plain object (Timestamp, Duration, the Struct/Value family, wrapper
// types, ...), or otherwise recurses into the message's own fields.
func buildMessageOrWellKnownExample(msg protoreflect.MessageDescriptor, depth int, warnings *[]string) any {
	if depth > maxMessageDepth {
		return map[string]any{}
	}
	switch string(msg.FullName()) {
	case "google.protobuf.Timestamp":
		return "2024-01-01T00:00:00Z"
	case "google.protobuf.Duration":
		return "0s"
	case "google.protobuf.Empty":
		return map[string]any{}
	case "google.protobuf.StringValue":
		return "string"
	case "google.protobuf.BytesValue":
		return schemaexample.ExampleString("byte")
	case "google.protobuf.BoolValue":
		return true
	case "google.protobuf.Int32Value", "google.protobuf.UInt32Value":
		return 0
	case "google.protobuf.Int64Value", "google.protobuf.UInt64Value":
		return "0"
	case "google.protobuf.FloatValue", "google.protobuf.DoubleValue":
		return 0.0
	case "google.protobuf.FieldMask":
		return ""
	case "google.protobuf.Struct":
		appendWarningOnce(warnings, "google.protobuf.Struct field(s) approximated as an empty object; fill in manually")
		return map[string]any{}
	case "google.protobuf.Value":
		appendWarningOnce(warnings, "google.protobuf.Value field(s) approximated as null; fill in manually")
		return nil
	case "google.protobuf.ListValue":
		appendWarningOnce(warnings, "google.protobuf.ListValue field(s) approximated as an empty array; fill in manually")
		return []any{}
	case "google.protobuf.Any":
		appendWarningOnce(warnings, `google.protobuf.Any field(s) approximated as an empty object; fill in "@type" and its fields manually`)
		return map[string]any{}
	default:
		return buildMessageFieldsExample(msg, depth+1, warnings)
	}
}

// buildMessageFieldsExample builds a map of field name -> example value for
// an ordinary (non-well-known) message type. Fields are visited in sorted
// name order for deterministic output. Only one representative field per
// real "oneof" group is included (a real protobuf message can only ever
// have one member of a oneof set) — proto3 "optional" scalar fields use a
// *synthetic* oneof under the hood and are deliberately excluded from that
// rule, so they still all appear.
func buildMessageFieldsExample(msg protoreflect.MessageDescriptor, depth int, warnings *[]string) map[string]any {
	fields := msg.Fields()
	byName := make(map[string]protoreflect.FieldDescriptor, fields.Len())
	names := make([]string, 0, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		n := string(f.Name())
		byName[n] = f
		names = append(names, n)
	}
	sort.Strings(names)

	obj := map[string]any{}
	seenOneof := map[protoreflect.Name]bool{}
	for _, n := range names {
		f := byName[n]
		if oo := f.ContainingOneof(); oo != nil && !oo.IsSynthetic() {
			if seenOneof[oo.Name()] {
				continue
			}
			seenOneof[oo.Name()] = true
		}
		obj[n] = buildFieldExample(f, depth, warnings)
	}
	return obj
}

func appendWarningOnce(warnings *[]string, msg string) {
	for _, w := range *warnings {
		if w == msg {
			return
		}
	}
	*warnings = append(*warnings, msg)
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = slugRE.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}

func uniqueID(usedIDs map[string]bool, base string) string {
	id := base
	if id == "" {
		id = "mock"
	}
	for n := 2; usedIDs[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	usedIDs[id] = true
	return id
}
