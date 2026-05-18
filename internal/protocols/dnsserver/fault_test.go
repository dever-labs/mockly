package dnsserver_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/dnsserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func startServer(t *testing.T, srv interface{ Start(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(100 * time.Millisecond)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	time.Sleep(10 * time.Millisecond)
	return port
}

func queryDNS(t *testing.T, addr string) *dns.Msg {
	t.Helper()
	c := &dns.Client{Net: "udp", Timeout: time.Second}
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	r, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("dns exchange: %v", err)
	}
	return r
}

func TestDNSServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := dnsserver.New(&config.DNSConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.DNSMock{{
			ID:      "m",
			Name:    "example.com",
			Type:    "A",
			Records: []string{"1.2.3.4"},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{DNS: &config.DNSFault{ErrorRate: 0}})

	faultResp := queryDNS(t, addr)
	if faultResp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("fault rcode = %d, want %d", faultResp.Rcode, dns.RcodeServerFailure)
	}

	sc.ClearDirectFaults()
	normalResp := queryDNS(t, addr)
	if normalResp.Rcode != dns.RcodeSuccess {
		t.Fatalf("normal rcode = %d, want %d", normalResp.Rcode, dns.RcodeSuccess)
	}
	if len(normalResp.Answer) == 0 {
		t.Fatal("normal response missing answers")
	}
}

func TestDNSServer_DNSFault_NXDOMAIN(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := dnsserver.New(&config.DNSConfig{
		Enabled: true,
		Port:    port,
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	sc.SetDirectFaults(config.ProtocolFaults{DNS: &config.DNSFault{Rcode: "NXDOMAIN", ErrorRate: 0}})
	resp := queryDNS(t, fmt.Sprintf("127.0.0.1:%d", port))
	if resp.Rcode != dns.RcodeNameError {
		t.Fatalf("fault rcode = %d, want %d", resp.Rcode, dns.RcodeNameError)
	}
}
