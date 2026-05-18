package coapserver_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/coapserver"
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

func buildCoAPGet(path string, msgID uint16, token byte) []byte {
	parts := []byte{0x41, 0x01, 0, 0, token}
	binary.BigEndian.PutUint16(parts[2:4], msgID)
	segment := path
	if len(segment) > 0 && segment[0] == '/' {
		segment = segment[1:]
	}
	parts = append(parts, byte((11<<4)|len(segment)))
	parts = append(parts, []byte(segment)...)
	return parts
}

func sendCoAP(t *testing.T, addr string) []byte {
	t.Helper()
	conn, err := net.DialTimeout("udp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial CoAP: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write(buildCoAPGet("/temp", 0x1234, 0x7a)); err != nil {
		t.Fatalf("write CoAP request: %v", err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read CoAP response: %v", err)
	}
	return append([]byte(nil), buf[:n]...)
}

func TestCoAPServer_GlobalFault(t *testing.T) {
	port := freeUDPPort(t)
	sc := scenarios.New(nil)
	srv := coapserver.New(&config.CoAPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.CoAPMock{{
			ID:       "m",
			Method:   "GET",
			Path:     "/temp",
			Response: config.CoAPResponse{Code: "2.05", Payload: "25C"},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{CoAP: &config.CoAPFault{ErrorRate: 0}})
	faultResp := sendCoAP(t, addr)
	if len(faultResp) < 2 || faultResp[1] != 0xA0 {
		t.Fatalf("fault response code byte = %#x, want 0xA0", faultResp[1])
	}

	sc.ClearDirectFaults()
	normalResp := sendCoAP(t, addr)
	if len(normalResp) < 2 || normalResp[1] != 0x45 {
		t.Fatalf("normal response code byte = %#x, want 0x45", normalResp[1])
	}
}

func TestCoAPServer_CoAPFault_CustomCode(t *testing.T) {
	port := freeUDPPort(t)
	sc := scenarios.New(nil)
	srv := coapserver.New(&config.CoAPConfig{Enabled: true, Port: port}, state.New(), sc, logger.New(100))
	startServer(t, srv)
	sc.SetDirectFaults(config.ProtocolFaults{CoAP: &config.CoAPFault{Code: "4.04", ErrorRate: 0}})
	resp := sendCoAP(t, fmt.Sprintf("127.0.0.1:%d", port))
	if len(resp) < 2 || resp[1] != 0x84 {
		t.Fatalf("fault response code byte = %#x, want 0x84", resp[1])
	}
}
