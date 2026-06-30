package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// Sequence exhaustion strategies for HTTPMock.SequenceExhausted.
const (
	SequenceExhaustedHoldLast = "hold_last" // keep returning the last entry (default)
	SequenceExhaustedLoop     = "loop"      // restart from the first entry
	SequenceExhaustedNotFound = "not_found" // return 404
)

// Default capacities for bounded in-memory stores.
const (
	DefaultInboxSize        = 1000
	DefaultMessageStoreSize = 1000
)

// Default network constants.
const (
	DefaultTCPReadBufferSize = 65536
	DefaultTCPReadDeadline   = 30 * time.Second
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
	Port int        `yaml:"port" json:"port"`
	TLS  *TLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
	// CORS configures Cross-Origin Resource Sharing for the management API.
	// Defaults to wide-open (AllowedOrigins: ["*"]) for maximum compatibility
	// as a local mock tool. Set Enabled: false to strip the CORS middleware
	// entirely (useful when running behind a reverse proxy that handles CORS).
	CORS *CORSConfig `yaml:"cors,omitempty" json:"cors,omitempty"`
}

// CORSConfig controls the CORS middleware on the management API server.
// All fields are optional; omitting the cors block keeps the default
// wide-open behaviour (AllowedOrigins: ["*"]).
type CORSConfig struct {
	// Enabled controls whether CORS headers are sent at all.
	// Set to false to disable the middleware entirely (default: true).
	Enabled        *bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	AllowedOrigins []string `yaml:"allowed_origins,omitempty" json:"allowed_origins,omitempty"`
	AllowedMethods []string `yaml:"allowed_methods,omitempty" json:"allowed_methods,omitempty"`
	AllowedHeaders []string `yaml:"allowed_headers,omitempty" json:"allowed_headers,omitempty"`
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
	SNMP      *SNMPConfig      `yaml:"snmp,omitempty" json:"snmp,omitempty"`
	DNS       *DNSConfig       `yaml:"dns,omitempty" json:"dns,omitempty"`
	AMQP      *AMQPConfig      `yaml:"amqp,omitempty" json:"amqp,omitempty"`
	Kafka     *KafkaConfig     `yaml:"kafka,omitempty" json:"kafka,omitempty"`
	LDAP      *LDAPConfig      `yaml:"ldap,omitempty" json:"ldap,omitempty"`
	IMAP      *IMAPConfig      `yaml:"imap,omitempty" json:"imap,omitempty"`
	FTP       *FTPConfig       `yaml:"ftp,omitempty" json:"ftp,omitempty"`
	Memcached *MemcachedConfig `yaml:"memcached,omitempty" json:"memcached,omitempty"`
	STOMP     *STOMPConfig     `yaml:"stomp,omitempty" json:"stomp,omitempty"`
	CoAP      *CoAPConfig      `yaml:"coap,omitempty" json:"coap,omitempty"`
	SIP       *SIPConfig       `yaml:"sip,omitempty" json:"sip,omitempty"`
}

// ---------------------------------------------------------------------------
// TLS
// ---------------------------------------------------------------------------

// TLSConfig enables TLS on a server. Set enabled: true and provide paths to
// a PEM-encoded certificate and private key file.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

type HTTPConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Port    int        `yaml:"port" json:"port"`
	TLS     *TLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
	// MaxBodyBytes limits the size of incoming request bodies in bytes.
	// 0 (default) means unlimited, which is appropriate when simulating
	// endpoints that accept large payloads (file uploads, bulk imports, etc.).
	MaxBodyBytes int64      `yaml:"max_body_bytes,omitempty" json:"max_body_bytes,omitempty"`
	Mocks        []HTTPMock `yaml:"mocks" json:"mocks"`
}

type HTTPMock struct {
	ID       string          `yaml:"id" json:"id"`
	Request  HTTPRequest     `yaml:"request" json:"request"`
	Response HTTPResponse    `yaml:"response" json:"response"`
	State    *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`

	// Sequence returns a different HTTPResponse on each successive call.
	// Once exhausted the behaviour is governed by SequenceExhausted:
	//   "loop"      — restart from the first entry
	//   "hold_last" — keep returning the last entry (default)
	//   "not_found" — return 404
	Sequence          []HTTPResponse `yaml:"sequence,omitempty" json:"sequence,omitempty"`
	SequenceExhausted string         `yaml:"sequence_exhausted,omitempty" json:"sequence_exhausted,omitempty"`

	// Fault overrides the response for this mock independently of the global fault.
	Fault *MockFault `yaml:"fault,omitempty" json:"fault,omitempty"`
}

// MockFault injects latency or error responses for a specific mock.
type MockFault struct {
	Delay          Duration          `yaml:"delay,omitempty" json:"delay,omitempty"`
	StatusOverride int               `yaml:"status_override,omitempty" json:"status_override,omitempty"`
	Body           string            `yaml:"body,omitempty" json:"body,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"` // Extra response headers
	ErrorRate      float64           `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type HTTPRequest struct {
	Method  string            `yaml:"method" json:"method"`
	Path    string            `yaml:"path" json:"path"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty" json:"body,omitempty"`

	// PathRegex matches the request path against a regular expression.
	// When set it takes priority over Path.
	PathRegex string `yaml:"path_regex,omitempty" json:"path_regex,omitempty"`

	// Query matches URL query parameters (exact value or "*" wildcard).
	Query map[string]string `yaml:"query,omitempty" json:"query,omitempty"`

	// BodyJSON matches fields in a JSON request body using dot-notation paths.
	// Example: {"user.role": "admin"} matches {"user":{"role":"admin"}}.
	BodyJSON map[string]string `yaml:"body_json,omitempty" json:"body_json,omitempty"`

	// Auth requires the incoming request to carry valid credentials.
	// When set, the mock is skipped if authentication fails — add a fallback
	// mock (without auth) to return a 401 for unauthenticated callers.
	// For type "ntlm" the server automatically handles the 3-step handshake.
	Auth *HTTPAuth `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// HTTPAuth configures authentication matching for an HTTP mock.
// Type selects the authentication scheme; the remaining fields depend on Type:
//
//   - bearer: Token is matched against the value in "Authorization: Bearer <token>".
//     Token supports exact strings, "re:…" regex, and "*" (any token present).
//
//   - basic: Username and Password are matched against the decoded Basic credentials.
//
//   - api_key: Value is matched against the key found in the named Header or Query param.
//     Value supports exact strings, "re:…" regex, and "*" (any value present).
//
//   - ntlm: The server performs a full 3-step NTLM challenge/response handshake.
//     Any well-formed NTLM token is accepted (no credential validation).
//
//   - digest: The mock matches when an Authorization: Digest header is present.
type HTTPAuth struct {
	// Type is the authentication scheme: bearer | basic | api_key | ntlm | digest.
	Type string `yaml:"type" json:"type"`

	// Token is the expected bearer token value (bearer only).
	// Supports exact match, "re:…" regex, and "*" (any non-empty token).
	Token string `yaml:"token,omitempty" json:"token,omitempty"`

	// Username and Password are the expected Basic auth credentials (basic only).
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Header is the request header name that carries the API key (api_key only).
	// Mutually exclusive with Query.
	Header string `yaml:"header,omitempty" json:"header,omitempty"`

	// Query is the URL query parameter name that carries the API key (api_key only).
	// Mutually exclusive with Header.
	Query string `yaml:"query,omitempty" json:"query,omitempty"`

	// Value is the expected API key value (api_key only).
	// Supports exact match, "re:…" regex, and "*" (any non-empty value).
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
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
	TLS     *TLSConfig      `yaml:"tls,omitempty" json:"tls,omitempty"`
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
	TLS     *TLSConfig    `yaml:"tls,omitempty" json:"tls,omitempty"`
	Mocks   []GraphQLMock `yaml:"mocks" json:"mocks"`
}

// GraphQLMock matches an incoming GraphQL operation and returns a response.
type GraphQLMock struct {
	ID            string                 `yaml:"id" json:"id"`
	OperationType string                 `yaml:"operation_type,omitempty" json:"operation_type,omitempty"` // query|mutation|subscription (empty = any)
	OperationName string                 `yaml:"operation_name,omitempty" json:"operation_name,omitempty"` // exact or wildcard, empty = any
	Response      map[string]interface{} `yaml:"response,omitempty" json:"response,omitempty"`             // data field
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
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Port    int        `yaml:"port" json:"port"`
	TLS     *TLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
	Mocks   []TCPMock  `yaml:"mocks" json:"mocks"`
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
	Enabled   bool       `yaml:"enabled" json:"enabled"`
	Port      int        `yaml:"port" json:"port"`
	Domain    string     `yaml:"domain,omitempty" json:"domain,omitempty"` // default: mockly.local
	MaxEmails int        `yaml:"max_emails,omitempty" json:"max_emails,omitempty"`
	Rules     []SMTPRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// SMTPRule defines accept/reject behaviour for incoming emails.
// From, To, Subject are matched as exact strings, "re:…" regexes, or wildcards.
// Action is "accept" (default) or "reject".
type SMTPRule struct {
	ID      string `yaml:"id" json:"id"`
	From    string `yaml:"from,omitempty" json:"from,omitempty"`
	To      string `yaml:"to,omitempty" json:"to,omitempty"`
	Subject string `yaml:"subject,omitempty" json:"subject,omitempty"`
	Action  string `yaml:"action" json:"action"`                       // accept | reject
	Message string `yaml:"message,omitempty" json:"message,omitempty"` // SMTP reject error message
}

// ReceivedEmail is a captured inbound email stored in the SMTP inbox.
type ReceivedEmail struct {
	ID         string   `json:"id"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	Subject    string   `json:"subject"`
	Body       string   `json:"body"`
	ReceivedAt string   `json:"received_at"`
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
	ID       string          `yaml:"id" json:"id"`
	Topic    string          `yaml:"topic" json:"topic"`
	Response *MQTTResponse   `yaml:"response,omitempty" json:"response,omitempty"`
	State    *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type MQTTResponse struct {
	Topic   string   `yaml:"topic" json:"topic"` // publish on this topic; empty = <incoming>/response
	Payload string   `yaml:"payload" json:"payload"`
	QoS     byte     `yaml:"qos,omitempty" json:"qos,omitempty"`
	Retain  bool     `yaml:"retain,omitempty" json:"retain,omitempty"`
	Delay   Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
}

// ---------------------------------------------------------------------------
// SNMP
// ---------------------------------------------------------------------------

// SNMPConfig configures the SNMP agent mock server.
type SNMPConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Port    int  `yaml:"port" json:"port"`
	// Community is the v1/v2c community string accepted by the agent (default: public).
	Community string `yaml:"community,omitempty" json:"community,omitempty"`
	// V3Users lists SNMPv3 USM user credentials.
	V3Users []SNMPUser `yaml:"v3_users,omitempty" json:"v3_users,omitempty"`
	Mocks   []SNMPMock `yaml:"mocks,omitempty" json:"mocks,omitempty"`
	// Traps lists outbound TRAP configurations that can be triggered via API.
	Traps []SNMPTrap `yaml:"traps,omitempty" json:"traps,omitempty"`
}

// SNMPMock defines a single OID value returned by the agent.
// Type is one of: string, integer, gauge32, counter32, counter64, timeticks, ipaddress, objectidentifier.
// Value is the raw value (string is marshalled from YAML; numeric types accepted as int or string).
type SNMPMock struct {
	ID    string          `yaml:"id" json:"id"`
	OID   string          `yaml:"oid" json:"oid"`
	Type  string          `yaml:"type" json:"type"`
	Value interface{}     `yaml:"value" json:"value"`
	State *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

// SNMPUser defines a SNMPv3 USM user credential set.
type SNMPUser struct {
	Username       string `yaml:"username" json:"username"`
	AuthProtocol   string `yaml:"auth_protocol,omitempty" json:"auth_protocol,omitempty"` // md5|sha|sha224|sha256|sha384|sha512
	AuthPassphrase string `yaml:"auth_passphrase,omitempty" json:"auth_passphrase,omitempty"`
	PrivProtocol   string `yaml:"priv_protocol,omitempty" json:"priv_protocol,omitempty"` // des|aes|aes192|aes256
	PrivPassphrase string `yaml:"priv_passphrase,omitempty" json:"priv_passphrase,omitempty"`
}

// SNMPTrap is an outbound TRAP definition that can be triggered via the management API.
type SNMPTrap struct {
	ID        string            `yaml:"id" json:"id"`
	Target    string            `yaml:"target" json:"target"`                       // host:port
	Version   string            `yaml:"version,omitempty" json:"version,omitempty"` // "1"|"2c"|"3"
	Community string            `yaml:"community,omitempty" json:"community,omitempty"`
	OID       string            `yaml:"oid" json:"oid"`
	Bindings  []SNMPTrapBinding `yaml:"bindings,omitempty" json:"bindings,omitempty"`
}

// SNMPTrapBinding is a variable binding attached to an outbound TRAP.
type SNMPTrapBinding struct {
	OID   string      `yaml:"oid" json:"oid"`
	Type  string      `yaml:"type" json:"type"`
	Value interface{} `yaml:"value" json:"value"`
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
// DNS
// ---------------------------------------------------------------------------

type DNSConfig struct {
	Enabled bool      `yaml:"enabled" json:"enabled"`
	Port    int       `yaml:"port" json:"port"`
	Mocks   []DNSMock `yaml:"mocks" json:"mocks"`
}

type DNSMock struct {
	ID      string          `yaml:"id" json:"id"`
	Name    string          `yaml:"name" json:"name"`
	Type    string          `yaml:"type" json:"type"`
	Records []string        `yaml:"records" json:"records"`
	TTL     uint32          `yaml:"ttl,omitempty" json:"ttl,omitempty"`
	Delay   Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State   *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

// ---------------------------------------------------------------------------
// AMQP
// ---------------------------------------------------------------------------

type AMQPConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Port    int        `yaml:"port" json:"port"`
	Mocks   []AMQPMock `yaml:"mocks" json:"mocks"`
}

type AMQPMock struct {
	ID         string          `yaml:"id" json:"id"`
	Exchange   string          `yaml:"exchange,omitempty" json:"exchange,omitempty"`
	RoutingKey string          `yaml:"routing_key,omitempty" json:"routing_key,omitempty"`
	Response   *AMQPResponse   `yaml:"response,omitempty" json:"response,omitempty"`
	Delay      Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State      *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type AMQPResponse struct {
	Exchange   string `yaml:"exchange,omitempty" json:"exchange,omitempty"`
	RoutingKey string `yaml:"routing_key,omitempty" json:"routing_key,omitempty"`
	Body       string `yaml:"body" json:"body"`
}

type ReceivedAMQPMessage struct {
	ID         string `json:"id"`
	Exchange   string `json:"exchange"`
	RoutingKey string `json:"routing_key"`
	Body       string `json:"body"`
	Timestamp  string `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Kafka
// ---------------------------------------------------------------------------

type KafkaConfig struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Port    int         `yaml:"port" json:"port"`
	Mocks   []KafkaMock `yaml:"mocks" json:"mocks"`
}

type KafkaMock struct {
	ID      string          `yaml:"id" json:"id"`
	Topic   string          `yaml:"topic" json:"topic"`
	Records []KafkaRecord   `yaml:"records" json:"records"`
	Delay   Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State   *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type KafkaRecord struct {
	Key     string            `yaml:"key,omitempty" json:"key,omitempty"`
	Value   string            `yaml:"value" json:"value"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type ProducedKafkaMessage struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	Partition int32  `json:"partition"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Timestamp string `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// LDAP
// ---------------------------------------------------------------------------

type LDAPConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Port    int        `yaml:"port" json:"port"`
	Mocks   []LDAPMock `yaml:"mocks" json:"mocks"`
}

type LDAPMock struct {
	ID         string              `yaml:"id" json:"id"`
	BaseDN     string              `yaml:"base_dn" json:"base_dn"`
	Filter     string              `yaml:"filter,omitempty" json:"filter,omitempty"`
	Attributes map[string][]string `yaml:"attributes" json:"attributes"`
	Delay      Duration            `yaml:"delay,omitempty" json:"delay,omitempty"`
	State      *StateCondition     `yaml:"state,omitempty" json:"state,omitempty"`
}

// ---------------------------------------------------------------------------
// IMAP
// ---------------------------------------------------------------------------

type IMAPConfig struct {
	Enabled   bool          `yaml:"enabled" json:"enabled"`
	Port      int           `yaml:"port" json:"port"`
	Users     []IMAPUser    `yaml:"users,omitempty" json:"users,omitempty"`
	Mailboxes []IMAPMailbox `yaml:"mailboxes" json:"mailboxes"`
}

type IMAPUser struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

type IMAPMailbox struct {
	ID       string        `yaml:"id" json:"id"`
	Name     string        `yaml:"name" json:"name"`
	Messages []IMAPMessage `yaml:"messages" json:"messages"`
}

type IMAPMessage struct {
	SeqNum  int      `yaml:"seq_num" json:"seq_num"`
	UID     uint32   `yaml:"uid,omitempty" json:"uid,omitempty"`
	From    string   `yaml:"from" json:"from"`
	To      string   `yaml:"to" json:"to"`
	Subject string   `yaml:"subject" json:"subject"`
	Body    string   `yaml:"body" json:"body"`
	Flags   []string `yaml:"flags,omitempty" json:"flags,omitempty"`
	Date    string   `yaml:"date,omitempty" json:"date,omitempty"`
}

// ---------------------------------------------------------------------------
// FTP
// ---------------------------------------------------------------------------

type FTPConfig struct {
	Enabled          bool      `yaml:"enabled" json:"enabled"`
	Port             int       `yaml:"port" json:"port"`
	PassivePortStart int       `yaml:"passive_port_start,omitempty" json:"passive_port_start,omitempty"`
	Files            []FTPFile `yaml:"files" json:"files"`
}

type FTPFile struct {
	ID          string `yaml:"id" json:"id"`
	Path        string `yaml:"path" json:"path"`
	Content     string `yaml:"content" json:"content"`
	Permissions string `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Size        int64  `yaml:"size,omitempty" json:"size,omitempty"`
}

// ---------------------------------------------------------------------------
// Memcached
// ---------------------------------------------------------------------------

type MemcachedConfig struct {
	Enabled bool            `yaml:"enabled" json:"enabled"`
	Port    int             `yaml:"port" json:"port"`
	Mocks   []MemcachedMock `yaml:"mocks" json:"mocks"`
}

type MemcachedMock struct {
	ID       string            `yaml:"id" json:"id"`
	Command  string            `yaml:"command" json:"command"`
	Key      string            `yaml:"key,omitempty" json:"key,omitempty"`
	Response MemcachedResponse `yaml:"response" json:"response"`
	Delay    Duration          `yaml:"delay,omitempty" json:"delay,omitempty"`
	State    *StateCondition   `yaml:"state,omitempty" json:"state,omitempty"`
}

type MemcachedResponse struct {
	Value  string `yaml:"value,omitempty" json:"value,omitempty"`
	Flags  uint32 `yaml:"flags,omitempty" json:"flags,omitempty"`
	Status string `yaml:"status,omitempty" json:"status,omitempty"`
}

// ---------------------------------------------------------------------------
// STOMP
// ---------------------------------------------------------------------------

type STOMPConfig struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Port    int         `yaml:"port" json:"port"`
	Mocks   []STOMPMock `yaml:"mocks" json:"mocks"`
}

type STOMPMock struct {
	ID          string          `yaml:"id" json:"id"`
	Destination string          `yaml:"destination" json:"destination"`
	Response    *STOMPResponse  `yaml:"response,omitempty" json:"response,omitempty"`
	Delay       Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State       *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type STOMPResponse struct {
	Destination string            `yaml:"destination,omitempty" json:"destination,omitempty"`
	Body        string            `yaml:"body" json:"body"`
	ContentType string            `yaml:"content_type,omitempty" json:"content_type,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type ReceivedSTOMPMessage struct {
	ID          string            `json:"id"`
	Destination string            `json:"destination"`
	Body        string            `json:"body"`
	Headers     map[string]string `json:"headers"`
	Timestamp   string            `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// CoAP
// ---------------------------------------------------------------------------

type CoAPConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Port    int        `yaml:"port" json:"port"`
	Mocks   []CoAPMock `yaml:"mocks" json:"mocks"`
}

type CoAPMock struct {
	ID        string          `yaml:"id" json:"id"`
	Method    string          `yaml:"method" json:"method"`
	Path      string          `yaml:"path" json:"path"`
	PathRegex string          `yaml:"path_regex,omitempty" json:"path_regex,omitempty"`
	Response  CoAPResponse    `yaml:"response" json:"response"`
	Delay     Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State     *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type CoAPResponse struct {
	Code          string `yaml:"code" json:"code"`
	Payload       string `yaml:"payload,omitempty" json:"payload,omitempty"`
	ContentFormat int    `yaml:"content_format,omitempty" json:"content_format,omitempty"`
}

// ---------------------------------------------------------------------------
// SIP
// ---------------------------------------------------------------------------

type SIPConfig struct {
	Enabled bool      `yaml:"enabled" json:"enabled"`
	Port    int       `yaml:"port" json:"port"`
	Mocks   []SIPMock `yaml:"mocks" json:"mocks"`
}

type SIPMock struct {
	ID        string          `yaml:"id" json:"id"`
	Method    string          `yaml:"method" json:"method"`
	URI       string          `yaml:"uri,omitempty" json:"uri,omitempty"`
	URIRegex  string          `yaml:"uri_regex,omitempty" json:"uri_regex,omitempty"`
	Response  SIPResponse     `yaml:"response" json:"response"`
	Delay     Duration        `yaml:"delay,omitempty" json:"delay,omitempty"`
	State     *StateCondition `yaml:"state,omitempty" json:"state,omitempty"`
}

type SIPResponse struct {
	Status  int               `yaml:"status" json:"status"`
	Reason  string            `yaml:"reason,omitempty" json:"reason,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty" json:"body,omitempty"`
}

// ---------------------------------------------------------------------------
// Scenarios & Fault injection
// ---------------------------------------------------------------------------

// Scenario is a named set of mock patches that can be activated/deactivated
// at runtime via the management API. Dependency teams ship these alongside
// their preset configs to let consuming teams toggle error states, latency,
// and other failure modes deterministically.
type Scenario struct {
	ID          string          `yaml:"id" json:"id"`
	Name        string          `yaml:"name" json:"name"`
	Description string          `yaml:"description" json:"description"`
	Patches     []MockPatch     `yaml:"patches" json:"patches"`
	Faults      *ProtocolFaults `yaml:"faults,omitempty" json:"faults,omitempty"`
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

// ---------------------------------------------------------------------------
// Per-protocol fault types
// ---------------------------------------------------------------------------

type DNSFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Rcode     string   `yaml:"rcode,omitempty" json:"rcode,omitempty"` // NXDOMAIN|SERVFAIL|REFUSED|NOTIMP|FORMERR (default SERVFAIL)
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type GRPCFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Code      string   `yaml:"code,omitempty" json:"code,omitempty"` // gRPC status code: UNAVAILABLE|NOT_FOUND|DEADLINE_EXCEEDED|PERMISSION_DENIED|RESOURCE_EXHAUSTED|INTERNAL (default UNAVAILABLE)
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type HTTPFault struct {
	Delay     Duration          `yaml:"delay,omitempty" json:"delay,omitempty"`
	Status    int               `yaml:"status,omitempty" json:"status,omitempty"` // HTTP status code (default 503 when non-zero status/body is set)
	Body      string            `yaml:"body,omitempty" json:"body,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"` // Extra response headers (e.g. Retry-After)
	ErrorRate float64           `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
	// Abort closes the connection immediately with a TCP reset — no response is sent.
	Abort bool `yaml:"abort,omitempty" json:"abort,omitempty"`
	// TruncateBody sends only the first N bytes of the response body then abruptly
	// closes the connection, simulating a mid-transfer server crash.
	TruncateBody int `yaml:"truncate_body,omitempty" json:"truncate_body,omitempty"`
}

type WebSocketFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	CloseCode int      `yaml:"close_code,omitempty" json:"close_code,omitempty"` // WS close code (default 1011)
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type TCPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Response  string   `yaml:"response,omitempty" json:"response,omitempty"` // bytes to send before closing (default: just close)
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type RedisFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Error     string   `yaml:"error,omitempty" json:"error,omitempty"` // Redis error string, e.g. "LOADING" (default "ERR fault injected")
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type MQTTFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
	// When injected: response publish is silently dropped
}

type SMTPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Code      int      `yaml:"code,omitempty" json:"code,omitempty"` // SMTP error code: 421|450|550 (default 421)
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type SNMPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type AMQPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
	// When injected: delivery silently dropped
}

type KafkaFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	ErrorCode int16    `yaml:"error_code,omitempty" json:"error_code,omitempty"` // Kafka error code: 3=UNKNOWN_TOPIC|5=LEADER_NOT_AVAILABLE|7=REQUEST_TIMED_OUT (default 5)
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type LDAPFault struct {
	Delay      Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	ResultCode int      `yaml:"result_code,omitempty" json:"result_code,omitempty"` // LDAP result code: 32=NO_SUCH_OBJECT|49=INVALID_CREDENTIALS|50=INSUFFICIENT_ACCESS|52=UNAVAILABLE (default 52)
	Message    string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate  float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type IMAPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Response  string   `yaml:"response,omitempty" json:"response,omitempty"` // NO|BAD|BYE (default NO)
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type FTPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Code      int      `yaml:"code,omitempty" json:"code,omitempty"` // FTP error code: 421|530|550 (default 421)
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type MemcachedFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	ErrorType string   `yaml:"error_type,omitempty" json:"error_type,omitempty"` // SERVER_ERROR|CLIENT_ERROR (default SERVER_ERROR)
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type STOMPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type CoAPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Code      string   `yaml:"code,omitempty" json:"code,omitempty"` // CoAP code: "4.01"|"4.03"|"4.04"|"5.00"|"5.03" (default "5.00")
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

type SIPFault struct {
	Delay     Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Status    int      `yaml:"status,omitempty" json:"status,omitempty"` // SIP status: 404|408|486|503 (default 503)
	Reason    string   `yaml:"reason,omitempty" json:"reason,omitempty"`
	ErrorRate float64  `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
}

// ProtocolFaults holds optional fault configs for each protocol.
// Only non-nil fields are active.
type ProtocolFaults struct {
	HTTP      *HTTPFault      `yaml:"http,omitempty" json:"http,omitempty"`
	GraphQL   *HTTPFault      `yaml:"graphql,omitempty" json:"graphql,omitempty"`
	WebSocket *WebSocketFault `yaml:"websocket,omitempty" json:"websocket,omitempty"`
	GRPC      *GRPCFault      `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	TCP       *TCPFault       `yaml:"tcp,omitempty" json:"tcp,omitempty"`
	Redis     *RedisFault     `yaml:"redis,omitempty" json:"redis,omitempty"`
	MQTT      *MQTTFault      `yaml:"mqtt,omitempty" json:"mqtt,omitempty"`
	SMTP      *SMTPFault      `yaml:"smtp,omitempty" json:"smtp,omitempty"`
	SNMP      *SNMPFault      `yaml:"snmp,omitempty" json:"snmp,omitempty"`
	DNS       *DNSFault       `yaml:"dns,omitempty" json:"dns,omitempty"`
	AMQP      *AMQPFault      `yaml:"amqp,omitempty" json:"amqp,omitempty"`
	Kafka     *KafkaFault     `yaml:"kafka,omitempty" json:"kafka,omitempty"`
	LDAP      *LDAPFault      `yaml:"ldap,omitempty" json:"ldap,omitempty"`
	IMAP      *IMAPFault      `yaml:"imap,omitempty" json:"imap,omitempty"`
	FTP       *FTPFault       `yaml:"ftp,omitempty" json:"ftp,omitempty"`
	Memcached *MemcachedFault `yaml:"memcached,omitempty" json:"memcached,omitempty"`
	STOMP     *STOMPFault     `yaml:"stomp,omitempty" json:"stomp,omitempty"`
	CoAP      *CoAPFault      `yaml:"coap,omitempty" json:"coap,omitempty"`
	SIP       *SIPFault       `yaml:"sip,omitempty" json:"sip,omitempty"`
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
	return json.Marshal(d.String())
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
	return d.String(), nil
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

	data, err := os.ReadFile(path) // #nosec G304 -- path is user-supplied config file, intentional
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
	return os.WriteFile(path, data, 0o600)
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
	if cfg.Protocols.SNMP != nil {
		if cfg.Protocols.SNMP.Port == 0 {
			cfg.Protocols.SNMP.Port = 1161
		}
		if cfg.Protocols.SNMP.Community == "" {
			cfg.Protocols.SNMP.Community = "public"
		}
	}
	if cfg.Protocols.DNS != nil && cfg.Protocols.DNS.Port == 0 {
		cfg.Protocols.DNS.Port = 5353
	}
	if cfg.Protocols.AMQP != nil && cfg.Protocols.AMQP.Port == 0 {
		cfg.Protocols.AMQP.Port = 5672
	}
	if cfg.Protocols.Kafka != nil && cfg.Protocols.Kafka.Port == 0 {
		cfg.Protocols.Kafka.Port = 9092
	}
	if cfg.Protocols.LDAP != nil && cfg.Protocols.LDAP.Port == 0 {
		cfg.Protocols.LDAP.Port = 3893
	}
	if cfg.Protocols.IMAP != nil && cfg.Protocols.IMAP.Port == 0 {
		cfg.Protocols.IMAP.Port = 1143
	}
	if cfg.Protocols.FTP != nil {
		if cfg.Protocols.FTP.Port == 0 {
			cfg.Protocols.FTP.Port = 2121
		}
		if cfg.Protocols.FTP.PassivePortStart == 0 {
			cfg.Protocols.FTP.PassivePortStart = 50000
		}
	}
	if cfg.Protocols.Memcached != nil && cfg.Protocols.Memcached.Port == 0 {
		cfg.Protocols.Memcached.Port = 11211
	}
	if cfg.Protocols.STOMP != nil && cfg.Protocols.STOMP.Port == 0 {
		cfg.Protocols.STOMP.Port = 61613
	}
	if cfg.Protocols.CoAP != nil && cfg.Protocols.CoAP.Port == 0 {
		cfg.Protocols.CoAP.Port = 5683
	}
	if cfg.Protocols.SIP != nil && cfg.Protocols.SIP.Port == 0 {
		cfg.Protocols.SIP.Port = 5060
	}
}
