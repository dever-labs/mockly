package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level Mockly configuration.
type Config struct {
	Mockly    MocklyConfig    `yaml:"mockly" json:"mockly"`
	Protocols ProtocolsConfig `yaml:"protocols" json:"protocols"`
	Scenarios []Scenario      `yaml:"scenarios,omitempty" json:"scenarios,omitempty"`
}

type MocklyConfig struct {
	UI  UIConfig  `yaml:"ui" json:"ui"`
	API APIConfig `yaml:"api" json:"api"`
}

type UIConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Port    int  `yaml:"port" json:"port"`
}

type APIConfig struct {
	Port int `yaml:"port" json:"port"`
}

// ProtocolsConfig holds configuration for each supported protocol.
type ProtocolsConfig struct {
	HTTP      *HTTPConfig      `yaml:"http,omitempty" json:"http,omitempty"`
	WebSocket *WebSocketConfig `yaml:"websocket,omitempty" json:"websocket,omitempty"`
	GRPC      *GRPCConfig      `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	GraphQL   *GraphQLConfig   `yaml:"graphql,omitempty" json:"graphql,omitempty"`
	TCP       *TCPConfig       `yaml:"tcp,omitempty" json:"tcp,omitempty"`
	Redis     *RedisConfig     `yaml:"redis,omitempty" json:"redis,omitempty"`
	SMTP      *SMTPConfig      `yaml:"smtp,omitempty" json:"smtp,omitempty"`
	MQTT      *MQTTConfig      `yaml:"mqtt,omitempty" json:"mqtt,omitempty"`
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

type HTTPConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Port    int        `yaml:"port" json:"port"`
	Mocks   []HTTPMock `yaml:"mocks" json:"mocks"`
}

type HTTPMock struct {
	ID       string          `yaml:"id" json:"id"`
	Request  HTTPRequest     `yaml:"request" json:"request"`
	Response HTTPResponse    `yaml:"response" json:"response"`
	State    *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type HTTPRequest struct {
	Method  string            `yaml:"method" json:"method"`
	Path    string            `yaml:"path" json:"path"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty" json:"body,omitempty"`
}

type HTTPResponse struct {
	Status  int               `yaml:"status" json:"status"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty" json:"body,omitempty"`
	Delay   Duration          `yaml:"delay,omitempty" json:"delay,omitempty"`
}

// ---------------------------------------------------------------------------
// WebSocket
// ---------------------------------------------------------------------------

type WebSocketConfig struct {
	Enabled bool            `yaml:"enabled" json:"enabled"`
	Port    int             `yaml:"port" json:"port"`
	Mocks   []WebSocketMock `yaml:"mocks" json:"mocks"`
}

type WebSocketMock struct {
	ID        string           `yaml:"id" json:"id"`
	Path      string           `yaml:"path" json:"path"`
	OnConnect *WebSocketAction `yaml:"on_connect,omitempty" json:"on_connect,omitempty"`
	OnMessage []WebSocketRule  `yaml:"on_message,omitempty" json:"on_message,omitempty"`
	State     *StateCondition  `yaml:"state,omitempty" json:"state,omitempty"`
}

type WebSocketAction struct {
	Send  string   `yaml:"send,omitempty" json:"send,omitempty"`
	Delay Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
}

type WebSocketRule struct {
	Match   string   `yaml:"match" json:"match"`
	Respond string   `yaml:"respond,omitempty" json:"respond,omitempty"`
	Close   bool     `yaml:"close,omitempty" json:"close,omitempty"`
	Delay   Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
}

// ---------------------------------------------------------------------------
// gRPC
// ---------------------------------------------------------------------------

type GRPCConfig struct {
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	Port     int           `yaml:"port" json:"port"`
	Services []GRPCService `yaml:"services" json:"services"`
}

type GRPCService struct {
	Proto string     `yaml:"proto" json:"proto"`
	Mocks []GRPCMock `yaml:"mocks" json:"mocks"`
}

type GRPCMock struct {
	ID       string                 `yaml:"id" json:"id"`
	Method   string                 `yaml:"method" json:"method"`
	Response map[string]interface{} `yaml:"response" json:"response,omitempty"`
	Error    *GRPCError             `yaml:"error,omitempty" json:"error,omitempty"`
	Delay    Duration               `yaml:"delay,omitempty" json:"delay,omitempty"`
}

type GRPCError struct {
	Code    int    `yaml:"code" json:"code"`
	Message string `yaml:"message" json:"message"`
}

// ---------------------------------------------------------------------------
// GraphQL
// ---------------------------------------------------------------------------

type GraphQLConfig struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Port    int           `yaml:"port" json:"port"`
	Path    string        `yaml:"path,omitempty" json:"path,omitempty"` // default: /graphql
	Mocks   []GraphQLMock `yaml:"mocks" json:"mocks"`
}

// GraphQLMock matches an incoming GraphQL operation and returns a response.
type GraphQLMock struct {
	ID            string                 `yaml:"id" json:"id"`
	OperationType string                 `yaml:"operation_type,omitempty" json:"operation_type,omitempty"` // query|mutation|subscription (empty = any)
	OperationName string                 `yaml:"operation_name,omitempty" json:"operation_name,omitempty"` // exact or wildcard, empty = any
	Response      map[string]interface{} `yaml:"response,omitempty" json:"response,omitempty"`              // data field
	Errors        []GraphQLError         `yaml:"errors,omitempty" json:"errors,omitempty"`
	Delay         Duration               `yaml:"delay,omitempty" json:"delay,omitempty"`
	State         *StateCondition        `yaml:"state,omitempty" json:"state,omitempty"`
}

type GraphQLError struct {
	Message    string                 `yaml:"message" json:"message"`
	Path       []string               `yaml:"path,omitempty" json:"path,omitempty"`
	Extensions map[string]interface{} `yaml:"extensions,omitempty" json:"extensions,omitempty"`
}

// ---------------------------------------------------------------------------
// TCP
// ---------------------------------------------------------------------------

type TCPConfig struct {
	Enabled bool      `yaml:"enabled" json:"enabled"`
	Port    int       `yaml:"port" json:"port"`
	Mocks   []TCPMock `yaml:"mocks" json:"mocks"`
}

// TCPMock matches incoming raw TCP data and sends a response.
// Match is an exact string, a "re:…" regex, or "hex:…" hex bytes.
// Response can be a plain string or "hex:…" hex bytes.
type TCPMock struct {
	ID       string          `yaml:"id" json:"id"`
	Match    string          `yaml:"match" json:"match"`
	Response string          `yaml:"response" json:"response"`
	Close    bool            `yaml:"close,omitempty" json:"close,omitempty"`
	Delay    Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State    *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

// ---------------------------------------------------------------------------
// Redis
// ---------------------------------------------------------------------------

type RedisConfig struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Port    int         `yaml:"port" json:"port"`
	Mocks   []RedisMock `yaml:"mocks" json:"mocks"`
}

// RedisMock matches an incoming Redis command and returns a configured response.
// Key supports exact, "re:…" regex, and "*" wildcards.
type RedisMock struct {
	ID       string          `yaml:"id" json:"id"`
	Command  string          `yaml:"command" json:"command"` // e.g. GET, SET, HGET
	Key      string          `yaml:"key,omitempty" json:"key,omitempty"`
	Response RedisResponse   `yaml:"response" json:"response"`
	Delay    Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State    *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

// RedisResponse holds the value to return for a matched Redis command.
// Type is one of: string | bulk | integer | array | error | nil.
// For type "array", Value should be a []interface{} (each element a string).
type RedisResponse struct {
	Type  string      `yaml:"type" json:"type"`
	Value interface{} `yaml:"value,omitempty" json:"value,omitempty"`
}

// ---------------------------------------------------------------------------
// SMTP
// ---------------------------------------------------------------------------

type SMTPConfig struct {
	Enabled    bool       `yaml:"enabled" json:"enabled"`
	Port       int        `yaml:"port" json:"port"`
	Domain     string     `yaml:"domain,omitempty" json:"domain,omitempty"` // default: mockly.local
	MaxEmails  int        `yaml:"max_emails,omitempty" json:"max_emails,omitempty"`
	Rules      []SMTPRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// SMTPRule defines accept/reject behaviour for incoming emails.
// From, To, Subject are matched as exact strings, "re:…" regexes, or wildcards.
// Action is "accept" (default) or "reject".
type SMTPRule struct {
	ID      string `yaml:"id" json:"id"`
	From    string `yaml:"from,omitempty" json:"from,omitempty"`
	To      string `yaml:"to,omitempty" json:"to,omitempty"`
	Subject string `yaml:"subject,omitempty" json:"subject,omitempty"`
	Action  string `yaml:"action" json:"action"` // accept | reject
	Message string `yaml:"message,omitempty" json:"message,omitempty"` // SMTP reject error message
}

// ReceivedEmail is a captured inbound email stored in the SMTP inbox.
type ReceivedEmail struct {
	ID          string   `json:"id"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	ReceivedAt  string   `json:"received_at"`
}

// ---------------------------------------------------------------------------
// MQTT
// ---------------------------------------------------------------------------

type MQTTConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Port    int        `yaml:"port" json:"port"`
	Mocks   []MQTTMock `yaml:"mocks" json:"mocks"`
}

// MQTTMock subscribes to a topic pattern. When a message arrives, the broker
// publishes Response to ResponseTopic (defaults to <topic>/response).
// Topic supports MQTT wildcards: + (single level) and # (multi-level).
type MQTTMock struct {
	ID            string          `yaml:"id" json:"id"`
	Topic         string          `yaml:"topic" json:"topic"`
	Response      *MQTTResponse   `yaml:"response,omitempty" json:"response,omitempty"`
	State         *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type MQTTResponse struct {
	Topic   string   `yaml:"topic" json:"topic"` // publish on this topic; empty = <incoming>/response
	Payload string   `yaml:"payload" json:"payload"`
	QoS     byte     `yaml:"qos,omitempty" json:"qos,omitempty"`
	Retain  bool     `yaml:"retain,omitempty" json:"retain,omitempty"`
	Delay   Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
}

// ---------------------------------------------------------------------------
// Shared
// ---------------------------------------------------------------------------

// StateCondition allows mocks to fire only when a named state value matches.
type StateCondition struct {
	Key   string `yaml:"key" json:"key"`
	Value string `yaml:"value" json:"value"`
}

// ---------------------------------------------------------------------------
// Scenarios & Fault injection
// ---------------------------------------------------------------------------

// Scenario is a named set of mock patches that can be activated/deactivated
// at runtime via the management API. Dependency teams ship these alongside
// their preset configs to let consuming teams toggle error states, latency,
// and other failure modes deterministically.
type Scenario struct {
	ID          string      `yaml:"id" json:"id"`
	Name        string      `yaml:"name" json:"name"`
	Description string      `yaml:"description" json:"description"`
	Patches     []MockPatch `yaml:"patches" json:"patches"`
}

// MockPatch overrides specific response fields for a named mock when a
// scenario is active. Only non-zero/non-nil fields are applied.
type MockPatch struct {
	MockID   string            `yaml:"mock_id" json:"mock_id"`
	Status   int               `yaml:"status,omitempty" json:"status,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body     string            `yaml:"body,omitempty" json:"body,omitempty"`
	Delay    *Duration         `yaml:"delay,omitempty" json:"delay,omitempty"`
	Disabled bool              `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

// GlobalFault injects faults across all mock responses. Delay adds latency to
// every request. StatusOverride replaces response status codes at the given
// ErrorRate probability (0.0–1.0; omit or set to 0 to always override).
type GlobalFault struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	StatusOverride int      `yaml:"status_override,omitempty" json:"status_override,omitempty"`
	Delay          Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Body           string   `yaml:"body,omitempty" json:"body,omitempty"`
	ErrorRate      float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

// Duration is a yaml-decodable time.Duration.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(text), err)
	}
	d.Duration = dur
	return nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" || s == "0s" {
		d.Duration = 0
		return nil
	}
	return d.UnmarshalText([]byte(s))
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// ---------------------------------------------------------------------------
// Defaults & Loading
// ---------------------------------------------------------------------------

func defaults() Config {
	return Config{
		Mockly: MocklyConfig{
			UI:  UIConfig{Enabled: true, Port: 9090},
			API: APIConfig{Port: 9091},
		},
	}
}

// Load reads and parses a YAML config file. Missing file returns defaults.
func Load(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

// Save writes the config back to the given path as YAML.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func applyDefaults(cfg *Config) {
	if cfg.Mockly.UI.Port == 0 {
		cfg.Mockly.UI.Port = 9090
	}
	if cfg.Mockly.API.Port == 0 {
		cfg.Mockly.API.Port = 9091
	}
	if cfg.Protocols.HTTP != nil && cfg.Protocols.HTTP.Port == 0 {
		cfg.Protocols.HTTP.Port = 8080
	}
	if cfg.Protocols.WebSocket != nil && cfg.Protocols.WebSocket.Port == 0 {
		cfg.Protocols.WebSocket.Port = 8081
	}
	if cfg.Protocols.GRPC != nil && cfg.Protocols.GRPC.Port == 0 {
		cfg.Protocols.GRPC.Port = 50051
	}
	if cfg.Protocols.GraphQL != nil {
		if cfg.Protocols.GraphQL.Port == 0 {
			cfg.Protocols.GraphQL.Port = 8082
		}
		if cfg.Protocols.GraphQL.Path == "" {
			cfg.Protocols.GraphQL.Path = "/graphql"
		}
	}
	if cfg.Protocols.TCP != nil && cfg.Protocols.TCP.Port == 0 {
		cfg.Protocols.TCP.Port = 8083
	}
	if cfg.Protocols.Redis != nil && cfg.Protocols.Redis.Port == 0 {
		cfg.Protocols.Redis.Port = 6379
	}
	if cfg.Protocols.SMTP != nil {
		if cfg.Protocols.SMTP.Port == 0 {
			cfg.Protocols.SMTP.Port = 2525
		}
		if cfg.Protocols.SMTP.Domain == "" {
			cfg.Protocols.SMTP.Domain = "mockly.local"
		}
		if cfg.Protocols.SMTP.MaxEmails == 0 {
			cfg.Protocols.SMTP.MaxEmails = 1000
		}
	}
	if cfg.Protocols.MQTT != nil && cfg.Protocols.MQTT.Port == 0 {
		cfg.Protocols.MQTT.Port = 1883
	}
}
