package protoidl_test

import (
	"strings"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/protoidl"
)

func TestIsProtoIDL(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"testdata/greeter.proto", true},
		{"testdata/not-proto.yaml", false},
	}
	for _, c := range cases {
		got, err := protoidl.IsProtoIDL(c.path)
		if err != nil {
			t.Fatalf("IsProtoIDL(%q): %v", c.path, err)
		}
		if got != c.want {
			t.Errorf("IsProtoIDL(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestGenerate_Greeter(t *testing.T) {
	res, err := protoidl.Generate("testdata/greeter.proto")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(res.Mocks) != 4 {
		t.Fatalf("Mocks = %d, want 4 (SayHello unary; ListHellos/CollectHellos/ChatHellos streaming)", len(res.Mocks))
	}
	mock := res.Mocks[0]
	if mock.Method != "SayHello" {
		t.Errorf("Method = %q, want SayHello", mock.Method)
	}

	findMock := func(method string) *config.GRPCMock {
		for i := range res.Mocks {
			if res.Mocks[i].Method == method {
				return &res.Mocks[i]
			}
		}
		return nil
	}

	listHellos := findMock("ListHellos")
	if listHellos == nil {
		t.Fatalf("mocks = %+v, want a ListHellos mock", res.Mocks)
	}
	if len(listHellos.Stream) != 1 {
		t.Errorf("ListHellos.Stream = %d messages, want 1", len(listHellos.Stream))
	}
	if listHellos.ClientStream {
		t.Errorf("ListHellos.ClientStream = true, want false (server-streaming only)")
	}
	if listHellos.Response != nil {
		t.Errorf("ListHellos.Response = %v, want nil (response lives in Stream)", listHellos.Response)
	}

	collectHellos := findMock("CollectHellos")
	if collectHellos == nil {
		t.Fatalf("mocks = %+v, want a CollectHellos mock", res.Mocks)
	}
	if !collectHellos.ClientStream {
		t.Errorf("CollectHellos.ClientStream = false, want true (client-streaming)")
	}
	if len(collectHellos.Stream) != 0 {
		t.Errorf("CollectHellos.Stream = %d messages, want 0 (reply lives in Response)", len(collectHellos.Stream))
	}
	if collectHellos.Response == nil {
		t.Errorf("CollectHellos.Response = nil, want the example reply")
	}

	chatHellos := findMock("ChatHellos")
	if chatHellos == nil {
		t.Fatalf("mocks = %+v, want a ChatHellos mock", res.Mocks)
	}
	if !chatHellos.ClientStream {
		t.Errorf("ChatHellos.ClientStream = false, want true (bidi-streaming)")
	}
	if len(chatHellos.Stream) != 1 {
		t.Errorf("ChatHellos.Stream = %d messages, want 1", len(chatHellos.Stream))
	}

	foundStreamingWarning := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "ListHellos") && strings.Contains(w, "server-streaming") {
			foundStreamingWarning = true
		}
	}
	if !foundStreamingWarning {
		t.Errorf("warnings = %v, want one flagging ListHellos as a server-streaming stub", res.Warnings)
	}

	resp := mock.Response
	checks := map[string]any{
		"message": "string",
		"ok":      true,
		"count":   "0", // int64 -> JSON string per proto3 JSON mapping
		"score":   0.0,
		"status":  "STATUS_UNSPECIFIED",
	}
	for field, want := range checks {
		if got := resp[field]; got != want {
			t.Errorf("response[%q] = %#v, want %#v", field, got, want)
		}
	}

	if tags, ok := resp["tags"].([]any); !ok || len(tags) != 1 || tags[0] != "string" {
		t.Errorf("response[tags] = %#v, want a single-element string list", resp["tags"])
	}
	if attrs, ok := resp["attrs"].(map[string]any); !ok || attrs["key1"] != 0 {
		t.Errorf("response[attrs] = %#v, want a single string->int32 entry", resp["attrs"])
	}
	if created, ok := resp["created_at"].(string); !ok || created != "2024-01-01T00:00:00Z" {
		t.Errorf("response[created_at] = %#v, want the Timestamp placeholder string", resp["created_at"])
	}
	if addr, ok := resp["address"].(map[string]any); !ok || addr["city"] != "string" {
		t.Errorf("response[address] = %#v, want a nested Address object (from the local common/address.proto import)", resp["address"])
	}

	// Only one member of the "contact" oneof should appear.
	_, hasEmail := resp["email"]
	_, hasPhone := resp["phone"]
	if hasEmail == hasPhone {
		t.Errorf("exactly one of email/phone should be present in a oneof, got email=%v phone=%v", hasEmail, hasPhone)
	}

	// proto3 "optional" fields use a *synthetic* oneof and must still appear.
	if _, ok := resp["nickname"]; !ok {
		t.Error("response[nickname] missing; proto3 optional fields should not be treated as a real oneof")
	}
}

func TestGenerate_DuplicateMethodNameIsSkippedWithWarning(t *testing.T) {
	res, err := protoidl.Generate("testdata/duplicate-method.proto")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Mocks) != 1 {
		t.Fatalf("Mocks = %d, want 1 (second Get shadowed by the first)", len(res.Mocks))
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "ServiceB") && strings.Contains(w, "already used") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings = %v, want one flagging ServiceB.Get as an unreachable duplicate", res.Warnings)
	}
}

func TestGenerate_RejectsExternalImports(t *testing.T) {
	_, err := protoidl.Generate("testdata/external-import.proto")
	if err == nil {
		t.Fatal("expected an error for an absolute-path import, got nil")
	}
}

// TestGenerate_WellKnownTypes exercises every google.protobuf well-known
// type the generator special-cases (previously 0% unit-covered beyond the
// single Timestamp field in greeter.proto): the JSON-mapping placeholders
// for Duration/Empty/wrapper types/FieldMask, and the "approximated, fill
// in manually" warnings for Struct/Value/ListValue/Any. It also confirms
// appendWarningOnce actually dedupes: the message below has two separate
// google.protobuf.Struct fields, so the Struct warning must appear exactly
// once, not twice.
func TestGenerate_WellKnownTypes(t *testing.T) {
	res, err := protoidl.Generate("testdata/wellknown.proto")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Mocks) != 1 {
		t.Fatalf("Mocks = %d, want 1", len(res.Mocks))
	}
	resp := res.Mocks[0].Response

	checks := map[string]any{
		"ttl":           "0s",
		"marker":        map[string]any{},
		"name":          "string",
		"flag":          true,
		"count32":       0,
		"ucount32":      0,
		"count64":       "0",
		"ucount64":      "0",
		"ratio":         0.0,
		"precise_ratio": 0.0,
		"mask":          "",
		"extra":         map[string]any{},
		"more_extra":    map[string]any{},
		"anything":      nil,
		"items":         []any{},
		"payload":       map[string]any{},
	}
	for field, want := range checks {
		got, ok := resp[field]
		if field == "anything" {
			// google.protobuf.Value maps to JSON null; distinguish "present
			// and nil" from "absent" explicitly.
			if _, present := resp[field]; !present {
				t.Errorf("response[%q] missing", field)
			}
			continue
		}
		if !ok {
			t.Errorf("response[%q] missing", field)
			continue
		}
		switch want := want.(type) {
		case map[string]any:
			gm, ok := got.(map[string]any)
			if !ok || len(gm) != len(want) {
				t.Errorf("response[%q] = %#v, want an empty object", field, got)
			}
		case []any:
			ga, ok := got.([]any)
			if !ok || len(ga) != len(want) {
				t.Errorf("response[%q] = %#v, want an empty array", field, got)
			}
		default:
			if got != want {
				t.Errorf("response[%q] = %#v, want %#v", field, got, want)
			}
		}
	}

	wantWarningSubstrings := []string{
		"protobuf.Struct field(s) approximated",
		"protobuf.Value field(s) approximated",
		"protobuf.ListValue field(s) approximated",
		"protobuf.Any field(s) approximated",
	}
	for _, sub := range wantWarningSubstrings {
		count := 0
		for _, w := range res.Warnings {
			if strings.Contains(w, sub) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("warnings containing %q = %d, want exactly 1 (two Struct fields should still dedupe to one warning); warnings = %v", sub, count, res.Warnings)
		}
	}
}

func TestGenerate_Proto2Syntax(t *testing.T) {
	res, err := protoidl.Generate("testdata/legacy-proto2.proto")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Mocks) != 1 {
		t.Fatalf("Mocks = %d, want 1", len(res.Mocks))
	}
	if res.Mocks[0].Response["id"] != "string" {
		t.Errorf("response[id] = %#v, want \"string\"", res.Mocks[0].Response["id"])
	}
}
