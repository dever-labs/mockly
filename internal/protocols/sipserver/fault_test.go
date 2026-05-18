package sipserver_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/sipserver"
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

func sipInvite() string {
	return "INVITE sip:alice@example.com SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP 127.0.0.1:5060\r\n" +
		"From: <sip:bob@example.com>\r\n" +
		"To: <sip:alice@example.com>\r\n" +
		"Call-ID: 1234@example.com\r\n" +
		"CSeq: 1 INVITE\r\n" +
		"Content-Length: 0\r\n\r\n"
}

func sendSIP(t *testing.T, addr string) string {
	t.Helper()
	conn, err := net.DialTimeout("udp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial SIP: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(sipInvite())); err != nil {
		t.Fatalf("write SIP INVITE: %v", err)
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read SIP response: %v", err)
	}
	return string(buf[:n])
}

func TestSIPServer_GlobalFault(t *testing.T) {
	port := freeUDPPort(t)
	sc := scenarios.New(nil)
	srv := sipserver.New(&config.SIPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.SIPMock{{
			ID:       "m",
			Method:   "INVITE",
			URI:      "sip:*",
			Response: config.SIPResponse{Status: 200, Reason: "OK"},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{SIP: &config.SIPFault{ErrorRate: 0}})
	faultResp := sendSIP(t, addr)
	if !strings.HasPrefix(faultResp, "SIP/2.0 503") {
		t.Fatalf("fault response = %q, want SIP 503", faultResp)
	}

	sc.ClearDirectFaults()
	normalResp := sendSIP(t, addr)
	if !strings.HasPrefix(normalResp, "SIP/2.0 200") {
		t.Fatalf("normal response = %q, want SIP 200", normalResp)
	}
}

func TestSIPServer_SIPFault_CustomStatus(t *testing.T) {
	port := freeUDPPort(t)
	sc := scenarios.New(nil)
	srv := sipserver.New(&config.SIPConfig{Enabled: true, Port: port}, state.New(), sc, logger.New(100))
	startServer(t, srv)
	sc.SetDirectFaults(config.ProtocolFaults{SIP: &config.SIPFault{Status: 404, ErrorRate: 0}})
	resp := sendSIP(t, fmt.Sprintf("127.0.0.1:%d", port))
	if !strings.HasPrefix(resp, "SIP/2.0 404") {
		t.Fatalf("fault response = %q", resp)
	}
}
