package config

import (
	"strings"
	"testing"
)

func TestValidateProtocolVersionAccepted(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"mqtt 3.1 ok", func(c *Config) { c.Protocols.MQTT = &MQTTConfig{Enabled: true, Port: 1883, ProtocolVersion: "3.1"} }, false},
		{"mqtt 3.1.1 ok", func(c *Config) { c.Protocols.MQTT = &MQTTConfig{Enabled: true, Port: 1883, ProtocolVersion: "3.1.1"} }, false},
		{"mqtt 5.0 ok", func(c *Config) { c.Protocols.MQTT = &MQTTConfig{Enabled: true, Port: 1883, ProtocolVersion: "5.0"} }, false},
		{"mqtt empty ok", func(c *Config) { c.Protocols.MQTT = &MQTTConfig{Enabled: true, Port: 1883} }, false},
		{"mqtt unsupported", func(c *Config) { c.Protocols.MQTT = &MQTTConfig{Enabled: true, Port: 1883, ProtocolVersion: "9.9"} }, true},
		{"amqp 0.9.1 ok", func(c *Config) { c.Protocols.AMQP = &AMQPConfig{Enabled: true, Port: 5672, ProtocolVersion: "0.9.1"} }, false},
		{"amqp 1.0 not yet supported", func(c *Config) { c.Protocols.AMQP = &AMQPConfig{Enabled: true, Port: 5672, ProtocolVersion: "1.0"} }, true},
		{"snmp v1 ok", func(c *Config) { c.Protocols.SNMP = &SNMPConfig{Enabled: true, Port: 1161, ProtocolVersion: "v1"} }, false},
		{"snmp v2c ok", func(c *Config) { c.Protocols.SNMP = &SNMPConfig{Enabled: true, Port: 1161, ProtocolVersion: "v2c"} }, false},
		{"snmp v3 ok", func(c *Config) { c.Protocols.SNMP = &SNMPConfig{Enabled: true, Port: 1161, ProtocolVersion: "v3"} }, false},
		{"snmp unsupported", func(c *Config) { c.Protocols.SNMP = &SNMPConfig{Enabled: true, Port: 1161, ProtocolVersion: "v4"} }, true},
		{"ldap v3 ok", func(c *Config) { c.Protocols.LDAP = &LDAPConfig{Enabled: true, Port: 389, ProtocolVersion: "v3"} }, false},
		{"ldap v2 not yet supported", func(c *Config) { c.Protocols.LDAP = &LDAPConfig{Enabled: true, Port: 389, ProtocolVersion: "v2"} }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaults()
			tc.mutate(&cfg)
			errs := Validate(&cfg)
			if tc.wantErr && len(errs) == 0 {
				t.Fatalf("expected a validation error, got none")
			}
			if !tc.wantErr && len(errs) != 0 {
				t.Fatalf("expected no validation errors, got %v", errs)
			}
		})
	}
}

func TestValidateProtocolVersionErrorListsSupportedVersions(t *testing.T) {
	cfg := defaults()
	cfg.Protocols.LDAP = &LDAPConfig{Enabled: true, Port: 389, ProtocolVersion: "v2"}
	errs := Validate(&cfg)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %v", len(errs), errs)
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, `protocols.ldap.protocol_version`) || !strings.Contains(msg, `"v2"`) || !strings.Contains(msg, "v3") {
		t.Fatalf("expected error to name the field, the bad value, and the supported list, got: %s", msg)
	}
}
