package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestLoad_DefaultsWhenMissing(t *testing.T) {
	cfg, err := config.Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mockly.UI.Port != 9090 {
		t.Errorf("want UI port 9090, got %d", cfg.Mockly.UI.Port)
	}
	if cfg.Mockly.API.Port != 9091 {
		t.Errorf("want API port 9091, got %d", cfg.Mockly.API.Port)
	}
}

func TestLoad_ParseYAML(t *testing.T) {
	yaml := `
mockly:
  ui:
    enabled: true
    port: 3000
  api:
    port: 3001
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: test-mock
        request:
          method: GET
          path: /hello
        response:
          status: 200
          body: '{"hello":"world"}'
          delay: 50ms
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Mockly.UI.Port != 3000 {
		t.Errorf("want 3000, got %d", cfg.Mockly.UI.Port)
	}
	if cfg.Protocols.HTTP == nil {
		t.Fatal("expected HTTP config")
	}
	if len(cfg.Protocols.HTTP.Mocks) != 1 {
		t.Fatalf("want 1 mock, got %d", len(cfg.Protocols.HTTP.Mocks))
	}
	m := cfg.Protocols.HTTP.Mocks[0]
	if m.ID != "test-mock" {
		t.Errorf("unexpected mock ID %q", m.ID)
	}
	if m.Response.Delay.Milliseconds() != 50 {
		t.Errorf("unexpected delay %v", m.Response.Delay)
	}
}

func TestLoad_NewProtocolDefaults(t *testing.T) {
	yaml := `
protocols:
  graphql:
    enabled: true
  tcp:
    enabled: true
  redis:
    enabled: true
  smtp:
    enabled: true
  mqtt:
    enabled: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Protocols.GraphQL == nil || cfg.Protocols.GraphQL.Port != 8082 {
		t.Errorf("want GraphQL port 8082, got %v", cfg.Protocols.GraphQL)
	}
	if cfg.Protocols.GraphQL.Path != "/graphql" {
		t.Errorf("want GraphQL path /graphql, got %q", cfg.Protocols.GraphQL.Path)
	}
	if cfg.Protocols.TCP == nil || cfg.Protocols.TCP.Port != 8083 {
		t.Errorf("want TCP port 8083, got %v", cfg.Protocols.TCP)
	}
	if cfg.Protocols.Redis == nil || cfg.Protocols.Redis.Port != 6379 {
		t.Errorf("want Redis port 6379, got %v", cfg.Protocols.Redis)
	}
	if cfg.Protocols.SMTP == nil || cfg.Protocols.SMTP.Port != 2525 {
		t.Errorf("want SMTP port 2525, got %v", cfg.Protocols.SMTP)
	}
	if cfg.Protocols.SMTP.Domain != "mockly.local" {
		t.Errorf("want SMTP domain mockly.local, got %q", cfg.Protocols.SMTP.Domain)
	}
	if cfg.Protocols.MQTT == nil || cfg.Protocols.MQTT.Port != 1883 {
		t.Errorf("want MQTT port 1883, got %v", cfg.Protocols.MQTT)
	}
}

func TestLoad_Scenarios(t *testing.T) {
	yaml := `
scenarios:
  - id: auth-down
    name: Auth Down
    description: Simulate auth service outage
    patches:
      - mock_id: token
        status: 503
        body: '{"error":"down"}'
      - mock_id: userinfo
        disabled: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Scenarios) != 1 {
		t.Fatalf("want 1 scenario, got %d", len(cfg.Scenarios))
	}
	sc := cfg.Scenarios[0]
	if sc.ID != "auth-down" {
		t.Errorf("unexpected scenario ID %q", sc.ID)
	}
	if len(sc.Patches) != 2 {
		t.Fatalf("want 2 patches, got %d", len(sc.Patches))
	}
	if sc.Patches[0].MockID != "token" || sc.Patches[0].Status != 503 {
		t.Errorf("unexpected first patch: %+v", sc.Patches[0])
	}
	if !sc.Patches[1].Disabled {
		t.Error("second patch should be disabled")
	}
}

func TestLoad_GraphQLMocks(t *testing.T) {
	yaml := `
protocols:
  graphql:
    enabled: true
    port: 8082
    mocks:
      - id: get-user
        operation_type: query
        operation_name: GetUser
        response:
          user:
            id: "123"
            name: Alice
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gql := cfg.Protocols.GraphQL
	if gql == nil {
		t.Fatal("expected GraphQL config")
	}
	if len(gql.Mocks) != 1 {
		t.Fatalf("want 1 mock, got %d", len(gql.Mocks))
	}
	m := gql.Mocks[0]
	if m.ID != "get-user" || m.OperationType != "query" || m.OperationName != "GetUser" {
		t.Errorf("unexpected mock: %+v", m)
	}
}

func TestLoad_IncludeMergesEveryProtocolAndSideCollections(t *testing.T) {
	dir := t.TempDir()
	// main.yaml configures every protocol with one mock/item each, plus the
	// "extra" side-collections (AMQP exchanges/bindings, LDAP/IMAP/FTP
	// users, SNMP traps) that aren't plain ID-keyed mock lists.
	main := `
protocols:
  http: { enabled: true, port: 8080, mocks: [{id: h1, request: {method: GET, path: /h1}, response: {status: 200}}] }
  websocket: { enabled: true, port: 8081, mocks: [{id: w1, path: /w1}] }
  grpc: { enabled: true, port: 50051, services: [{mocks: [{id: g1, method: G1, response: {}}]}] }
  graphql: { enabled: true, port: 8082, mocks: [{id: q1, operation_type: query, operation_name: Q1, response: {}}] }
  tcp: { enabled: true, port: 8083, mocks: [{id: t1, match: "T1", response: "r1"}] }
  redis: { enabled: true, port: 6379, mocks: [{id: r1, command: GET, key: "k1", response: {type: bulk, value: v1}}] }
  smtp: { enabled: true, port: 2525, rules: [{id: s1, action: accept}] }
  mqtt: { enabled: true, port: 1883, mocks: [{id: m1, topic: "t/1"}] }
  nats: { enabled: true, port: 4222, mocks: [{id: n1, subject: "s.1"}] }
  snmp: { enabled: true, port: 1161, mocks: [{id: sn1, oid: "1.1"}], traps: [{id: tr1, target: "127.0.0.1:1", oid: "1.1"}] }
  dns: { enabled: true, port: 5353, mocks: [{id: d1, name: "d1.example.", type: A, value: "1.2.3.4"}] }
  amqp: { enabled: true, port: 5672, mocks: [{id: a1, exchange: ex1, routing_key: rk1}], exchanges: [{name: ex1, type: direct}], bindings: [{exchange: ex1, queue: q1}] }
  kafka: { enabled: true, port: 9092, mocks: [{id: k1, topic: "topic1"}] }
  ldap: { enabled: true, port: 1389, mocks: [{id: l1, base_dn: "dc=a", attributes: {cn: [a1]}}], users: [{username: u1, password: p1}] }
  imap: { enabled: true, port: 1143, mailboxes: [{id: i1, name: INBOX, messages: []}], users: [{username: iu1, password: p1}] }
  ftp: { enabled: true, port: 2121, files: [{id: f1, path: "/f1", content: c1}], users: [{username: fu1, password: p1}] }
  memcached: { enabled: true, port: 11211, mocks: [{id: mc1, key: "mc1"}] }
  stomp: { enabled: true, port: 61613, mocks: [{id: st1, destination: "/queue/1"}] }
  coap: { enabled: true, port: 5683, mocks: [{id: c1, path: /c1}] }
  sip: { enabled: true, port: 5060, mocks: [{id: sip1, method: INVITE}] }
include:
  - extra.yaml
`
	// extra.yaml appends a second item to every protocol's mocks/items, and
	// a second entry to every side-collection.
	extra := `
protocols:
  http: { mocks: [{id: h2, request: {method: GET, path: /h2}, response: {status: 200}}] }
  websocket: { mocks: [{id: w2, path: /w2}] }
  grpc: { services: [{mocks: [{id: g2, method: G2, response: {}}]}] }
  graphql: { mocks: [{id: q2, operation_type: query, operation_name: Q2, response: {}}] }
  tcp: { mocks: [{id: t2, match: "T2", response: "r2"}] }
  redis: { mocks: [{id: r2, command: GET, key: "k2", response: {type: bulk, value: v2}}] }
  smtp: { rules: [{id: s2, action: accept}] }
  mqtt: { mocks: [{id: m2, topic: "t/2"}] }
  nats: { mocks: [{id: n2, subject: "s.2"}] }
  snmp: { mocks: [{id: sn2, oid: "1.2"}], traps: [{id: tr2, target: "127.0.0.1:2", oid: "1.2"}] }
  dns: { mocks: [{id: d2, name: "d2.example.", type: A, value: "1.2.3.5"}] }
  amqp: { mocks: [{id: a2, exchange: ex2, routing_key: rk2}], exchanges: [{name: ex2, type: fanout}], bindings: [{exchange: ex2, queue: q2}] }
  kafka: { mocks: [{id: k2, topic: "topic2"}] }
  ldap: { mocks: [{id: l2, base_dn: "dc=b", attributes: {cn: [a2]}}], users: [{username: u2, password: p2}] }
  imap: { mailboxes: [{id: i2, name: SENT, messages: []}], users: [{username: iu2, password: p2}] }
  ftp: { files: [{id: f2, path: "/f2", content: c2}], users: [{username: fu2, password: p2}] }
  memcached: { mocks: [{id: mc2, key: "mc2"}] }
  stomp: { mocks: [{id: st2, destination: "/queue/2"}] }
  coap: { mocks: [{id: c2, path: /c2}] }
  sip: { mocks: [{id: sip2, method: BYE}] }
`
	writeFile(t, dir, "mockly.yaml", main)
	writeFile(t, dir, "extra.yaml", extra)

	cfg, err := config.Load(filepath.Join(dir, "mockly.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	checks := []struct {
		name string
		got  int
		want int
	}{
		{"http.mocks", len(cfg.Protocols.HTTP.Mocks), 2},
		{"websocket.mocks", len(cfg.Protocols.WebSocket.Mocks), 2},
		{"grpc.services", len(cfg.Protocols.GRPC.Services), 2},
		{"graphql.mocks", len(cfg.Protocols.GraphQL.Mocks), 2},
		{"tcp.mocks", len(cfg.Protocols.TCP.Mocks), 2},
		{"redis.mocks", len(cfg.Protocols.Redis.Mocks), 2},
		{"smtp.rules", len(cfg.Protocols.SMTP.Rules), 2},
		{"mqtt.mocks", len(cfg.Protocols.MQTT.Mocks), 2},
		{"nats.mocks", len(cfg.Protocols.NATS.Mocks), 2},
		{"snmp.mocks", len(cfg.Protocols.SNMP.Mocks), 2},
		{"snmp.traps", len(cfg.Protocols.SNMP.Traps), 2},
		{"dns.mocks", len(cfg.Protocols.DNS.Mocks), 2},
		{"amqp.mocks", len(cfg.Protocols.AMQP.Mocks), 2},
		{"amqp.exchanges", len(cfg.Protocols.AMQP.Exchanges), 2},
		{"amqp.bindings", len(cfg.Protocols.AMQP.Bindings), 2},
		{"kafka.mocks", len(cfg.Protocols.Kafka.Mocks), 2},
		{"ldap.mocks", len(cfg.Protocols.LDAP.Mocks), 2},
		{"ldap.users", len(cfg.Protocols.LDAP.Users), 2},
		{"imap.mailboxes", len(cfg.Protocols.IMAP.Mailboxes), 2},
		{"imap.users", len(cfg.Protocols.IMAP.Users), 2},
		{"ftp.files", len(cfg.Protocols.FTP.Files), 2},
		{"ftp.users", len(cfg.Protocols.FTP.Users), 2},
		{"memcached.mocks", len(cfg.Protocols.Memcached.Mocks), 2},
		{"stomp.mocks", len(cfg.Protocols.STOMP.Mocks), 2},
		{"coap.mocks", len(cfg.Protocols.CoAP.Mocks), 2},
		{"sip.mocks", len(cfg.Protocols.SIP.Mocks), 2},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestLoad_RedisMocks(t *testing.T) {
	yaml := `
protocols:
  redis:
    enabled: true
    mocks:
      - id: get-session
        command: GET
        key: "session:*"
        response:
          type: bulk
          value: '{"userId":"abc"}'
          delay: 10ms
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mockly.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := cfg.Protocols.Redis
	if r == nil || len(r.Mocks) != 1 {
		t.Fatal("expected 1 redis mock")
	}
	m := r.Mocks[0]
	if m.Command != "GET" || m.Key != "session:*" {
		t.Errorf("unexpected mock: %+v", m)
	}
	if m.Response.Type != "bulk" {
		t.Errorf("want type bulk, got %q", m.Response.Type)
	}
}

// ---------------------------------------------------------------------------
// Multi-file config: Include field + directory mode
// ---------------------------------------------------------------------------

func TestLoad_IncludeMergesMocksAndAdoptsNewProtocol(t *testing.T) {
	dir := t.TempDir()
	main := `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: health
        request: { method: GET, path: /health }
        response: { status: 200, body: "ok" }
include:
  - orders.yaml
  - users.yaml
`
	orders := `
protocols:
  http:
    mocks:
      - id: get-order
        request: { method: GET, path: /orders/1 }
        response: { status: 200, body: "order" }
`
	// users.yaml introduces a protocol the main file never mentions (LDAP);
	// its whole section (including port/enabled) must be adopted as-is.
	users := `
protocols:
  ldap:
    enabled: true
    port: 1389
    mocks:
      - id: find-user
        base_dn: "dc=example,dc=com"
        attributes: { cn: ["alice"] }
`
	writeFile(t, dir, "mockly.yaml", main)
	writeFile(t, dir, "orders.yaml", orders)
	writeFile(t, dir, "users.yaml", users)

	cfg, err := config.Load(filepath.Join(dir, "mockly.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Protocols.HTTP == nil || len(cfg.Protocols.HTTP.Mocks) != 2 {
		t.Fatalf("HTTP mocks = %+v, want 2 (health from main, get-order from include)", cfg.Protocols.HTTP)
	}
	if cfg.Protocols.HTTP.Port != 8080 {
		t.Errorf("HTTP port = %d, want 8080 (from main file, not overwritten by include)", cfg.Protocols.HTTP.Port)
	}

	if cfg.Protocols.LDAP == nil {
		t.Fatal("expected LDAP section adopted wholesale from users.yaml")
	}
	if !cfg.Protocols.LDAP.Enabled || cfg.Protocols.LDAP.Port != 1389 {
		t.Errorf("LDAP section = %+v, want enabled on port 1389 (adopted from include)", cfg.Protocols.LDAP)
	}
	if len(cfg.Protocols.LDAP.Mocks) != 1 || cfg.Protocols.LDAP.Mocks[0].ID != "find-user" {
		t.Errorf("LDAP mocks = %+v, want [find-user]", cfg.Protocols.LDAP.Mocks)
	}
}

func TestLoad_IncludeOverridesByID(t *testing.T) {
	dir := t.TempDir()
	main := `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: health
        request: { method: GET, path: /health }
        response: { status: 200, body: "old" }
include:
  - override.yaml
`
	override := `
protocols:
  http:
    mocks:
      - id: health
        request: { method: GET, path: /health }
        response: { status: 200, body: "new" }
`
	writeFile(t, dir, "mockly.yaml", main)
	writeFile(t, dir, "override.yaml", override)

	cfg, err := config.Load(filepath.Join(dir, "mockly.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Protocols.HTTP.Mocks) != 1 {
		t.Fatalf("HTTP mocks = %+v, want exactly 1 (same ID overrides, not appended)", cfg.Protocols.HTTP.Mocks)
	}
	if cfg.Protocols.HTTP.Mocks[0].Response.Body != "new" {
		t.Errorf("response.body = %q, want %q (include with matching ID should override)", cfg.Protocols.HTTP.Mocks[0].Response.Body, "new")
	}
}

func TestLoad_IncludeGlobNoMatchIsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "mockly.yaml", "include:\n  - does-not-exist-*.yaml\n")

	_, err := config.Load(filepath.Join(dir, "mockly.yaml"))
	if err == nil {
		t.Fatal("expected an error for an include pattern matching no files")
	}
}

func TestLoad_DirectoryModeAutoIncludesSiblingFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "mockly.yaml", `
protocols:
  http:
    enabled: true
    port: 8080
    mocks:
      - id: health
        request: { method: GET, path: /health }
        response: { status: 200, body: "ok" }
`)
	writeFile(t, dir, "orders.yaml", `
protocols:
  http:
    mocks:
      - id: get-order
        request: { method: GET, path: /orders/1 }
        response: { status: 200, body: "order" }
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load(dir): %v", err)
	}
	if cfg.Protocols.HTTP == nil || len(cfg.Protocols.HTTP.Mocks) != 2 {
		t.Fatalf("HTTP mocks = %+v, want 2 (auto-included orders.yaml alongside mockly.yaml)", cfg.Protocols.HTTP)
	}
}

func TestLoad_DirectoryModeRequiresMocklyYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "orders.yaml", "protocols:\n  http:\n    enabled: true\n    mocks: []\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected an error: directory has no mockly.yaml")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Duration — marshal / unmarshal
// ---------------------------------------------------------------------------

func TestDuration_UnmarshalText_Valid(t *testing.T) {
	var d config.Duration
	if err := d.UnmarshalText([]byte("500ms")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Milliseconds() != 500 {
		t.Errorf("want 500ms, got %v", d.Duration)
	}
}

func TestDuration_UnmarshalText_Invalid(t *testing.T) {
	var d config.Duration
	if err := d.UnmarshalText([]byte("not-a-duration")); err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}

func TestDuration_MarshalJSON(t *testing.T) {
	d := config.Duration{}
	d.Duration = 2 * 1e9 // 2s
	b, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	// Should be a JSON-quoted string like "2s".
	if string(b) != `"2s"` {
		t.Errorf("want \"2s\", got %s", b)
	}
}

func TestDuration_UnmarshalJSON_Valid(t *testing.T) {
	var d config.Duration
	if err := d.UnmarshalJSON([]byte(`"1m30s"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Seconds() != 90 {
		t.Errorf("want 90s, got %v", d.Duration)
	}
}

func TestDuration_UnmarshalJSON_EmptyString(t *testing.T) {
	var d config.Duration
	d.Duration = 5 * 1e9 // pre-set to non-zero
	if err := d.UnmarshalJSON([]byte(`""`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Duration != 0 {
		t.Errorf("empty string should set duration to 0, got %v", d.Duration)
	}
}

func TestDuration_UnmarshalJSON_ZeroS(t *testing.T) {
	var d config.Duration
	d.Duration = 5 * 1e9
	if err := d.UnmarshalJSON([]byte(`"0s"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Duration != 0 {
		t.Errorf("\"0s\" should set duration to 0, got %v", d.Duration)
	}
}

func TestDuration_UnmarshalJSON_Invalid(t *testing.T) {
	var d config.Duration
	if err := d.UnmarshalJSON([]byte(`"not-valid"`)); err == nil {
		t.Fatal("expected error for invalid duration JSON, got nil")
	}
}

func TestDuration_MarshalYAML(t *testing.T) {
	d := config.Duration{}
	d.Duration = 100 * 1e6 // 100ms
	v, err := d.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	if v != "100ms" {
		t.Errorf("want \"100ms\", got %v", v)
	}
}

// ---------------------------------------------------------------------------
// Save
// ---------------------------------------------------------------------------

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.yaml")

	original, err := config.Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Load defaults: %v", err)
	}
	original.Mockly.UI.Port = 7777

	if err := config.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load saved: %v", err)
	}
	if loaded.Mockly.UI.Port != 7777 {
		t.Errorf("round-trip: want port 7777, got %d", loaded.Mockly.UI.Port)
	}
}

