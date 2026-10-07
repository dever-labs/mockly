package protoidl_test

import (
	"strings"
	"testing"

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

	if len(res.Mocks) != 1 {
		t.Fatalf("Mocks = %d, want 1 (SayHello only; ListHellos streams)", len(res.Mocks))
	}
	mock := res.Mocks[0]
	if mock.Method != "SayHello" {
		t.Errorf("Method = %q, want SayHello", mock.Method)
	}

	foundStreamingWarning := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "ListHellos") && strings.Contains(w, "streaming") {
			foundStreamingWarning = true
		}
	}
	if !foundStreamingWarning {
		t.Errorf("warnings = %v, want one flagging ListHellos as skipped (streaming)", res.Warnings)
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
