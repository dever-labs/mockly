// API update/patch endpoint tests and nil-server branch tests.
package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

// ---------------------------------------------------------------------------
// Update (PUT) tests for existing startAPI stubs
// ---------------------------------------------------------------------------

func TestAPI_TCP_Update(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	// Create
	mock := map[string]interface{}{"id": "tcp-upd", "match": "PING", "response": "PONG"}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/tcp", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	// Update
	updated := map[string]interface{}{"match": "PING", "response": "PONG2"}
	upBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/tcp/tcp-upd", bytes.NewReader(upBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("update tcp mock: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_TCP_Update_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]interface{}{"match": "X", "response": "Y"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/tcp/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("update nonexistent tcp mock: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_Redis_Update(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{"id": "redis-upd", "command": "GET", "key": "k1", "response": map[string]interface{}{"type": "string", "value": "v1"}}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/redis", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	updated := map[string]interface{}{"command": "GET", "key": "k1", "response": map[string]interface{}{"type": "string", "value": "v2"}}
	upBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/redis/redis-upd", bytes.NewReader(upBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("update redis mock: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_Redis_Update_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]interface{}{"command": "GET", "key": "k"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/redis/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("update nonexistent redis mock: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_SMTP_Update(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	rule := map[string]interface{}{"id": "smtp-upd", "from": "spam@*", "action": "reject"}
	body, _ := json.Marshal(rule)
	resp, _ := http.Post(base+"/api/mocks/smtp", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	updated := map[string]interface{}{"from": "spam@*", "action": "accept"}
	upBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/smtp/smtp-upd", bytes.NewReader(upBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("update smtp rule: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_SMTP_Update_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]interface{}{"from": "*", "action": "accept"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/smtp/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("update nonexistent smtp rule: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_MQTT_Update(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{"id": "mqtt-upd", "topic": "sensors/#", "response": map[string]interface{}{"topic": "sensors/resp", "payload": "ok"}}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/mqtt", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	updated := map[string]interface{}{"topic": "sensors/#", "response": map[string]interface{}{"topic": "sensors/resp", "payload": "updated"}}
	upBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/mqtt/mqtt-upd", bytes.NewReader(upBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("update mqtt mock: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_MQTT_Update_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]interface{}{"topic": "t"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/mqtt/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("update nonexistent mqtt mock: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_WebSocket_Update(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{"id": "ws-upd", "match": "hello", "response": map[string]interface{}{"message": "hi"}}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/websocket", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	updated := map[string]interface{}{"match": "hello", "response": map[string]interface{}{"message": "hey"}}
	upBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/websocket/ws-upd", bytes.NewReader(upBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("update ws mock: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_WebSocket_Update_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]interface{}{"match": "x"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/websocket/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("update nonexistent ws mock: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_GRPC_Update(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{"id": "grpc-upd", "service": "Svc", "method": "Method", "response": map[string]interface{}{"id": "1"}}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/grpc", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	updated := map[string]interface{}{"service": "Svc", "method": "Method", "response": map[string]interface{}{"id": "2"}}
	upBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/grpc/grpc-upd", bytes.NewReader(upBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("update grpc mock: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_GRPC_Update_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]interface{}{"service": "S", "method": "M"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/grpc/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("update nonexistent grpc mock: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_HTTP_Patch(t *testing.T) {
	base, httpStub, _, _, _ := startAPI(t)

	mock := map[string]interface{}{"id": "http-patch", "request": map[string]interface{}{"method": "GET", "path": "/patch"}, "response": map[string]interface{}{"status": 200}}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/http", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()
	if len(httpStub.GetMocks()) == 0 {
		t.Fatal("mock not created")
	}

	patch := map[string]interface{}{"response": map[string]interface{}{"status": 201}}
	pBody, _ := json.Marshal(patch)
	req, _ := http.NewRequest(http.MethodPatch, base+"/api/mocks/http/http-patch", bytes.NewReader(pBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("patch http mock: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_HTTP_Patch_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	body, _ := json.Marshal(map[string]interface{}{"response": map[string]interface{}{"status": 200}})
	req, _ := http.NewRequest(http.MethodPatch, base+"/api/mocks/http/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("patch nonexistent http mock: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_WebSocket_Patch(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{"id": "ws-patch", "match": "hello"}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/websocket", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	patch := map[string]interface{}{"match": "world"}
	pBody, _ := json.Marshal(patch)
	req, _ := http.NewRequest(http.MethodPatch, base+"/api/mocks/websocket/ws-patch", bytes.NewReader(pBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("patch ws mock: want 200, got %d", resp2.StatusCode)
	}
}

func TestAPI_GRPC_Patch(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	mock := map[string]interface{}{"id": "grpc-patch", "service": "Svc", "method": "M"}
	body, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/grpc", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()

	patch := map[string]interface{}{"response": map[string]interface{}{"id": "patched"}}
	pBody, _ := json.Marshal(patch)
	req, _ := http.NewRequest(http.MethodPatch, base+"/api/mocks/grpc/grpc-patch", bytes.NewReader(pBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("patch grpc mock: want 200, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Nil-server branches: POST/PUT to protocols not enabled in startAPI
// ---------------------------------------------------------------------------

func TestAPI_NilServer_DNS_Add(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	body, _ := json.Marshal(map[string]interface{}{"name": "example.com", "type": "A"})
	resp, _ := http.Post(base+"/api/mocks/dns", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("POST /api/mocks/dns with nil server: want 503, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_DNS_List(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/dns")
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/mocks/dns with nil server: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_AMQP_Add(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	body, _ := json.Marshal(map[string]interface{}{"exchange": "events"})
	resp, _ := http.Post(base+"/api/mocks/amqp", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("POST /api/mocks/amqp with nil server: want 503, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_Kafka_Add(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	body, _ := json.Marshal(map[string]interface{}{"topic": "orders"})
	resp, _ := http.Post(base+"/api/mocks/kafka", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("POST /api/mocks/kafka with nil server: want 503, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Update endpoints for new protocols (using startAPIFull)
// ---------------------------------------------------------------------------

func putJSON(t *testing.T, url string, payload interface{}) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	return resp
}

func TestAPI_DNS_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.dns.SetMocks(append(stubs.dns.GetMocks(), config.DNSMock{ID: "dns-upd", Name: "example.com.", Type: "A"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/dns/dns-upd", base),
		map[string]interface{}{"name": "updated.com.", "type": "A"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update dns mock: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_DNS_Update_NotFound(t *testing.T) {
	base, _ := startAPIFull(t)
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/dns/nonexistent", base),
		map[string]interface{}{"name": "x.com."})
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("update nonexistent dns mock: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_AMQP_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.amqp.SetMocks(append(stubs.amqp.GetMocks(), config.AMQPMock{ID: "amqp-upd", Exchange: "events"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/amqp/amqp-upd", base),
		map[string]interface{}{"exchange": "events2", "routing_key": "updated"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update amqp mock: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Kafka_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.kafka.SetMocks(append(stubs.kafka.GetMocks(), config.KafkaMock{ID: "kafka-upd", Topic: "orders"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/kafka/kafka-upd", base),
		map[string]interface{}{"topic": "new-topic"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update kafka mock: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_LDAP_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.ldap.SetMocks(append(stubs.ldap.GetMocks(), config.LDAPMock{ID: "ldap-upd", BaseDN: "dc=example,dc=com"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/ldap/ldap-upd", base),
		map[string]interface{}{"base_dn": "dc=new,dc=com"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update ldap mock: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_IMAP_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.imap.SetMailboxes(append(stubs.imap.GetMailboxes(), config.IMAPMailbox{ID: "imap-upd", Name: "INBOX"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/imap/imap-upd", base),
		map[string]interface{}{"name": "UpdatedBox"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update imap mailbox: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_FTP_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.ftp.SetFiles(append(stubs.ftp.GetFiles(), config.FTPFile{ID: "ftp-upd", Path: "/file.txt", Content: "old"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/ftp/ftp-upd", base),
		map[string]interface{}{"path": "/updated.txt", "content": "new"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update ftp file: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Memcached_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.memcached.SetMocks(append(stubs.memcached.GetMocks(), config.MemcachedMock{ID: "mem-upd", Key: "k1", Command: "get"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/memcached/mem-upd", base),
		map[string]interface{}{"key": "newkey", "command": "get"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update memcached mock: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_STOMP_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.stomp.SetMocks(append(stubs.stomp.GetMocks(), config.STOMPMock{ID: "stomp-upd", Destination: "/queue/orders"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/stomp/stomp-upd", base),
		map[string]interface{}{"destination": "/queue/new"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update stomp mock: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_CoAP_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.coap.SetMocks(append(stubs.coap.GetMocks(), config.CoAPMock{ID: "coap-upd", Method: "GET", Path: "/sensor"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/coap/coap-upd", base),
		map[string]interface{}{"method": "POST", "path": "/new"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update coap mock: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_SIP_Update(t *testing.T) {
	base, stubs := startAPIFull(t)

	stubs.sip.SetMocks(append(stubs.sip.GetMocks(), config.SIPMock{ID: "sip-upd", Method: "INVITE"}))
	resp := putJSON(t, fmt.Sprintf("%s/api/mocks/sip/sip-upd", base),
		map[string]interface{}{"method": "BYE", "uri": "sip:new@example.com"})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("update sip mock: want 200, got %d", resp.StatusCode)
	}
}
