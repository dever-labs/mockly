package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/api"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/amqpserver"
	"github.com/dever-labs/mockly/internal/protocols/kafkaserver"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/protocols/stompserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// Stubs for the protocols not covered by the existing startAPI helper
// ---------------------------------------------------------------------------

type stubDNS struct {
	mocks []config.DNSMock
}

func (s *stubDNS) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "dns"}
}
func (s *stubDNS) GetMocks() []config.DNSMock  { return s.mocks }
func (s *stubDNS) SetMocks(m []config.DNSMock) { s.mocks = m }

type stubAMQP struct {
	mocks []config.AMQPMock
	ms    *amqpserver.MessageStore
}

func (s *stubAMQP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "amqp"}
}
func (s *stubAMQP) GetMocks() []config.AMQPMock        { return s.mocks }
func (s *stubAMQP) SetMocks(m []config.AMQPMock)       { s.mocks = m }
func (s *stubAMQP) GetMessageStore() *amqpserver.MessageStore { return s.ms }

type stubKafka struct {
	mocks []config.KafkaMock
	ms    *kafkaserver.MessageStore
}

func (s *stubKafka) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "kafka"}
}
func (s *stubKafka) GetMocks() []config.KafkaMock          { return s.mocks }
func (s *stubKafka) SetMocks(m []config.KafkaMock)         { s.mocks = m }
func (s *stubKafka) GetMessageStore() *kafkaserver.MessageStore { return s.ms }

type stubLDAP struct {
	mocks []config.LDAPMock
}

func (s *stubLDAP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "ldap"}
}
func (s *stubLDAP) GetMocks() []config.LDAPMock  { return s.mocks }
func (s *stubLDAP) SetMocks(m []config.LDAPMock) { s.mocks = m }

type stubIMAP struct {
	mailboxes []config.IMAPMailbox
}

func (s *stubIMAP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "imap"}
}
func (s *stubIMAP) GetMailboxes() []config.IMAPMailbox  { return s.mailboxes }
func (s *stubIMAP) SetMailboxes(m []config.IMAPMailbox) { s.mailboxes = m }

type stubFTP struct {
	files []config.FTPFile
}

func (s *stubFTP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "ftp"}
}
func (s *stubFTP) GetFiles() []config.FTPFile  { return s.files }
func (s *stubFTP) SetFiles(f []config.FTPFile) { s.files = f }

type stubMemcached struct {
	mocks []config.MemcachedMock
}

func (s *stubMemcached) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "memcached"}
}
func (s *stubMemcached) GetMocks() []config.MemcachedMock  { return s.mocks }
func (s *stubMemcached) SetMocks(m []config.MemcachedMock) { s.mocks = m }

type stubSTOMP struct {
	mocks []config.STOMPMock
	ms    *stompserver.MessageStore
}

func (s *stubSTOMP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "stomp"}
}
func (s *stubSTOMP) GetMocks() []config.STOMPMock         { return s.mocks }
func (s *stubSTOMP) SetMocks(m []config.STOMPMock)        { s.mocks = m }
func (s *stubSTOMP) GetMessageStore() *stompserver.MessageStore { return s.ms }

type stubCoAP struct {
	mocks []config.CoAPMock
}

func (s *stubCoAP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "coap"}
}
func (s *stubCoAP) GetMocks() []config.CoAPMock  { return s.mocks }
func (s *stubCoAP) SetMocks(m []config.CoAPMock) { s.mocks = m }

type stubSIP struct {
	mocks []config.SIPMock
}

func (s *stubSIP) StatusInfo() map[string]interface{} {
	return map[string]interface{}{"protocol": "sip"}
}
func (s *stubSIP) GetMocks() []config.SIPMock  { return s.mocks }
func (s *stubSIP) SetMocks(m []config.SIPMock) { s.mocks = m }

// ---------------------------------------------------------------------------
// startAPIFull: API server with all protocol stubs wired in
// ---------------------------------------------------------------------------

type fullAPIStubs struct {
	dns       *stubDNS
	amqp      *stubAMQP
	kafka     *stubKafka
	ldap      *stubLDAP
	imap      *stubIMAP
	ftp       *stubFTP
	memcached *stubMemcached
	stomp     *stubSTOMP
	coap      *stubCoAP
	sip       *stubSIP
}

func startAPIFull(t *testing.T) (string, *fullAPIStubs) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{}
	cfg.Mockly.API.Port = port

	sc := scenarios.New(nil)
	store := state.New()
	log := logger.New(100)

	stubs := &fullAPIStubs{
		dns:       &stubDNS{},
		amqp:      &stubAMQP{ms: amqpserver.NewMessageStore(50)},
		kafka:     &stubKafka{ms: kafkaserver.NewMessageStore(50)},
		ldap:      &stubLDAP{},
		imap:      &stubIMAP{},
		ftp:       &stubFTP{},
		memcached: &stubMemcached{},
		stomp:     &stubSTOMP{ms: stompserver.NewMessageStore(50)},
		coap:      &stubCoAP{},
		sip:       &stubSIP{},
	}

	srv := api.New(
		cfg, store, sc, log,
		&stubHTTP{},
		&stubWS{},
		&stubGRPC{},
		&stubGraphQL{},
		&stubTCP{},
		&stubRedis{},
		&stubSMTP{inbox: smtpserver.NewInbox(50)},
		&stubMQTT{ms: mqttserver.NewMessageStore(50)},
		&stubSNMP{},
		stubs.dns,
		stubs.amqp,
		stubs.kafka,
		stubs.ldap,
		stubs.imap,
		stubs.ftp,
		stubs.memcached,
		stubs.stomp,
		stubs.coap,
		stubs.sip,
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base+"/api/protocols", 2*time.Second)
	return base, stubs
}

// ---------------------------------------------------------------------------
// DNS mock CRUD
// ---------------------------------------------------------------------------

func TestAPI_DNS_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	// Create.
	mock := map[string]interface{}{
		"id": "dns-mock", "name": "example.com", "type": "A",
		"records": []string{"1.2.3.4"},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/dns", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/dns: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create dns mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.dns.GetMocks()) != 1 {
		t.Fatalf("stub not updated after create: %+v", stubs.dns.GetMocks())
	}

	// List.
	resp2, err := http.Get(base + "/api/mocks/dns")
	if err != nil {
		t.Fatalf("GET /api/mocks/dns: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list dns mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	updated := map[string]interface{}{"id": "dns-mock", "name": "example.com", "type": "A", "records": []string{"5.6.7.8"}}
	putBody, _ := json.Marshal(updated)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/dns/dns-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update dns mock: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/dns/dns-mock", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete dns mock: want 200, got %d", resp4.StatusCode)
	}
	if len(stubs.dns.GetMocks()) != 0 {
		t.Errorf("expected 0 dns mocks after delete, got %d", len(stubs.dns.GetMocks()))
	}
}

// ---------------------------------------------------------------------------
// AMQP mock CRUD + messages
// ---------------------------------------------------------------------------

func TestAPI_AMQP_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mock := map[string]interface{}{
		"id":          "amqp-mock",
		"exchange":    "events",
		"routing_key": "order.created",
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/amqp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/amqp: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create amqp mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.amqp.GetMocks()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.amqp.GetMocks())
	}

	resp2, _ := http.Get(base + "/api/mocks/amqp")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list amqp mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "amqp-mock", "exchange": "events-v2"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/amqp/amqp-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update amqp mock: want 200, got %d", resp3.StatusCode)
	}

	// Messages list and clear.
	resp4, _ := http.Get(base + "/api/amqp/messages")
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("list amqp messages: want 200, got %d", resp4.StatusCode)
	}

	req5, _ := http.NewRequest(http.MethodDelete, base+"/api/amqp/messages", nil)
	resp5, _ := http.DefaultClient.Do(req5)
	_ = resp5.Body.Close()
	if resp5.StatusCode != 200 {
		t.Errorf("clear amqp messages: want 200, got %d", resp5.StatusCode)
	}

	// Delete mock.
	req6, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/amqp/amqp-mock", nil)
	resp6, _ := http.DefaultClient.Do(req6)
	_ = resp6.Body.Close()
	if resp6.StatusCode != 200 {
		t.Errorf("delete amqp mock: want 200, got %d", resp6.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Kafka mock CRUD + messages
// ---------------------------------------------------------------------------

func TestAPI_Kafka_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mock := map[string]interface{}{
		"id":    "kafka-mock",
		"topic": "orders",
		"records": []map[string]interface{}{
			{"key": "k1", "value": "v1"},
		},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/kafka", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/kafka: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create kafka mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.kafka.GetMocks()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.kafka.GetMocks())
	}

	resp2, _ := http.Get(base + "/api/mocks/kafka")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list kafka mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "kafka-mock", "topic": "orders-v2"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/kafka/kafka-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update kafka mock: want 200, got %d", resp3.StatusCode)
	}

	// Messages list and clear.
	resp4, _ := http.Get(base + "/api/kafka/messages")
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("list kafka messages: want 200, got %d", resp4.StatusCode)
	}

	req5, _ := http.NewRequest(http.MethodDelete, base+"/api/kafka/messages", nil)
	resp5, _ := http.DefaultClient.Do(req5)
	_ = resp5.Body.Close()
	if resp5.StatusCode != 200 {
		t.Errorf("clear kafka messages: want 200, got %d", resp5.StatusCode)
	}

	// Delete mock.
	req6, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/kafka/kafka-mock", nil)
	resp6, _ := http.DefaultClient.Do(req6)
	_ = resp6.Body.Close()
	if resp6.StatusCode != 200 {
		t.Errorf("delete kafka mock: want 200, got %d", resp6.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// LDAP mock CRUD
// ---------------------------------------------------------------------------

func TestAPI_LDAP_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mock := map[string]interface{}{
		"id":      "ldap-mock",
		"base_dn": "dc=example,dc=com",
		"filter":  "(uid=*)",
		"attributes": map[string]interface{}{
			"cn": []string{"Test User"},
		},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/ldap", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/ldap: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create ldap mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.ldap.GetMocks()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.ldap.GetMocks())
	}

	resp2, _ := http.Get(base + "/api/mocks/ldap")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list ldap mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "ldap-mock", "base_dn": "dc=updated,dc=com"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/ldap/ldap-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update ldap mock: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/ldap/ldap-mock", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete ldap mock: want 200, got %d", resp4.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// IMAP mailbox CRUD
// ---------------------------------------------------------------------------

func TestAPI_IMAP_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mailbox := map[string]interface{}{
		"id":   "imap-mbox",
		"name": "INBOX",
		"messages": []map[string]interface{}{
			{"from": "sender@example.com", "to": "user@example.com", "subject": "Test", "body": "Hello"},
		},
	}
	body, _ := json.Marshal(mailbox)
	resp, err := http.Post(base+"/api/mocks/imap", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/imap: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create imap mailbox: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.imap.GetMailboxes()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.imap.GetMailboxes())
	}

	resp2, _ := http.Get(base + "/api/mocks/imap")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list imap mailboxes: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "imap-mbox", "name": "Sent"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/imap/imap-mbox", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update imap mailbox: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/imap/imap-mbox", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete imap mailbox: want 200, got %d", resp4.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// FTP file CRUD
// ---------------------------------------------------------------------------

func TestAPI_FTP_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	file := map[string]interface{}{
		"id":      "ftp-file",
		"path":    "/data/report.csv",
		"content": "id,name\n1,Alice",
	}
	body, _ := json.Marshal(file)
	resp, err := http.Post(base+"/api/mocks/ftp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/ftp: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create ftp file: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.ftp.GetFiles()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.ftp.GetFiles())
	}

	resp2, _ := http.Get(base + "/api/mocks/ftp")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list ftp files: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "ftp-file", "path": "/data/report.csv", "content": "updated"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/ftp/ftp-file", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update ftp file: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/ftp/ftp-file", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete ftp file: want 200, got %d", resp4.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Memcached mock CRUD
// ---------------------------------------------------------------------------

func TestAPI_Memcached_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mock := map[string]interface{}{
		"id":      "mc-mock",
		"command": "get",
		"key":     "user:*",
		"response": map[string]interface{}{
			"value": "alice",
			"flags": 0,
		},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/memcached", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/memcached: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create memcached mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.memcached.GetMocks()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.memcached.GetMocks())
	}

	resp2, _ := http.Get(base + "/api/mocks/memcached")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list memcached mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "mc-mock", "command": "get", "key": "session:*"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/memcached/mc-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update memcached mock: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/memcached/mc-mock", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete memcached mock: want 200, got %d", resp4.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// STOMP mock CRUD + messages
// ---------------------------------------------------------------------------

func TestAPI_STOMP_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mock := map[string]interface{}{
		"id":          "stomp-mock",
		"destination": "/queue/orders",
		"response": map[string]interface{}{
			"destination": "/queue/responses",
			"body":        "ack",
		},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/stomp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/stomp: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create stomp mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.stomp.GetMocks()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.stomp.GetMocks())
	}

	resp2, _ := http.Get(base + "/api/mocks/stomp")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list stomp mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "stomp-mock", "destination": "/queue/orders-v2"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/stomp/stomp-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update stomp mock: want 200, got %d", resp3.StatusCode)
	}

	// Messages list and clear.
	resp4, _ := http.Get(base + "/api/stomp/messages")
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("list stomp messages: want 200, got %d", resp4.StatusCode)
	}

	req5, _ := http.NewRequest(http.MethodDelete, base+"/api/stomp/messages", nil)
	resp5, _ := http.DefaultClient.Do(req5)
	_ = resp5.Body.Close()
	if resp5.StatusCode != 200 {
		t.Errorf("clear stomp messages: want 200, got %d", resp5.StatusCode)
	}

	// Delete mock.
	req6, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/stomp/stomp-mock", nil)
	resp6, _ := http.DefaultClient.Do(req6)
	_ = resp6.Body.Close()
	if resp6.StatusCode != 200 {
		t.Errorf("delete stomp mock: want 200, got %d", resp6.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// CoAP mock CRUD
// ---------------------------------------------------------------------------

func TestAPI_CoAP_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mock := map[string]interface{}{
		"id":     "coap-mock",
		"method": "GET",
		"path":   "/sensors/temperature",
		"response": map[string]interface{}{
			"code":    "2.05",
			"payload": "22.5",
		},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/coap", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/coap: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create coap mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.coap.GetMocks()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.coap.GetMocks())
	}

	resp2, _ := http.Get(base + "/api/mocks/coap")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list coap mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "coap-mock", "method": "POST", "path": "/sensors/temperature"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/coap/coap-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update coap mock: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/coap/coap-mock", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete coap mock: want 200, got %d", resp4.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// SIP mock CRUD
// ---------------------------------------------------------------------------

func TestAPI_SIP_CRUD(t *testing.T) {
	base, stubs := startAPIFull(t)

	mock := map[string]interface{}{
		"id":     "sip-mock",
		"method": "INVITE",
		"uri":    "sip:bob@example.com",
		"response": map[string]interface{}{
			"status": 200,
			"reason": "OK",
		},
	}
	body, _ := json.Marshal(mock)
	resp, err := http.Post(base+"/api/mocks/sip", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/mocks/sip: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("create sip mock: want 201, got %d", resp.StatusCode)
	}
	if len(stubs.sip.GetMocks()) != 1 {
		t.Fatalf("stub not updated: %+v", stubs.sip.GetMocks())
	}

	resp2, _ := http.Get(base + "/api/mocks/sip")
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("list sip mocks: want 200, got %d", resp2.StatusCode)
	}

	// Update.
	putBody, _ := json.Marshal(map[string]interface{}{"id": "sip-mock", "method": "REGISTER"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/mocks/sip/sip-mock", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req)
	_ = resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("update sip mock: want 200, got %d", resp3.StatusCode)
	}

	// Delete.
	req4, _ := http.NewRequest(http.MethodDelete, base+"/api/mocks/sip/sip-mock", nil)
	resp4, _ := http.DefaultClient.Do(req4)
	_ = resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("delete sip mock: want 200, got %d", resp4.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/fault/{protocol} GET + per-protocol clear
// ---------------------------------------------------------------------------

func TestAPI_Fault_GetProtocol(t *testing.T) {
	base, _, _, sc, _ := startAPI(t)
	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})

	resp, err := http.Get(base + "/api/fault/http")
	if err != nil {
		t.Fatalf("GET /api/fault/http: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/fault/http: want 200, got %d", resp.StatusCode)
	}
}

func TestAPI_Fault_ClearProtocol(t *testing.T) {
	base, _, _, sc, _ := startAPI(t)
	sc.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})

	req, _ := http.NewRequest(http.MethodDelete, base+"/api/fault/http", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/fault/http: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 204 {
		t.Errorf("DELETE /api/fault/http: want 204, got %d", resp.StatusCode)
	}
	if sc.DirectFaults().HTTP != nil {
		t.Error("HTTP fault should be nil after per-protocol clear")
	}
}
