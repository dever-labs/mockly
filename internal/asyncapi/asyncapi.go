// Package asyncapi generates a starter set of protocol mocks from an
// AsyncAPI 2.x or 3.x document, the event-driven sibling of the openapi
// package: point `mockly generate` at a spec and get a runnable config for
// the Kafka/MQTT/AMQP/NATS/WebSocket channels it describes, instead of
// hand-writing every mock.
//
// AsyncAPI operations don't map onto Mockly mocks as directly as OpenAPI's
// request/response pairs do, since the two directions mean different
// things per protocol: for Kafka, an operation where the application emits
// messages becomes a pre-seeded topic (data for a consumer to read); for
// MQTT/AMQP/NATS, an operation where the application receives messages
// becomes a reactive listener (optionally replying, if the operation
// declares a reply message). The opposite direction has no Mockly
// equivalent for those protocols yet and is skipped with a warning rather
// than silently dropped or approximated. See buildMocks for the full
// mapping.
//
// Like the OpenAPI generator, only local ($ref starting with "#/")
// references are ever resolved — a spec is often downloaded from an
// untrusted source, and resolving external refs would let it make Mockly
// read arbitrary local files or issue SSRF-style requests on the user's
// behalf.
package asyncapi

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dever-labs/mockly/internal/config"
)

// Result is the outcome of generating mocks from an AsyncAPI document, one
// slice per protocol the generator could target.
type Result struct {
	Kafka     []config.KafkaMock
	MQTT      []config.MQTTMock
	AMQP      []config.AMQPMock
	NATS      []config.NATSMock
	WebSocket []config.WebSocketMock
	Warnings  []string
}

// Empty reports whether no mocks at all were generated for any protocol.
func (r *Result) Empty() bool {
	return len(r.Kafka) == 0 && len(r.MQTT) == 0 && len(r.AMQP) == 0 &&
		len(r.NATS) == 0 && len(r.WebSocket) == 0
}

// Generate parses the AsyncAPI 2.x or 3.x document at specPath (YAML or
// JSON, local file) and returns the mocks it could derive from its
// channels/operations.
func Generate(specPath string) (*Result, error) {
	doc, err := loadDoc(specPath)
	if err != nil {
		return nil, err
	}

	verRaw, _ := doc["asyncapi"].(string)
	if verRaw == "" {
		return nil, fmt.Errorf("%s: not an AsyncAPI document (missing top-level \"asyncapi\" version field)", specPath)
	}
	major := strings.SplitN(verRaw, ".", 2)[0]

	var ops []operation
	var warnings []string
	switch major {
	case "2":
		ops, warnings = parseV2(doc)
	case "3":
		ops, warnings = parseV3(doc)
	default:
		return nil, fmt.Errorf("%s: unsupported AsyncAPI version %q (only 2.x and 3.x are supported)", specPath, verRaw)
	}

	res := buildMocks(ops)
	res.Warnings = append(warnings, res.Warnings...)
	return res, nil
}

// IsAsyncAPI reports whether the document at specPath looks like an
// AsyncAPI spec (as opposed to, say, an OpenAPI spec), by sniffing for a
// top-level "asyncapi" field. Used by the unified `mockly generate` CLI
// command to pick the right generator automatically.
func IsAsyncAPI(specPath string) (bool, error) {
	doc, err := loadDoc(specPath)
	if err != nil {
		return false, err
	}
	_, ok := doc["asyncapi"]
	return ok, nil
}

func loadDoc(specPath string) (map[string]any, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading AsyncAPI spec %q: %w", specPath, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing AsyncAPI spec %q: %w", specPath, err)
	}
	return doc, nil
}
