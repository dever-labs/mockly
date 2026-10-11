package config

import (
	"strings"
	"testing"
)

func TestValidateNoErrorsOnDefaultConfig(t *testing.T) {
	cfg := defaults()
	if errs := Validate(&cfg); len(errs) != 0 {
		t.Fatalf("expected no errors on default config, got %v", errs)
	}
}

func TestValidateDetectsDuplicateMockIDs(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Mocks: []HTTPMock{
			{ID: "dup", Request: HTTPRequest{Method: "GET", Path: "/a"}},
			{ID: "dup", Request: HTTPRequest{Method: "GET", Path: "/b"}},
			{ID: "unique", Request: HTTPRequest{Method: "GET", Path: "/c"}},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 duplicate-id error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), `duplicate mock id "dup"`) {
		t.Fatalf("unexpected error message: %v", errs[0])
	}
}

func TestValidateDetectsInvalidPathRegex(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Mocks: []HTTPMock{
			{ID: "bad-regex", Request: HTTPRequest{Method: "GET", PathRegex: "(unclosed"}},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 invalid-regex error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "invalid regex") {
		t.Fatalf("unexpected error message: %v", errs[0])
	}
}

func TestValidateDetectsInvalidBase64MatchBinary(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.WebSocket = &WebSocketConfig{
		Enabled: true,
		Port:    8090,
		Mocks: []WebSocketMock{
			{
				ID:   "bin",
				Path: "/ws",
				OnMessage: []WebSocketRule{
					{MatchBinary: "not-valid-base64!!!"},
				},
			},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 invalid-base64 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "invalid base64") {
		t.Fatalf("unexpected error message: %v", errs[0])
	}
}

func TestValidateAllowsValidBase64BinaryFields(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.WebSocket = &WebSocketConfig{
		Enabled: true,
		Port:    8090,
		Mocks: []WebSocketMock{
			{
				ID:        "bin",
				Path:      "/ws",
				OnConnect: &WebSocketAction{SendBinary: "AQID"},
				OnMessage: []WebSocketRule{
					{MatchBinary: "AQID", RespondBinary: "BAUG"},
				},
			},
		},
	}
	if errs := Validate(&cfg); len(errs) != 0 {
		t.Fatalf("expected no errors for valid base64, got %v", errs)
	}
}

func TestValidateDetectsInvalidReprefixedRegex(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Mocks: []HTTPMock{
			{
				ID: "bad-header-regex",
				Request: HTTPRequest{
					Method:  "GET",
					Path:    "/x",
					Headers: map[string]string{"X-Test": "re:(unclosed"},
				},
			},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 invalid-regex error, got %d: %v", len(errs), errs)
	}
}

func TestValidateAllowsPlainStringsAndValidRegexes(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Mocks: []HTTPMock{
			{
				ID: "ok",
				Request: HTTPRequest{
					Method:    "GET",
					PathRegex: "^/users/[0-9]+$",
					Headers:   map[string]string{"X-Test": "re:^abc$", "X-Plain": "just-a-string"},
				},
			},
		},
	}
	if errs := Validate(&cfg); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateAMQPTopology_UnsupportedExchangeType(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.AMQP = &AMQPConfig{
		Enabled:   true,
		Port:      5672,
		Exchanges: []AMQPExchange{{Name: "orders", Type: "headers"}},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), `unsupported type "headers"`) {
		t.Fatalf("unexpected error message: %v", errs[0])
	}
}

func TestValidateAMQPTopology_BindingMissingExchangeOrQueue(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.AMQP = &AMQPConfig{
		Enabled: true,
		Port:    5672,
		Bindings: []AMQPBinding{
			{Exchange: "", Queue: ""},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 2 {
		t.Fatalf("expected exactly 2 errors (missing exchange + missing queue), got %d: %v", len(errs), errs)
	}
}

func TestValidateAMQPTopology_ValidConfig(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.AMQP = &AMQPConfig{
		Enabled:   true,
		Port:      5672,
		Exchanges: []AMQPExchange{{Name: "orders", Type: "topic"}},
		Bindings: []AMQPBinding{
			{Exchange: "orders", RoutingKeyPattern: "orders.*.created", Queue: "created-queue"},
		},
		Mocks: []AMQPMock{
			{ID: "created", Queue: "created-queue", Response: &AMQPResponse{Body: "ok"}},
		},
	}
	if errs := Validate(&cfg); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateTLSConfigs_RejectsOnlyCertFileSet(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		TLS:     &TLSConfig{Enabled: true, CertFile: "cert.pem"},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %v", errs)
	}
	if !strings.Contains(errs[0].Error(), "protocols.http.tls") {
		t.Errorf("error %q does not mention protocols.http.tls", errs[0])
	}
}

func TestValidateTLSConfigs_RejectsOnlyKeyFileSet(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.TCP = &TCPConfig{
		Enabled: true,
		Port:    9000,
		TLS:     &TLSConfig{Enabled: true, KeyFile: "key.pem"},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %v", errs)
	}
	if !strings.Contains(errs[0].Error(), "protocols.tcp.tls") {
		t.Errorf("error %q does not mention protocols.tcp.tls", errs[0])
	}
}

func TestValidateTLSConfigs_AllowsBothSetOrBothEmpty(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		TLS:     &TLSConfig{Enabled: true, CertFile: "cert.pem", KeyFile: "key.pem"},
	}
	cfg.Protocols.WebSocket = &WebSocketConfig{
		Enabled: true,
		Port:    8081,
		TLS:     &TLSConfig{Enabled: true}, // implicit generation: both empty is fine
	}
	if errs := Validate(&cfg); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateHTTPListeners_ValidConfig(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Listeners: []HTTPListener{
			{Name: "secure", Port: 8443},
		},
		Mocks: []HTTPMock{
			{ID: "public", Request: HTTPRequest{Method: "GET", Path: "/public"}},
			{ID: "secret", Listener: "secure", Request: HTTPRequest{Method: "GET", Path: "/secret"}},
		},
	}
	if errs := Validate(&cfg); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateHTTPListeners_EmptyName(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled:   true,
		Port:      8080,
		Listeners: []HTTPListener{{Name: "", Port: 8443}},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "name must not be empty") {
		t.Fatalf("expected a 'name must not be empty' error, got %v", errs)
	}
}

func TestValidateHTTPListeners_DuplicateName(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Listeners: []HTTPListener{
			{Name: "secure", Port: 8443},
			{Name: "secure", Port: 8444},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "duplicate listener name") {
		t.Fatalf("expected a 'duplicate listener name' error, got %v", errs)
	}
}

func TestValidateHTTPListeners_PortCollidesWithDefault(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled:   true,
		Port:      8080,
		Listeners: []HTTPListener{{Name: "secure", Port: 8080}},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "already used by") {
		t.Fatalf("expected a port-collision error, got %v", errs)
	}
}

func TestValidateHTTPListeners_PortCollidesBetweenListeners(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Listeners: []HTTPListener{
			{Name: "a", Port: 9000},
			{Name: "b", Port: 9000},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "already used by") {
		t.Fatalf("expected a port-collision error, got %v", errs)
	}
}

func TestValidateHTTPListeners_MockReferencesUnknownListener(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled:   true,
		Port:      8080,
		Listeners: []HTTPListener{{Name: "secure", Port: 8443}},
		Mocks: []HTTPMock{
			{ID: "bad", Listener: "nonexistent", Request: HTTPRequest{Method: "GET", Path: "/x"}},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "is not declared") {
		t.Fatalf("expected an unknown-listener error, got %v", errs)
	}
}

func TestValidateHTTPListeners_OwnTLSBothOrNeitherEnforced(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.HTTP = &HTTPConfig{
		Enabled: true,
		Port:    8080,
		Listeners: []HTTPListener{
			{Name: "secure", Port: 8443, TLS: &TLSConfig{Enabled: true, CertFile: "cert.pem"}},
		},
	}
	errs := Validate(&cfg)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "protocols.http.listeners[0]") {
		t.Fatalf("expected a listener TLS cert/key error, got %v", errs)
	}
}
