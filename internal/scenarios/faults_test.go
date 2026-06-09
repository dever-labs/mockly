package scenarios_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/scenarios"
)

// ---------------------------------------------------------------------------
// Effective*Fault helpers: shared test pattern
// ---------------------------------------------------------------------------

// faultScenario creates a store with one scenario carrying the given faults.
func faultScenario(t *testing.T, faults *config.ProtocolFaults) *scenarios.Store {
	t.Helper()
	return scenarios.New([]config.Scenario{{ID: "s1", Faults: faults}})
}

func TestEffectiveDNSFault_NilWhenNothingSet(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveDNSFault() != nil {
		t.Fatal("expected nil DNS fault when nothing is set")
	}
}

func TestEffectiveDNSFault_DirectFault(t *testing.T) {
	s := scenarios.New(nil)
	s.SetDirectFaults(config.ProtocolFaults{DNS: &config.DNSFault{Rcode: "NXDOMAIN"}})
	f := s.EffectiveDNSFault()
	if f == nil || f.Rcode != "NXDOMAIN" {
		t.Fatalf("unexpected DNS fault: %+v", f)
	}
}

func TestEffectiveDNSFault_ScenarioOverridesDirectFault(t *testing.T) {
	s := faultScenario(t, &config.ProtocolFaults{DNS: &config.DNSFault{Rcode: "REFUSED"}})
	s.SetDirectFaults(config.ProtocolFaults{DNS: &config.DNSFault{Rcode: "NXDOMAIN"}})
	s.Activate("s1")
	f := s.EffectiveDNSFault()
	if f == nil || f.Rcode != "REFUSED" {
		t.Fatalf("scenario should override direct fault: %+v", f)
	}
}

func TestEffectiveGRPCFault_DirectAndScenario(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveGRPCFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{GRPC: &config.GRPCFault{Code: "UNAVAILABLE"}})
	if f := s.EffectiveGRPCFault(); f == nil || f.Code != "UNAVAILABLE" {
		t.Fatalf("unexpected gRPC fault: %+v", f)
	}
}

func TestEffectiveGraphQLFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveGraphQLFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{GraphQL: &config.HTTPFault{Status: 422}})
	if f := s.EffectiveGraphQLFault(); f == nil || f.Status != 422 {
		t.Fatalf("unexpected GraphQL fault: %+v", f)
	}
}

func TestEffectiveWebSocketFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveWebSocketFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{WebSocket: &config.WebSocketFault{CloseCode: 1008}})
	if f := s.EffectiveWebSocketFault(); f == nil || f.CloseCode != 1008 {
		t.Fatalf("unexpected WebSocket fault: %+v", f)
	}
}

func TestEffectiveTCPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveTCPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{TCP: &config.TCPFault{Response: "ERR"}})
	if f := s.EffectiveTCPFault(); f == nil || f.Response != "ERR" {
		t.Fatalf("unexpected TCP fault: %+v", f)
	}
}

func TestEffectiveRedisFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveRedisFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{Redis: &config.RedisFault{Error: "LOADING"}})
	if f := s.EffectiveRedisFault(); f == nil || f.Error != "LOADING" {
		t.Fatalf("unexpected Redis fault: %+v", f)
	}
}

func TestEffectiveMQTTFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveMQTTFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{MQTT: &config.MQTTFault{ErrorRate: 1.0}})
	if f := s.EffectiveMQTTFault(); f == nil || f.ErrorRate != 1.0 {
		t.Fatalf("unexpected MQTT fault: %+v", f)
	}
}

func TestEffectiveSMTPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveSMTPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{SMTP: &config.SMTPFault{Code: 421}})
	if f := s.EffectiveSMTPFault(); f == nil || f.Code != 421 {
		t.Fatalf("unexpected SMTP fault: %+v", f)
	}
}

func TestEffectiveSNMPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveSNMPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{SNMP: &config.SNMPFault{Message: "timeout"}})
	if f := s.EffectiveSNMPFault(); f == nil || f.Message != "timeout" {
		t.Fatalf("unexpected SNMP fault: %+v", f)
	}
}

func TestEffectiveAMQPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveAMQPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{AMQP: &config.AMQPFault{ErrorRate: 0.5}})
	if f := s.EffectiveAMQPFault(); f == nil || f.ErrorRate != 0.5 {
		t.Fatalf("unexpected AMQP fault: %+v", f)
	}
}

func TestEffectiveKafkaFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveKafkaFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{Kafka: &config.KafkaFault{ErrorCode: 3}})
	if f := s.EffectiveKafkaFault(); f == nil || f.ErrorCode != 3 {
		t.Fatalf("unexpected Kafka fault: %+v", f)
	}
}

func TestEffectiveLDAPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveLDAPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{LDAP: &config.LDAPFault{ResultCode: 49}})
	if f := s.EffectiveLDAPFault(); f == nil || f.ResultCode != 49 {
		t.Fatalf("unexpected LDAP fault: %+v", f)
	}
}

func TestEffectiveIMAPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveIMAPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{IMAP: &config.IMAPFault{Response: "BYE"}})
	if f := s.EffectiveIMAPFault(); f == nil || f.Response != "BYE" {
		t.Fatalf("unexpected IMAP fault: %+v", f)
	}
}

func TestEffectiveFTPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveFTPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{FTP: &config.FTPFault{Code: 421, Message: "down"}})
	if f := s.EffectiveFTPFault(); f == nil || f.Code != 421 {
		t.Fatalf("unexpected FTP fault: %+v", f)
	}
}

func TestEffectiveMemcachedFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveMemcachedFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{Memcached: &config.MemcachedFault{ErrorType: "SERVER_ERROR"}})
	if f := s.EffectiveMemcachedFault(); f == nil || f.ErrorType != "SERVER_ERROR" {
		t.Fatalf("unexpected Memcached fault: %+v", f)
	}
}

func TestEffectiveSTOMPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveSTOMPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{STOMP: &config.STOMPFault{Message: "overloaded"}})
	if f := s.EffectiveSTOMPFault(); f == nil || f.Message != "overloaded" {
		t.Fatalf("unexpected STOMP fault: %+v", f)
	}
}

func TestEffectiveCoAPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveCoAPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{CoAP: &config.CoAPFault{Code: "5.00"}})
	if f := s.EffectiveCoAPFault(); f == nil || f.Code != "5.00" {
		t.Fatalf("unexpected CoAP fault: %+v", f)
	}
}

func TestEffectiveSIPFault(t *testing.T) {
	s := scenarios.New(nil)
	if s.EffectiveSIPFault() != nil {
		t.Fatal("expected nil when nothing set")
	}
	s.SetDirectFaults(config.ProtocolFaults{SIP: &config.SIPFault{Status: 503}})
	if f := s.EffectiveSIPFault(); f == nil || f.Status != 503 {
		t.Fatalf("unexpected SIP fault: %+v", f)
	}
}

// ---------------------------------------------------------------------------
// GetDirectProtocolFault
// ---------------------------------------------------------------------------

func TestGetDirectProtocolFault_AllProtocols(t *testing.T) {
	s := scenarios.New(nil)

	// Call each protocol before faults are set — should not panic and be "known".
	protocols := []string{
		"http", "graphql", "websocket", "grpc", "tcp", "redis", "mqtt",
		"smtp", "snmp", "dns", "amqp", "kafka", "ldap", "imap", "ftp",
		"memcached", "stomp", "coap", "sip",
	}
	for _, proto := range protocols {
		_, ok := s.GetDirectProtocolFault(proto)
		if !ok {
			t.Errorf("protocol %q should be known, got ok=false", proto)
		}
	}

	// Set HTTP fault and verify it's returned as non-nil.
	s.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})
	got, ok := s.GetDirectProtocolFault("http")
	if !ok || got == nil {
		t.Fatal("expected HTTP fault to be returned")
	}

	// Unknown protocol should return ok=false.
	_, ok = s.GetDirectProtocolFault("unknown")
	if ok {
		t.Fatal("unknown protocol should return ok=false")
	}
}

func TestGetDirectProtocolFault_CaseSensitive(t *testing.T) {
	s := scenarios.New(nil)
	s.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 500}})
	// GetDirectProtocolFault uses strings.ToLower internally.
	got, ok := s.GetDirectProtocolFault("HTTP")
	if !ok || got == nil {
		t.Fatal("GetDirectProtocolFault should be case-insensitive")
	}
}

// ---------------------------------------------------------------------------
// SetDirectProtocolFaultJSON
// ---------------------------------------------------------------------------

func TestSetDirectProtocolFaultJSON_HTTP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.HTTPFault{Status: 503, Body: "oops"})
	if err := s.SetDirectProtocolFaultJSON("http", data); err != nil {
		t.Fatalf("SetDirectProtocolFaultJSON http: %v", err)
	}
	f := s.EffectiveHTTPFault()
	if f == nil || f.Status != 503 || f.Body != "oops" {
		t.Fatalf("unexpected HTTP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_GraphQL(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.HTTPFault{Status: 422})
	if err := s.SetDirectProtocolFaultJSON("graphql", data); err != nil {
		t.Fatalf("SetDirectProtocolFaultJSON graphql: %v", err)
	}
	if f := s.EffectiveGraphQLFault(); f == nil || f.Status != 422 {
		t.Fatalf("unexpected GraphQL fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_WebSocket(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.WebSocketFault{CloseCode: 1008})
	if err := s.SetDirectProtocolFaultJSON("websocket", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveWebSocketFault(); f == nil || f.CloseCode != 1008 {
		t.Fatalf("unexpected WebSocket fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_GRPC(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.GRPCFault{Code: "UNAVAILABLE"})
	if err := s.SetDirectProtocolFaultJSON("grpc", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveGRPCFault(); f == nil || f.Code != "UNAVAILABLE" {
		t.Fatalf("unexpected gRPC fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_TCP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.TCPFault{Response: "RESET"})
	if err := s.SetDirectProtocolFaultJSON("tcp", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveTCPFault(); f == nil || f.Response != "RESET" {
		t.Fatalf("unexpected TCP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_Redis(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.RedisFault{Error: "LOADING"})
	if err := s.SetDirectProtocolFaultJSON("redis", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveRedisFault(); f == nil || f.Error != "LOADING" {
		t.Fatalf("unexpected Redis fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_MQTT(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.MQTTFault{ErrorRate: 0.5})
	if err := s.SetDirectProtocolFaultJSON("mqtt", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveMQTTFault(); f == nil || f.ErrorRate != 0.5 {
		t.Fatalf("unexpected MQTT fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_SMTP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.SMTPFault{Code: 421})
	if err := s.SetDirectProtocolFaultJSON("smtp", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveSMTPFault(); f == nil || f.Code != 421 {
		t.Fatalf("unexpected SMTP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_SNMP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.SNMPFault{Message: "timeout"})
	if err := s.SetDirectProtocolFaultJSON("snmp", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveSNMPFault(); f == nil || f.Message != "timeout" {
		t.Fatalf("unexpected SNMP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_DNS(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.DNSFault{Rcode: "NXDOMAIN"})
	if err := s.SetDirectProtocolFaultJSON("dns", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveDNSFault(); f == nil || f.Rcode != "NXDOMAIN" {
		t.Fatalf("unexpected DNS fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_AMQP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.AMQPFault{ErrorRate: 0.8})
	if err := s.SetDirectProtocolFaultJSON("amqp", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveAMQPFault(); f == nil || f.ErrorRate != 0.8 {
		t.Fatalf("unexpected AMQP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_Kafka(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.KafkaFault{ErrorCode: 7})
	if err := s.SetDirectProtocolFaultJSON("kafka", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveKafkaFault(); f == nil || f.ErrorCode != 7 {
		t.Fatalf("unexpected Kafka fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_LDAP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.LDAPFault{ResultCode: 52})
	if err := s.SetDirectProtocolFaultJSON("ldap", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveLDAPFault(); f == nil || f.ResultCode != 52 {
		t.Fatalf("unexpected LDAP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_IMAP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.IMAPFault{Response: "NO"})
	if err := s.SetDirectProtocolFaultJSON("imap", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveIMAPFault(); f == nil || f.Response != "NO" {
		t.Fatalf("unexpected IMAP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_FTP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.FTPFault{Code: 530})
	if err := s.SetDirectProtocolFaultJSON("ftp", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveFTPFault(); f == nil || f.Code != 530 {
		t.Fatalf("unexpected FTP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_Memcached(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.MemcachedFault{ErrorType: "CLIENT_ERROR"})
	if err := s.SetDirectProtocolFaultJSON("memcached", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveMemcachedFault(); f == nil || f.ErrorType != "CLIENT_ERROR" {
		t.Fatalf("unexpected Memcached fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_STOMP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.STOMPFault{Message: "fault"})
	if err := s.SetDirectProtocolFaultJSON("stomp", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveSTOMPFault(); f == nil || f.Message != "fault" {
		t.Fatalf("unexpected STOMP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_CoAP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.CoAPFault{Code: "4.04"})
	if err := s.SetDirectProtocolFaultJSON("coap", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveCoAPFault(); f == nil || f.Code != "4.04" {
		t.Fatalf("unexpected CoAP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_SIP(t *testing.T) {
	s := scenarios.New(nil)
	data, _ := json.Marshal(config.SIPFault{Status: 486})
	if err := s.SetDirectProtocolFaultJSON("sip", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := s.EffectiveSIPFault(); f == nil || f.Status != 486 {
		t.Fatalf("unexpected SIP fault: %+v", f)
	}
}

func TestSetDirectProtocolFaultJSON_UnknownProtocol(t *testing.T) {
	s := scenarios.New(nil)
	if err := s.SetDirectProtocolFaultJSON("unknown", []byte(`{}`)); err == nil {
		t.Fatal("expected error for unknown protocol")
	}
}

func TestSetDirectProtocolFaultJSON_InvalidJSON(t *testing.T) {
	s := scenarios.New(nil)
	if err := s.SetDirectProtocolFaultJSON("http", []byte(`{bad json`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSetDirectProtocolFaultJSON_InvalidJSON_AllProtocols(t *testing.T) {
	protocols := []string{
		"graphql", "websocket", "grpc", "tcp", "redis", "mqtt", "smtp",
		"snmp", "dns", "amqp", "kafka", "ldap", "imap", "ftp",
		"memcached", "stomp", "coap", "sip",
	}
	for _, proto := range protocols {
		s := scenarios.New(nil)
		if err := s.SetDirectProtocolFaultJSON(proto, []byte(`{bad json`)); err == nil {
			t.Errorf("protocol %q: expected error for invalid JSON, got nil", proto)
		}
	}
}

// ---------------------------------------------------------------------------
// ClearDirectProtocolFault
// ---------------------------------------------------------------------------

// isFaultNil checks whether an interface{} returned by GetDirectProtocolFault
// is nil or wraps a nil pointer (the typed-nil interface case in Go).
func isFaultNil(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}

func TestClearDirectProtocolFault_HTTP(t *testing.T) {
	s := scenarios.New(nil)
	s.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})
	if ok := s.ClearDirectProtocolFault("http"); !ok {
		t.Fatal("ClearDirectProtocolFault should return true for known protocol")
	}
	got, _ := s.GetDirectProtocolFault("http")
	if !isFaultNil(got) {
		t.Fatal("expected nil HTTP fault after clear")
	}
}

func TestClearDirectProtocolFault_AllProtocols(t *testing.T) {
	s := scenarios.New(nil)
	s.SetDirectFaults(config.ProtocolFaults{
		HTTP:      &config.HTTPFault{Status: 503},
		GraphQL:   &config.HTTPFault{Status: 422},
		WebSocket: &config.WebSocketFault{CloseCode: 1008},
		GRPC:      &config.GRPCFault{Code: "UNAVAILABLE"},
		TCP:       &config.TCPFault{Response: "ERR"},
		Redis:     &config.RedisFault{Error: "LOADING"},
		MQTT:      &config.MQTTFault{ErrorRate: 1},
		SMTP:      &config.SMTPFault{Code: 421},
		SNMP:      &config.SNMPFault{Message: "err"},
		DNS:       &config.DNSFault{Rcode: "NXDOMAIN"},
		AMQP:      &config.AMQPFault{ErrorRate: 1},
		Kafka:     &config.KafkaFault{ErrorCode: 5},
		LDAP:      &config.LDAPFault{ResultCode: 52},
		IMAP:      &config.IMAPFault{Response: "NO"},
		FTP:       &config.FTPFault{Code: 421},
		Memcached: &config.MemcachedFault{ErrorType: "SERVER_ERROR"},
		STOMP:     &config.STOMPFault{Message: "err"},
		CoAP:      &config.CoAPFault{Code: "5.00"},
		SIP:       &config.SIPFault{Status: 503},
	})

	protos := []string{
		"http", "graphql", "websocket", "grpc", "tcp", "redis", "mqtt",
		"smtp", "snmp", "dns", "amqp", "kafka", "ldap", "imap", "ftp",
		"memcached", "stomp", "coap", "sip",
	}
	for _, proto := range protos {
		if ok := s.ClearDirectProtocolFault(proto); !ok {
			t.Errorf("protocol %q: ClearDirectProtocolFault returned false", proto)
		}
		got, _ := s.GetDirectProtocolFault(proto)
		if !isFaultNil(got) {
			t.Errorf("protocol %q: expected nil after clear, got %+v", proto, got)
		}
	}
}

func TestClearDirectProtocolFault_UnknownReturnsFalse(t *testing.T) {
	s := scenarios.New(nil)
	if ok := s.ClearDirectProtocolFault("unknown"); ok {
		t.Fatal("ClearDirectProtocolFault should return false for unknown protocol")
	}
}

// ---------------------------------------------------------------------------
// GetEffectiveProtocolFault
// ---------------------------------------------------------------------------

func TestGetEffectiveProtocolFault_UnknownReturnsFalse(t *testing.T) {
	s := scenarios.New(nil)
	_, ok := s.GetEffectiveProtocolFault("unknown")
	if ok {
		t.Fatal("GetEffectiveProtocolFault should return ok=false for unknown protocol")
	}
}

func TestGetEffectiveProtocolFault_DirectFaultReturned(t *testing.T) {
	s := scenarios.New(nil)
	_ = s.SetDirectProtocolFaultJSON("http", []byte(`{"status":503}`))
	fault, ok := s.GetEffectiveProtocolFault("http")
	if !ok {
		t.Fatal("http is a known protocol")
	}
	if isFaultNil(fault) {
		t.Fatal("expected non-nil effective fault")
	}
}

func TestGetEffectiveProtocolFault_ScenarioOverridesDirect(t *testing.T) {
	sc := config.Scenario{
		ID:     "sc1",
		Faults: &config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 429}},
	}
	s := scenarios.New([]config.Scenario{sc})
	_ = s.SetDirectProtocolFaultJSON("http", []byte(`{"status":503}`))
	s.Activate("sc1")

	fault, ok := s.GetEffectiveProtocolFault("http")
	if !ok {
		t.Fatal("http is a known protocol")
	}
	httpFault, isHTTP := fault.(*config.HTTPFault)
	if !isHTTP || httpFault == nil || httpFault.Status != 429 {
		t.Fatalf("expected scenario fault (429), got %+v", fault)
	}
}

func TestGetEffectiveProtocolFault_AllProtocolsKnown(t *testing.T) {
	s := scenarios.New(nil)
	protocols := []string{
		"http", "graphql", "websocket", "grpc", "tcp", "redis", "mqtt",
		"smtp", "snmp", "dns", "amqp", "kafka", "ldap", "imap", "ftp",
		"memcached", "stomp", "coap", "sip",
	}
	for _, proto := range protocols {
		_, ok := s.GetEffectiveProtocolFault(proto)
		if !ok {
			t.Errorf("protocol %q should be known", proto)
		}
	}
}
