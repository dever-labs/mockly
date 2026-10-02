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
