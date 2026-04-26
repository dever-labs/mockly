// Integration tests for the SNMP server — sends real SNMP requests using gosnmp.
package snmpserver

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

// freeUDPPort returns a free UDP port for the test server.
func freeUDPPort() int {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	_ = conn.Close()
	return port
}

// startSNMP starts the server and returns a cancel func.
func startSNMP(t *testing.T, srv *Server) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Start(ctx) //nolint:errcheck

	// Wait until the UDP port is accepting packets.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("udp", fmt.Sprintf("127.0.0.1:%d", srv.cfg.Port), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Give the server a moment to be fully ready.
	time.Sleep(100 * time.Millisecond)
	return cancel
}

// snmpClient returns a gosnmp client pointed at the test server.
func snmpClient(port int) *gosnmp.GoSNMP {
	return &gosnmp.GoSNMP{
		Target:    "127.0.0.1",
		Port:      uint16(port),
		Community: "public",
		Version:   gosnmp.Version2c,
		Timeout:   2 * time.Second,
		Retries:   1,
	}
}

// defaultMocks returns a set of mocks covering multiple OID types.
func defaultMocks() []config.SNMPMock {
	return []config.SNMPMock{
		{ID: "sys-descr", OID: "1.3.6.1.2.1.1.1.0", Type: "string", Value: "Mockly Test Device"},
		{ID: "sys-uptime", OID: "1.3.6.1.2.1.1.3.0", Type: "timeticks", Value: 12345},
		{ID: "if-number", OID: "1.3.6.1.2.1.2.1.0", Type: "integer", Value: 4},
		{ID: "in-octets", OID: "1.3.6.1.2.1.2.2.1.10.1", Type: "counter32", Value: 1024},
		{ID: "mgmt-ip", OID: "1.3.6.1.4.1.9999.1.0", Type: "ipaddress", Value: "10.0.0.1"},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSNMPServer_GET_String(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	result, err := g.Get([]string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if len(result.Variables) == 0 {
		t.Fatal("no variables returned")
	}
	// gosnmp returns OctetString values as []byte.
	raw := result.Variables[0].Value
	var got string
	switch v := raw.(type) {
	case []byte:
		got = string(v)
	case string:
		got = v
	default:
		got = fmt.Sprint(v)
	}
	if got != "Mockly Test Device" {
		t.Errorf("sys-descr = %q, want %q", got, "Mockly Test Device")
	}
}

func TestSNMPServer_GET_Integer(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	result, err := g.Get([]string{"1.3.6.1.2.1.2.1.0"})
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if len(result.Variables) == 0 {
		t.Fatal("no variables returned")
	}
	got := gosnmp.ToBigInt(result.Variables[0].Value).Int64()
	if got != 4 {
		t.Errorf("if-number = %d, want 4", got)
	}
}

func TestSNMPServer_GET_TimeTicks(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	result, err := g.Get([]string{"1.3.6.1.2.1.1.3.0"})
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if len(result.Variables) == 0 {
		t.Fatal("no variables returned")
	}
	if result.Variables[0].Type != gosnmp.TimeTicks {
		t.Errorf("type = %v, want TimeTicks", result.Variables[0].Type)
	}
}

func TestSNMPServer_GET_Counter32(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	result, err := g.Get([]string{"1.3.6.1.2.1.2.2.1.10.1"})
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if len(result.Variables) == 0 {
		t.Fatal("no variables returned")
	}
	if result.Variables[0].Type != gosnmp.Counter32 {
		t.Errorf("type = %v, want Counter32", result.Variables[0].Type)
	}
}

func TestSNMPServer_GET_MultipleOIDs(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	oids := []string{"1.3.6.1.2.1.1.1.0", "1.3.6.1.2.1.2.1.0"}
	result, err := g.Get(oids)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if len(result.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(result.Variables))
	}
}

func TestSNMPServer_GETNEXT(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	// GetNext from sys-descr should return sys-uptime (lexicographically next).
	result, err := g.GetNext([]string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("GETNEXT: %v", err)
	}
	if len(result.Variables) == 0 {
		t.Fatal("no variables returned")
	}
	// The next OID should be different from the one we queried.
	if result.Variables[0].Name == "1.3.6.1.2.1.1.1.0" {
		t.Errorf("GETNEXT returned same OID")
	}
}

func TestSNMPServer_GETBULK(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	// BulkWalkAll should return all OIDs under 1.3.6.1.2.1.1.
	results, err := g.BulkWalkAll("1.3.6.1.2.1.1")
	if err != nil {
		t.Fatalf("GETBULK walk: %v", err)
	}
	if len(results) == 0 {
		t.Error("GETBULK returned no results")
	}
}

func TestSNMPServer_SET_UpdatesValue(t *testing.T) {
	port := freeUDPPort()
	mocks := []config.SNMPMock{
		{ID: "writable", OID: "1.3.6.1.4.1.9999.2.0", Type: "string", Value: "original"},
	}
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: mocks}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	// SET the value.
	pdus := []gosnmp.SnmpPDU{{
		Name:  "1.3.6.1.4.1.9999.2.0",
		Type:  gosnmp.OctetString,
		Value: "updated",
	}}
	if _, err := g.Set(pdus); err != nil {
		t.Fatalf("SET: %v", err)
	}

	// Verify the new value is returned on GET.
	result, err := g.Get([]string{"1.3.6.1.4.1.9999.2.0"})
	if err != nil {
		t.Fatalf("GET after SET: %v", err)
	}
	if len(result.Variables) == 0 {
		t.Fatal("no variables returned")
	}
	// gosnmp returns OctetString values as []byte.
	var got string
	switch v := result.Variables[0].Value.(type) {
	case []byte:
		got = string(v)
	case string:
		got = v
	default:
		got = fmt.Sprint(v)
	}
	if got != "updated" {
		t.Errorf("after SET got %q, want %q", got, "updated")
	}
}

func TestSNMPServer_SetMocks_DynamicUpdate(t *testing.T) {
	port := freeUDPPort()
	initial := []config.SNMPMock{
		{ID: "a", OID: "1.3.6.1.4.1.9999.3.0", Type: "integer", Value: 1},
	}
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: initial}
	srv := New(cfg, state.New(), logger.New(10))
	stop := startSNMP(t, srv)
	defer stop()

	newMocks := []config.SNMPMock{
		{ID: "b", OID: "1.3.6.1.4.1.9999.4.0", Type: "integer", Value: 99},
	}
	srv.SetMocks(newMocks)

	// Verify the new mocks are reflected in GetMocks().
	got := srv.GetMocks()
	if len(got) != 1 || got[0].ID != "b" {
		t.Errorf("after SetMocks: got %v, want [b]", got)
	}

	// Give the server time to restart with new OID list.
	time.Sleep(300 * time.Millisecond)

	g := snmpClient(port)
	if err := g.Connect(); err != nil {
		t.Fatalf("connect after restart: %v", err)
	}
	defer g.Conn.Close() //nolint:errcheck

	result, err := g.Get([]string{"1.3.6.1.4.1.9999.4.0"})
	if err != nil {
		t.Fatalf("GET new OID: %v", err)
	}
	if len(result.Variables) == 0 {
		t.Fatal("no variables returned")
	}
	got2 := gosnmp.ToBigInt(result.Variables[0].Value).Int64()
	if got2 != 99 {
		t.Errorf("new OID value = %d, want 99", got2)
	}
}

func TestSNMPServer_GetMocks_Roundtrip(t *testing.T) {
	port := freeUDPPort()
	mocks := defaultMocks()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: mocks}
	srv := New(cfg, state.New(), logger.New(10))

	got := srv.GetMocks()
	if len(got) != len(mocks) {
		t.Errorf("GetMocks() len = %d, want %d", len(got), len(mocks))
	}
}

func TestSNMPServer_StatusInfo(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: defaultMocks()}
	srv := New(cfg, state.New(), logger.New(10))

	info := srv.StatusInfo()
	if info["protocol"] != "snmp" {
		t.Errorf("protocol = %v, want snmp", info["protocol"])
	}
	if info["enabled"] != true {
		t.Errorf("enabled = %v, want true", info["enabled"])
	}
	if info["mocks"].(int) != len(defaultMocks()) {
		t.Errorf("mocks = %v, want %d", info["mocks"], len(defaultMocks()))
	}
}

func TestSNMPServer_GetTraps_Roundtrip(t *testing.T) {
	port := freeUDPPort()
	traps := []config.SNMPTrap{
		{ID: "t1", Target: "127.0.0.1:1162", Version: "2c", Community: "public", OID: "1.3.6.1.6.3.1.1.5.1"},
	}
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public", Traps: traps}
	srv := New(cfg, state.New(), logger.New(10))

	got := srv.GetTraps()
	if len(got) != 1 {
		t.Errorf("GetTraps() len = %d, want 1", len(got))
	}
	if got[0].ID != "t1" {
		t.Errorf("trap ID = %q, want t1", got[0].ID)
	}
}

func TestSNMPServer_SendTrap_NotFound(t *testing.T) {
	port := freeUDPPort()
	cfg := &config.SNMPConfig{Enabled: true, Port: port, Community: "public"}
	srv := New(cfg, state.New(), logger.New(10))

	err := srv.SendTrap("nonexistent")
	if err == nil {
		t.Error("expected error for unknown trap ID")
	}
}
