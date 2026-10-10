package kafkaserver

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/scenarios"
)

// buildKafkaRequest encodes a minimal Kafka request header (apiKey,
// apiVersion, correlationID, clientID) followed by body, matching the wire
// format handleRequest expects.
func buildKafkaRequest(apiKey, apiVersion int16, correlationID int32, clientID string, body []byte) []byte {
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, apiKey)
	_ = binary.Write(out, binary.BigEndian, apiVersion)
	_ = binary.Write(out, binary.BigEndian, correlationID)
	writeKafkaString(out, clientID)
	out.Write(body)
	return out.Bytes()
}

// readKafkaResponseHeader parses the common 4-byte size prefix + 4-byte
// correlationID response envelope that handleRequest always writes, and
// returns the remaining body via a reader positioned right after it.
func readKafkaResponseHeader(t *testing.T, resp []byte) (correlationID int32, body *bytes.Reader) {
	t.Helper()
	r := bytes.NewReader(resp)
	var size int32
	if err := binary.Read(r, binary.BigEndian, &size); err != nil {
		t.Fatalf("read response size: %v", err)
	}
	if int(size) != len(resp)-4 {
		t.Fatalf("response size = %d, want %d (len(resp)-4)", size, len(resp)-4)
	}
	if err := binary.Read(r, binary.BigEndian, &correlationID); err != nil {
		t.Fatalf("read correlationID: %v", err)
	}
	return correlationID, r
}

func TestMatchKafkaTopic(t *testing.T) {
	if !matchKafkaTopic("orders-*", "orders-created") {
		t.Fatal("expected wildcard match")
	}
	if !matchKafkaTopic(`re:^orders-[a-z]+$`, "orders-created") {
		t.Fatal("expected regex match")
	}
}

// TestHandleRequest_ApiVersions proves handleRequest answers Kafka's
// ApiVersions request (apiKey 18) — the first request real client libraries
// (librdkafka, sarama, kafka-go, etc.) send during connection bootstrap,
// before Produce/Fetch. This response's error code and advertised API list
// decide whether such clients even consider the connection usable.
func TestHandleRequest_ApiVersions(t *testing.T) {
	srv := New(&config.KafkaConfig{Enabled: true, Port: 9092}, nil, scenarios.New(nil), nil)
	req := buildKafkaRequest(18, 0, 42, "test-client", nil)

	resp, err := srv.handleRequest(req)
	if err != nil {
		t.Fatalf("handleRequest: %v", err)
	}

	correlationID, body := readKafkaResponseHeader(t, resp)
	if correlationID != 42 {
		t.Fatalf("correlationID = %d, want 42", correlationID)
	}

	var errorCode int16
	if err := binary.Read(body, binary.BigEndian, &errorCode); err != nil {
		t.Fatalf("read error code: %v", err)
	}
	if errorCode != 0 {
		t.Fatalf("ApiVersions error code = %d, want 0", errorCode)
	}

	var numAPIs int32
	if err := binary.Read(body, binary.BigEndian, &numAPIs); err != nil {
		t.Fatalf("read API count: %v", err)
	}
	wantKeys := map[int16]bool{0: true, 1: true, 3: true, 18: true}
	gotKeys := map[int16]bool{}
	for i := int32(0); i < numAPIs; i++ {
		var key, minVer, maxVer int16
		_ = binary.Read(body, binary.BigEndian, &key)
		_ = binary.Read(body, binary.BigEndian, &minVer)
		_ = binary.Read(body, binary.BigEndian, &maxVer)
		gotKeys[key] = true
	}
	if int(numAPIs) != len(wantKeys) {
		t.Fatalf("ApiVersions advertised %d APIs, want %d", numAPIs, len(wantKeys))
	}
	for key := range wantKeys {
		if !gotKeys[key] {
			t.Fatalf("ApiVersions response missing apiKey %d (supported keys: Produce=0, Fetch=1, Metadata=3, ApiVersions=18)", key)
		}
	}
}

// TestHandleRequest_Metadata proves handleRequest answers Kafka's Metadata
// request (apiKey 3) with the broker's own host/port and one partition per
// requested topic — the second bootstrap call most real client libraries
// make (after ApiVersions) to discover which broker/partition to route
// Produce/Fetch requests to.
func TestHandleRequest_Metadata(t *testing.T) {
	srv := New(&config.KafkaConfig{Enabled: true, Port: 9092}, nil, scenarios.New(nil), nil)

	reqBody := new(bytes.Buffer)
	_ = binary.Write(reqBody, binary.BigEndian, int32(2))
	writeKafkaString(reqBody, "orders")
	writeKafkaString(reqBody, "payments")
	req := buildKafkaRequest(3, 0, 7, "test-client", reqBody.Bytes())

	resp, err := srv.handleRequest(req)
	if err != nil {
		t.Fatalf("handleRequest: %v", err)
	}

	correlationID, body := readKafkaResponseHeader(t, resp)
	if correlationID != 7 {
		t.Fatalf("correlationID = %d, want 7", correlationID)
	}

	var numBrokers int32
	_ = binary.Read(body, binary.BigEndian, &numBrokers)
	if numBrokers != 1 {
		t.Fatalf("broker count = %d, want 1", numBrokers)
	}
	var nodeID int32
	_ = binary.Read(body, binary.BigEndian, &nodeID)
	host := readKafkaString(body)
	var port int32
	_ = binary.Read(body, binary.BigEndian, &port)
	if host != "localhost" {
		t.Fatalf("broker host = %q, want %q", host, "localhost")
	}
	if port != 9092 {
		t.Fatalf("broker port = %d, want 9092 (cfg.Port)", port)
	}

	var numTopics int32
	_ = binary.Read(body, binary.BigEndian, &numTopics)
	if numTopics != 2 {
		t.Fatalf("topic count = %d, want 2", numTopics)
	}
	for i := int32(0); i < numTopics; i++ {
		var topicErr int16
		_ = binary.Read(body, binary.BigEndian, &topicErr)
		topic := readKafkaString(body)
		if topicErr != 0 {
			t.Fatalf("topic %q error code = %d, want 0", topic, topicErr)
		}
		if topic != "orders" && topic != "payments" {
			t.Fatalf("unexpected topic %q in Metadata response", topic)
		}
		isInternal := make([]byte, 1)
		_, _ = body.Read(isInternal)
		var numPartitions int32
		_ = binary.Read(body, binary.BigEndian, &numPartitions)
		if numPartitions != 1 {
			t.Fatalf("topic %q partition count = %d, want 1", topic, numPartitions)
		}
		// Each partition record: error code, partition id, leader,
		// replica count + 1 replica, isr count + 1 isr (all int32 except
		// the leading int16 error code) — must be consumed in full before
		// the next topic's fields, or parsing desyncs.
		var partitionErr int16
		_ = binary.Read(body, binary.BigEndian, &partitionErr)
		if partitionErr != 0 {
			t.Fatalf("topic %q partition error code = %d, want 0", topic, partitionErr)
		}
		var partitionID, leader, replicaCount, replica, isrCount, isr int32
		_ = binary.Read(body, binary.BigEndian, &partitionID)
		_ = binary.Read(body, binary.BigEndian, &leader)
		_ = binary.Read(body, binary.BigEndian, &replicaCount)
		_ = binary.Read(body, binary.BigEndian, &replica)
		_ = binary.Read(body, binary.BigEndian, &isrCount)
		_ = binary.Read(body, binary.BigEndian, &isr)
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.KafkaConfig{Enabled: true, Port: 9092, Mocks: []config.KafkaMock{{ID: "1"}}}, nil, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "kafka" || info["port"] != 9092 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
