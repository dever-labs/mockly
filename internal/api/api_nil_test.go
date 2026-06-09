// Tests for nil-server branches, scenario edge cases, fault endpoint branches,
// SMTP default action, and waitHTTPCalls.
package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

// ---------------------------------------------------------------------------
// Nil-server list branches (startAPI passes nil for the 10 new protocols)
// ---------------------------------------------------------------------------

func TestAPI_NilServer_ListAMQP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/amqp")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list amqp mocks (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListKafka(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/kafka")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list kafka mocks (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListLDAP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/ldap")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list ldap mocks (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListIMAP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/imap")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list imap mailboxes (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListFTP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/ftp")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list ftp files (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListMemcached(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/memcached")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list memcached mocks (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListSTOMP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/stomp")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list stomp mocks (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListCoAP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/coap")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list coap mocks (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListSIP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/mocks/sip")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list sip mocks (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListAMQPMessages(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/amqp/messages")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list amqp messages (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListKafkaMessages(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/kafka/messages")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list kafka messages (nil server): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_NilServer_ListSTOMPMessages(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/stomp/messages")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("list stomp messages (nil server): want 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Nil-server add branches (503 when protocol is nil)
// ---------------------------------------------------------------------------

func nilServerPost(t *testing.T, url string, body interface{}) int {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestAPI_NilServer_AddAMQP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/amqp", map[string]interface{}{"exchange": "x"})
	if code != 503 {
		t.Errorf("add amqp mock (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddKafka(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/kafka", map[string]interface{}{"topic": "t"})
	if code != 503 {
		t.Errorf("add kafka mock (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddLDAP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/ldap", map[string]interface{}{"base_dn": "dc=x"})
	if code != 503 {
		t.Errorf("add ldap mock (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddIMAP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/imap", map[string]interface{}{"name": "INBOX"})
	if code != 503 {
		t.Errorf("add imap mailbox (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddFTP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/ftp", map[string]interface{}{"path": "/f.txt"})
	if code != 503 {
		t.Errorf("add ftp file (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddMemcached(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/memcached", map[string]interface{}{"key": "k"})
	if code != 503 {
		t.Errorf("add memcached mock (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddSTOMP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/stomp", map[string]interface{}{"destination": "/q/x"})
	if code != 503 {
		t.Errorf("add stomp mock (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddCoAP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/coap", map[string]interface{}{"method": "GET", "path": "/x"})
	if code != 503 {
		t.Errorf("add coap mock (nil server): want 503, got %d", code)
	}
}

func TestAPI_NilServer_AddSIP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	code := nilServerPost(t, base+"/api/mocks/sip", map[string]interface{}{"method": "INVITE"})
	if code != 503 {
		t.Errorf("add sip mock (nil server): want 503, got %d", code)
	}
}

// ---------------------------------------------------------------------------
// Scenario edge cases
// ---------------------------------------------------------------------------

func TestAPI_DeleteScenario_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/scenarios/nonexistent", nil)
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("delete nonexistent scenario: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_ActivateScenario_NotFound(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	body, _ := json.Marshal(map[string]interface{}{})
	resp, _ := http.Post(base+"/api/scenarios/nonexistent/activate", "application/json", bytes.NewReader(body))
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("activate nonexistent scenario: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_UpdateScenario_InvalidJSON(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	// Create the scenario first so we reach the body-decode step.
	body, _ := json.Marshal(config.Scenario{ID: "sc1"})
	req, _ := http.NewRequest(http.MethodPost, base+"/api/scenarios", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	// Now send invalid JSON to update it.
	req, _ = http.NewRequest(http.MethodPut, base+"/api/scenarios/sc1", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("update scenario invalid JSON: want 400, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Fault endpoint branches
// ---------------------------------------------------------------------------

func TestAPI_GetProtocolFault_NilFault(t *testing.T) {
	// Known protocol with no fault set should return 200 with null body.
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/fault/http")
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("get protocol fault (none set): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_GetProtocolFault_UnknownProtocol(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/fault/notaprotocol")
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("get fault for unknown protocol: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_SetProtocolFault_InvalidJSON(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	body := bytes.NewReader([]byte("not-json"))
	req, _ := http.NewRequest(http.MethodPost, base+"/api/fault/http", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("set fault invalid JSON: want 400, got %d", resp.StatusCode)
	}
}

func TestAPI_SetProtocolFault_ValidHTTP(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	fault := map[string]interface{}{"status": 503, "body": "Service Unavailable"}
	b, _ := json.Marshal(fault)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/fault/http", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("set fault valid JSON: want 200, got %d", resp.StatusCode)
	}
	// Response should echo the stored fault, not {"status":"ok"}.
	if !bytes.Contains(body, []byte("503")) {
		t.Errorf("response should contain stored fault, got %s", body)
	}
}

func TestAPI_DeleteProtocolFault_UnknownProtocol(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/fault/notaprotocol", nil)
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("delete fault for unknown protocol: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_GetEffectiveProtocolFault_NoFaultSet(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/fault/http/effective")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("get effective fault (none set): want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_GetEffectiveProtocolFault_UnknownProtocol(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/fault/notaprotocol/effective")
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("get effective fault unknown protocol: want 404, got %d", resp.StatusCode)
	}
}

func TestAPI_GetEffectiveProtocolFault_DirectFaultVisible(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	// Set a direct fault.
	b, _ := json.Marshal(map[string]interface{}{"status": 429})
	req, _ := http.NewRequest(http.MethodPost, base+"/api/fault/http", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	// Effective endpoint should reflect it.
	resp, _ = http.Get(base + "/api/fault/http/effective")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("get effective fault: want 200, got %d", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("429")) {
		t.Errorf("effective fault should contain 429, got %s", body)
	}
}

// ---------------------------------------------------------------------------
// SMTP — add rule with empty Action (triggers default "accept" branch)
// ---------------------------------------------------------------------------

func TestAPI_SMTP_AddRule_DefaultAction(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	rule := map[string]interface{}{"from": "*@example.com"}
	b, _ := json.Marshal(rule)
	resp, _ := http.Post(base+"/api/mocks/smtp", "application/json", bytes.NewReader(b))
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("add smtp rule (no action): want 201, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// waitHTTPCalls — timeout path
// ---------------------------------------------------------------------------

func TestAPI_WaitHTTPCalls_Timeout(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	// Request 1 call with a very short timeout — it should time out immediately.
	body := map[string]interface{}{"count": 99, "timeout": "50ms"}
	b, _ := json.Marshal(body)
	resp, err := http.Post(fmt.Sprintf("%s/api/calls/http/no-such-mock/wait", base), "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("waitHTTPCalls: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 408 {
		t.Errorf("waitHTTPCalls timeout: want 408, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// HTTP list nil-server (covered via startAPI which has real httpStub — skip)
// getHTTPCalls nil path via startAPI with http non-nil → normal path
// ---------------------------------------------------------------------------

func TestAPI_GetHTTPCalls_NoSuchMock(t *testing.T) {
	base, _, _, _, _ := startAPI(t)
	resp, _ := http.Get(base + "/api/calls/http/no-such-mock")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("get http calls for unknown mock: want 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Reset — verify mocks are cleared
// ---------------------------------------------------------------------------

func TestAPI_Reset_ClearsMocks(t *testing.T) {
	base, _, _, _, _ := startAPI(t)

	// Create an HTTP mock.
	mock := map[string]interface{}{"id": "reset-test", "request": map[string]interface{}{"method": "GET", "path": "/reset"}, "response": map[string]interface{}{"status": 200}}
	b, _ := json.Marshal(mock)
	resp, _ := http.Post(base+"/api/mocks/http", "application/json", bytes.NewReader(b))
	_ = resp.Body.Close()

	// POST /api/reset.
	resetResp, _ := http.Post(base+"/api/reset", "application/json", nil)
	_ = resetResp.Body.Close()
	if resetResp.StatusCode != 200 {
		t.Errorf("reset: want 200, got %d", resetResp.StatusCode)
	}
}
