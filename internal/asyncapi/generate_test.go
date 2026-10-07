package asyncapi_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dever-labs/mockly/internal/asyncapi"
)

func TestGenerate_KafkaV2(t *testing.T) {
	res, err := asyncapi.Generate("testdata/kafka-v2.yaml")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(res.Kafka) != 1 {
		t.Fatalf("Kafka mocks = %d, want 1 (got %+v)", len(res.Kafka), res.Kafka)
	}
	mock := res.Kafka[0]
	if mock.Topic != "user/signedup" {
		t.Errorf("topic = %q, want %q", mock.Topic, "user/signedup")
	}
	if len(mock.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(mock.Records))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(mock.Records[0].Value), &payload); err != nil {
		t.Fatalf("record value not valid JSON: %v (value=%s)", err, mock.Records[0].Value)
	}
	if payload["email"] != "user@example.com" {
		t.Errorf("payload.email = %v, want format-aware placeholder", payload["email"])
	}

	// The "user/deleted" subscribe (app-receives) operation on a Kafka
	// server has no Mockly equivalent and must be skipped with a warning,
	// not silently dropped or misrepresented as a pre-seeded topic.
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", res.Warnings)
	}
}

func TestGenerate_MultiProtocolV3(t *testing.T) {
	res, err := asyncapi.Generate("testdata/multi-v3.yaml")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}

	if len(res.MQTT) != 1 {
		t.Fatalf("MQTT mocks = %d, want 1 (got %+v)", len(res.MQTT), res.MQTT)
	}
	mqtt := res.MQTT[0]
	if mqtt.Topic != "devices/{deviceId}/commands" {
		t.Errorf("mqtt topic = %q", mqtt.Topic)
	}
	if mqtt.Response == nil {
		t.Fatal("mqtt mock has no response, want one derived from the operation's reply")
	}
	var ack map[string]any
	if err := json.Unmarshal([]byte(mqtt.Response.Payload), &ack); err != nil {
		t.Fatalf("reply payload not valid JSON: %v (payload=%s)", err, mqtt.Response.Payload)
	}
	if ack["status"] != "ok" {
		t.Errorf("reply status = %v, want %q (from the commandAcks channel's example)", ack["status"], "ok")
	}

	// sendNotification (send) and receivePing (receive) target the same
	// WebSocket path and must be merged into a single mock (the ws server
	// registers one handler per literal path).
	if len(res.WebSocket) != 1 {
		t.Fatalf("WebSocket mocks = %d, want 1 (got %+v)", len(res.WebSocket), res.WebSocket)
	}
	ws := res.WebSocket[0]
	if ws.Path != "/ws/notifications" {
		t.Errorf("ws path = %q", ws.Path)
	}
	if ws.OnConnect == nil || ws.OnConnect.Send == "" {
		t.Error("ws mock missing on_connect push from the 'send' operation")
	}
	if len(ws.OnMessage) != 1 || ws.OnMessage[0].Match != "*" {
		t.Errorf("ws mock missing on_message rule from the 'receive' operation: %+v", ws.OnMessage)
	}
}

func TestGenerate_RejectsExternalRefs(t *testing.T) {
	res, err := asyncapi.Generate("testdata/external-ref.yaml")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Generation still succeeds (one NATS mock, with an empty/default
	// payload in place of the unresolved ref) but must surface a warning
	// rather than silently following the external ref.
	if len(res.NATS) != 1 {
		t.Fatalf("NATS mocks = %d, want 1", len(res.NATS))
	}
	foundWarning := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "external $ref") && strings.Contains(w, "not allowed") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("warnings = %v, want one flagging the external $ref", res.Warnings)
	}
}

func TestGenerate_AmbiguousMultiServerIsSkipped(t *testing.T) {
	res, err := asyncapi.Generate("testdata/ambiguous-v2.yaml")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.Empty() {
		t.Errorf("expected no mocks for an unresolvable multi-server channel, got %+v", res)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", res.Warnings)
	}
}

func TestGenerate_UnsupportedVersion(t *testing.T) {
	_, err := asyncapi.Generate("testdata/unsupported-version.yaml")
	if err == nil {
		t.Fatal("expected an error for an unsupported asyncapi version")
	}
}

func TestGenerate_NotAnAsyncAPIDocument(t *testing.T) {
	_, err := asyncapi.Generate("testdata/not-asyncapi.yaml")
	if err == nil {
		t.Fatal("expected an error for a document with no top-level 'asyncapi' field")
	}
}

func TestIsAsyncAPI(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"testdata/kafka-v2.yaml", true},
		{"testdata/not-asyncapi.yaml", false},
	}
	for _, c := range cases {
		got, err := asyncapi.IsAsyncAPI(c.path)
		if err != nil {
			t.Fatalf("IsAsyncAPI(%q): %v", c.path, err)
		}
		if got != c.want {
			t.Errorf("IsAsyncAPI(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
