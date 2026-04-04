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
}
