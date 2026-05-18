package snmpserver_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/snmpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func startServer(t *testing.T, srv interface{ Start(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(150 * time.Millisecond)
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.LocalAddr().(*net.UDPAddr).Port
	_ = ln.Close()
	time.Sleep(10 * time.Millisecond)
	return port
}

func newSNMPClient(port int) *gosnmp.GoSNMP {
	return &gosnmp.GoSNMP{
		Target:    "127.0.0.1",
		Port:      uint16(port),
		Community: "public",
		Version:   gosnmp.Version2c,
		Timeout:   time.Second,
		Retries:   1,
	}
}

func TestSNMPServer_GlobalFault(t *testing.T) {
	port := freeUDPPort(t)
	sc := scenarios.New(nil)
	srv := snmpserver.New(&config.SNMPConfig{
		Enabled:   true,
		Port:      port,
		Community: "public",
		Mocks: []config.SNMPMock{{
			ID:    "m",
			OID:   "1.3.6.1.2.1.1.1.0",
			Type:  "string",
			Value: "test",
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	client := newSNMPClient(port)
	if err := client.Connect(); err != nil {
		t.Fatalf("connect SNMP: %v", err)
	}
	defer client.Conn.Close() //nolint:errcheck

	sc.SetDirectFaults(config.ProtocolFaults{SNMP: &config.SNMPFault{ErrorRate: 0}})
	pkt, err := client.Get([]string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("fault GET transport error: %v", err)
	}
	if len(pkt.Variables) == 0 {
		t.Fatal("fault GET returned no variables")
	}
	faultValue, ok := pkt.Variables[0].Value.([]byte)
	if !ok {
		t.Fatalf("fault variable type = %T, want []byte", pkt.Variables[0].Value)
	}
	if got := string(faultValue); !strings.Contains(got, "fault injected") {
		t.Fatalf("fault value = %q, want fault marker", got)
	}

	sc.ClearDirectFaults()
	pkt, err = client.Get([]string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("normal GET: %v", err)
	}
	if len(pkt.Variables) == 0 {
		t.Fatal("normal GET returned no variables")
	}
	var got string
	switch v := pkt.Variables[0].Value.(type) {
	case []byte:
		got = string(v)
	case string:
		got = v
	default:
		got = fmt.Sprint(v)
	}
	if got != "test" {
		t.Fatalf("normal value = %q, want %q", got, "test")
	}
}

func TestSNMPServer_SNMPFault_CustomMessage(t *testing.T) {
	port := freeUDPPort(t)
	sc := scenarios.New(nil)
	srv := snmpserver.New(&config.SNMPConfig{Enabled: true, Port: port, Community: "public", Mocks: []config.SNMPMock{{ID: "m", OID: "1.3.6.1.2.1.1.1.0", Type: "string", Value: "test"}}}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	client := newSNMPClient(port)
	if err := client.Connect(); err != nil {
		t.Fatalf("connect SNMP: %v", err)
	}
	defer client.Conn.Close() //nolint:errcheck
	sc.SetDirectFaults(config.ProtocolFaults{SNMP: &config.SNMPFault{Message: "boom", ErrorRate: 0}})
	pkt, err := client.Get([]string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	faultValue, ok := pkt.Variables[0].Value.([]byte)
	if !ok {
		t.Fatalf("fault variable type = %T", pkt.Variables[0].Value)
	}
	if got := string(faultValue); !strings.Contains(got, "boom") {
		t.Fatalf("fault value = %q", got)
	}
}
