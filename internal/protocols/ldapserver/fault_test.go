package ldapserver_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/ldapserver"
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

func berLen(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n < 0x100 {
		return []byte{0x81, byte(n)}
	}
	return []byte{0x82, byte(n >> 8), byte(n)}
}

func berTLV(tag byte, content []byte) []byte {
	out := []byte{tag}
	out = append(out, berLen(len(content))...)
	out = append(out, content...)
	return out
}

func buildLDAPMessage(msgID int, tag byte, content []byte) []byte {
	body := append(berTLV(0x02, []byte{byte(msgID)}), berTLV(tag, content)...)
	return berTLV(0x30, body)
}

func buildBindRequest(msgID int) []byte {
	content := append(berTLV(0x02, []byte{0x03}), berTLV(0x04, nil)...)
	content = append(content, 0x80, 0x00)
	return buildLDAPMessage(msgID, 0x60, content)
}

func buildSearchRequest(msgID int) []byte {
	content := berTLV(0x04, []byte("dc=example,dc=com"))
	content = append(content, berTLV(0x0a, []byte{0x02})...)
	content = append(content, berTLV(0x0a, []byte{0x00})...)
	content = append(content, berTLV(0x02, []byte{0x00})...)
	content = append(content, berTLV(0x02, []byte{0x00})...)
	content = append(content, berTLV(0x01, []byte{0x00})...)
	content = append(content, berTLV(0x87, []byte("objectClass"))...)
	content = append(content, berTLV(0x30, nil)...)
	return buildLDAPMessage(msgID, 0x63, content)
}

func readBERPacket(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	head := make([]byte, 2)
	if _, err := io.ReadFull(conn, head); err != nil {
		t.Fatalf("read LDAP header: %v", err)
	}
	length := int(head[1])
	if head[1]&0x80 != 0 {
		extra := int(head[1] & 0x7f)
		buf := make([]byte, extra)
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("read LDAP length bytes: %v", err)
		}
		length = 0
		for _, b := range buf {
			length = (length << 8) | int(b)
		}
		head = append(head, buf...)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatalf("read LDAP packet body: %v", err)
	}
	return append(head, body...)
}

func readTLV(b []byte) (byte, []byte, int) {
	if len(b) < 2 {
		return 0, nil, len(b)
	}
	length := int(b[1])
	hdr := 2
	if b[1]&0x80 != 0 {
		n := int(b[1] & 0x7f)
		length = 0
		for i := 0; i < n; i++ {
			length = (length << 8) | int(b[2+i])
		}
		hdr = 2 + n
	}
	return b[0], b[hdr : hdr+length], hdr + length
}

func ldapResultCode(packet []byte) (byte, byte) {
	_, content, _ := readTLV(packet)
	_, _, next := readTLV(content)
	opTag, opContent, _ := readTLV(content[next:])
	_, codeContent, _ := readTLV(opContent)
	return opTag, codeContent[0]
}

func TestLDAPServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := ldapserver.New(&config.LDAPConfig{
		Enabled: true,
		Port:    port,
		Mocks: []config.LDAPMock{{
			ID:         "m",
			BaseDN:     "dc=example,dc=com",
			Attributes: map[string][]string{"cn": {"testuser"}},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial LDAP: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(buildBindRequest(1)); err != nil {
		t.Fatalf("write bind request: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x61 || code != 0x00 {
		t.Fatalf("bind response tag/code = %#x/%#x, want 0x61/0x00", tag, code)
	}

	sc.SetDirectFaults(config.ProtocolFaults{LDAP: &config.LDAPFault{ErrorRate: 0}})
	if _, err := conn.Write(buildSearchRequest(2)); err != nil {
		t.Fatalf("write faulted search request: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x65 || code != 0x34 {
		t.Fatalf("fault search response tag/code = %#x/%#x, want 0x65/0x34", tag, code)
	}

	sc.ClearDirectFaults()
	if _, err := conn.Write(buildSearchRequest(3)); err != nil {
		t.Fatalf("write normal search request: %v", err)
	}
	if tag, _ := ldapResultCode(readBERPacket(t, conn)); tag != 0x64 {
		t.Fatalf("normal first search packet tag = %#x, want 0x64", tag)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x65 || code != 0x00 {
		t.Fatalf("normal search response tag/code = %#x/%#x, want 0x65/0x00", tag, code)
	}
}

func TestLDAPServer_LDAPFault_CustomResultCode(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := ldapserver.New(&config.LDAPConfig{Enabled: true, Port: port}, state.New(), sc, logger.New(100))
	startServer(t, srv)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial LDAP: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(buildBindRequest(1)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	_ = readBERPacket(t, conn)
	sc.SetDirectFaults(config.ProtocolFaults{LDAP: &config.LDAPFault{ResultCode: 32, ErrorRate: 0}})
	if _, err := conn.Write(buildSearchRequest(2)); err != nil {
		t.Fatalf("search: %v", err)
	}
	if tag, code := ldapResultCode(readBERPacket(t, conn)); tag != 0x65 || code != 0x20 {
		t.Fatalf("fault search response tag/code = %#x/%#x, want 0x65/0x20", tag, code)
	}
}
