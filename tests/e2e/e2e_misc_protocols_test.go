//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// Blank-imported so its init() registers the raw-bytes gRPC codec
	// (overriding the default "proto" codec) in this test binary's process,
	// matching what the real mockly binary does in its own process. Without
	// this, grpc.ClientConn.Invoke would try to protobuf-marshal a []byte,
	// which fails.
	_ "github.com/dever-labs/mockly/internal/protocols/grpcserver"
)

// ---------------------------------------------------------------------------
// LDAP
// ---------------------------------------------------------------------------

// berLen/berTLV/buildLDAPMessage/buildBindRequest/buildSearchRequest/
// readBERPacket/readTLV/ldapResultCode replicate the hand-rolled BER
// encoding used by internal/protocols/ldapserver's own fault_test.go, since
// no LDAP client dependency exists in go.mod.

func ldapBerLen(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n < 0x100 {
		return []byte{0x81, byte(n)}
	}
	return []byte{0x82, byte(n >> 8), byte(n)}
}

func ldapBerTLV(tag byte, content []byte) []byte {
	out := []byte{tag}
	out = append(out, ldapBerLen(len(content))...)
	out = append(out, content...)
	return out
}

func buildLDAPMessage(msgID int, tag byte, content []byte) []byte {
	body := append(ldapBerTLV(0x02, []byte{byte(msgID)}), ldapBerTLV(tag, content)...)
	return ldapBerTLV(0x30, body)
}

func buildBindRequest(msgID int) []byte {
	content := append(ldapBerTLV(0x02, []byte{0x03}), ldapBerTLV(0x04, nil)...)
	content = append(content, 0x80, 0x00)
	return buildLDAPMessage(msgID, 0x60, content)
}

func buildSearchRequest(msgID int, baseDN string) []byte {
	content := ldapBerTLV(0x04, []byte(baseDN))
	content = append(content, ldapBerTLV(0x0a, []byte{0x02})...)
	content = append(content, ldapBerTLV(0x0a, []byte{0x00})...)
	content = append(content, ldapBerTLV(0x02, []byte{0x00})...)
	content = append(content, ldapBerTLV(0x02, []byte{0x00})...)
	content = append(content, ldapBerTLV(0x01, []byte{0x00})...)
	content = append(content, ldapBerTLV(0x87, []byte("objectClass"))...)
	content = append(content, ldapBerTLV(0x30, nil)...)
	return buildLDAPMessage(msgID, 0x63, content)
}

func readLDAPPacket(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	head := make([]byte, 2)
	if _, err := readFull(conn, head); err != nil {
		t.Fatalf("read LDAP header: %v", err)
	}
	length := int(head[1])
	if head[1]&0x80 != 0 {
		extra := int(head[1] & 0x7f)
		buf := make([]byte, extra)
		if _, err := readFull(conn, buf); err != nil {
			t.Fatalf("read LDAP length bytes: %v", err)
		}
		length = 0
		for _, b := range buf {
			length = (length << 8) | int(b)
		}
		head = append(head, buf...)
	}
	body := make([]byte, length)
	if _, err := readFull(conn, body); err != nil {
		t.Fatalf("read LDAP packet body: %v", err)
	}
	return append(head, body...)
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func readLDAPTLV(b []byte) (byte, []byte, int) {
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

// ldapSearchEntryAttrs decodes a SearchResultEntry packet (opTag 0x64) and
// returns its attribute map, so the test can assert on real mocked values
// rather than merely on "a response arrived".
func ldapSearchEntryAttrs(t *testing.T, packet []byte) map[string][]string {
	t.Helper()
	_, content, _ := readLDAPTLV(packet) // outer SEQUENCE
	_, _, next := readLDAPTLV(content)   // msgID
	opTag, opContent, _ := readLDAPTLV(content[next:])
	if opTag != 0x64 {
		t.Fatalf("expected SearchResultEntry (0x64), got op tag %#x", opTag)
	}
	_, baseDNContent, baseNext := readLDAPTLV(opContent) // objectName
	_ = baseDNContent
	_, attrsContent, _ := readLDAPTLV(opContent[baseNext:]) // PartialAttributeList SEQUENCE
	attrs := map[string][]string{}
	rest := attrsContent
	for len(rest) > 0 {
		_, attrSeq, consumed := readLDAPTLV(rest)
		rest = rest[consumed:]
		_, nameContent, n2 := readLDAPTLV(attrSeq)
		name := string(nameContent)
		_, valsContent, _ := readLDAPTLV(attrSeq[n2:])
		var vals []string
		vrest := valsContent
		for len(vrest) > 0 {
			_, v, vconsumed := readLDAPTLV(vrest)
			vals = append(vals, string(v))
			vrest = vrest[vconsumed:]
		}
		attrs[name] = vals
	}
	return attrs
}

// TestE2E_LDAP_MockAndRequest starts the real mockly binary with the LDAP
// protocol enabled via a mockly.yaml config, dials it over raw TCP, and
// performs a bind + search round-trip using hand-rolled BER encoding (there
// is no LDAP client dependency in go.mod). It asserts the mocked attribute
// values are actually returned, proving cmd/mockly's
// `if cfg.Protocols.LDAP.Enabled` startup block really wires config-declared
// mocks into a running LDAP listener.
func TestE2E_LDAP_MockAndRequest(t *testing.T) {
	ldapPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  ldap:
    enabled: true
    port: %d
    mocks:
      - id: find-user
        base_dn: "dc=example,dc=com"
        attributes:
          cn: ["testuser"]
          mail: ["testuser@example.com"]
`, ldapPort)
	startMocklyConfig(t, cfg)

	conn := dialWithRetry(t, "tcp", fmt.Sprintf("127.0.0.1:%d", ldapPort), 5*time.Second)
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write(buildBindRequest(1)); err != nil {
		t.Fatalf("write bind request: %v", err)
	}
	bindResp := readLDAPPacket(t, conn)
	if opTag, _, _ := readLDAPTLVTop(bindResp); opTag != 0x61 {
		t.Fatalf("bind response op tag = %#x, want 0x61 (BindResponse)", opTag)
	}

	if _, err := conn.Write(buildSearchRequest(2, "dc=example,dc=com")); err != nil {
		t.Fatalf("write search request: %v", err)
	}
	entry := readLDAPPacket(t, conn)
	attrs := ldapSearchEntryAttrs(t, entry)
	if got := attrs["cn"]; len(got) != 1 || got[0] != "testuser" {
		t.Fatalf("cn attribute = %v, want [testuser]", got)
	}
	if got := attrs["mail"]; len(got) != 1 || got[0] != "testuser@example.com" {
		t.Fatalf("mail attribute = %v, want [testuser@example.com]", got)
	}

	done := readLDAPPacket(t, conn)
	if opTag, _, _ := readLDAPTLVTop(done); opTag != 0x65 {
		t.Fatalf("expected SearchResultDone (0x65), got %#x", opTag)
	}
}

// readLDAPTLVTop unwraps the outer SEQUENCE + msgID to return the operation
// tag/content of an LDAP message, mirroring parseLDAPEnvelope in the real
// server but reusing the local readLDAPTLV helper.
func readLDAPTLVTop(packet []byte) (byte, []byte, int) {
	_, content, _ := readLDAPTLV(packet)
	_, _, next := readLDAPTLV(content)
	return readLDAPTLV(content[next:])
}

// ---------------------------------------------------------------------------
// CoAP
// ---------------------------------------------------------------------------

// buildCoAPGet mirrors the hand-rolled CoAP GET request used in
// internal/protocols/coapserver's own fault_test.go: a confirmable GET with
// a 1-byte token and a single Uri-Path option.
func buildCoAPGet(path string, msgID uint16, token byte) []byte {
	parts := []byte{0x41, 0x01, 0, 0, token}
	binary.BigEndian.PutUint16(parts[2:4], msgID)
	segment := strings.TrimPrefix(path, "/")
	parts = append(parts, byte((11<<4)|len(segment)))
	parts = append(parts, []byte(segment)...)
	return parts
}

// coapResponsePayload extracts the payload (bytes after the 0xFF marker) and
// the response code byte from a raw CoAP response packet.
func coapResponsePayload(t *testing.T, resp []byte) (codeByte byte, payload string) {
	t.Helper()
	if len(resp) < 4 {
		t.Fatalf("CoAP response too short: %d bytes", len(resp))
	}
	codeByte = resp[1]
	tokenLen := int(resp[0] & 0x0f)
	rest := resp[4+tokenLen:]
	idx := bytes.IndexByte(rest, 0xff)
	if idx == -1 {
		return codeByte, ""
	}
	return codeByte, string(rest[idx+1:])
}

// TestE2E_CoAP_MockAndRequest starts the real mockly binary with the CoAP
// protocol enabled via a mockly.yaml config, sends a hand-rolled CoAP GET
// over UDP, and asserts the mocked response code and payload are returned —
// proving cmd/mockly's `if cfg.Protocols.CoAP.Enabled` startup block wires
// config-declared mocks into a running CoAP (UDP) listener.
func TestE2E_CoAP_MockAndRequest(t *testing.T) {
	coapPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  coap:
    enabled: true
    port: %d
    mocks:
      - id: get-temp
        method: GET
        path: /temp
        response:
          code: "2.05"
          payload: "25.5C"
`, coapPort)
	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", coapPort)
	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("udp", addr, 200*time.Millisecond)
		if err == nil {
			conn = c
			break
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	if conn == nil {
		t.Fatalf("dial CoAP: %v", lastErr)
	}
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write(buildCoAPGet("/temp", 0x1234, 0x7a)); err != nil {
		t.Fatalf("write CoAP request: %v", err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read CoAP response: %v", err)
	}
	code, payload := coapResponsePayload(t, buf[:n])
	if code != 0x45 {
		t.Fatalf("CoAP response code byte = %#x, want 0x45 (2.05 Content)", code)
	}
	if payload != "25.5C" {
		t.Fatalf("CoAP payload = %q, want %q", payload, "25.5C")
	}
}

// ---------------------------------------------------------------------------
// SIP
// ---------------------------------------------------------------------------

func sipInvite() string {
	return "INVITE sip:alice@example.com SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP 127.0.0.1:5060\r\n" +
		"From: <sip:bob@example.com>\r\n" +
		"To: <sip:alice@example.com>\r\n" +
		"Call-ID: 1234@example.com\r\n" +
		"CSeq: 1 INVITE\r\n" +
		"Content-Length: 0\r\n\r\n"
}

// TestE2E_SIP_MockAndRequest starts the real mockly binary with the SIP
// protocol enabled via a mockly.yaml config, sends a plain-text SIP INVITE
// over UDP, and asserts the mocked status/reason line is returned — proving
// cmd/mockly's `if cfg.Protocols.SIP.Enabled` startup block wires
// config-declared mocks into a running SIP (UDP) listener.
func TestE2E_SIP_MockAndRequest(t *testing.T) {
	sipPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  sip:
    enabled: true
    port: %d
    mocks:
      - id: invite-ok
        method: INVITE
        uri: "sip:alice@example.com"
        response:
          status: 200
          reason: OK
          headers:
            X-Mock: "true"
`, sipPort)
	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", sipPort)
	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("udp", addr, 200*time.Millisecond)
		if err == nil {
			conn = c
			break
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	if conn == nil {
		t.Fatalf("dial SIP: %v", lastErr)
	}
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write([]byte(sipInvite())); err != nil {
		t.Fatalf("write SIP INVITE: %v", err)
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read SIP response: %v", err)
	}
	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "SIP/2.0 200 OK") {
		t.Fatalf("SIP response = %q, want prefix %q", resp, "SIP/2.0 200 OK")
	}
	if !strings.Contains(resp, "X-Mock: true") {
		t.Fatalf("SIP response missing mocked header, got: %q", resp)
	}
}

// ---------------------------------------------------------------------------
// GraphQL
// ---------------------------------------------------------------------------

// TestE2E_GraphQL_MockAndRequest starts the real mockly binary with the
// GraphQL protocol enabled via a mockly.yaml config, POSTs a JSON GraphQL
// query to the configured path over plain HTTP, and asserts the mocked
// `data` field is returned — proving cmd/mockly's
// `if cfg.Protocols.GraphQL.Enabled` startup block wires config-declared
// mocks into a running GraphQL HTTP listener.
func TestE2E_GraphQL_MockAndRequest(t *testing.T) {
	gqlPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  graphql:
    enabled: true
    port: %d
    path: /graphql
    mocks:
      - id: get-user
        operation_type: query
        operation_name: GetUser
        response:
          user:
            id: "42"
            name: "Alice"
`, gqlPort)
	startMocklyConfig(t, cfg)

	gqlBase := fmt.Sprintf("http://127.0.0.1:%d/graphql", gqlPort)
	body, err := json.Marshal(map[string]interface{}{
		"query":         "query GetUser { user { id name } }",
		"operationName": "GetUser",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var resp *http.Response
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, postErr := http.Post(gqlBase, "application/json", bytes.NewReader(body))
		if postErr == nil {
			resp = r
			break
		}
		err = postErr
		time.Sleep(20 * time.Millisecond)
	}
	if resp == nil {
		t.Fatalf("POST %s: %v", gqlBase, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GraphQL response status = %d, want 200", resp.StatusCode)
	}
	var decoded struct {
		Data struct {
			User struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode GraphQL response: %v", err)
	}
	if decoded.Data.User.ID != "42" || decoded.Data.User.Name != "Alice" {
		t.Fatalf("unexpected GraphQL data: %+v", decoded.Data)
	}
}

// ---------------------------------------------------------------------------
// gRPC
// ---------------------------------------------------------------------------

// TestE2E_GRPC_MockAndRequest starts the real mockly binary with the gRPC
// protocol enabled via a mockly.yaml config, and invokes a mocked method
// using google.golang.org/grpc's generic Invoke with a raw-bytes JSON
// payload (the mockly gRPC server uses grpc.UnknownServiceHandler plus a
// codec that overrides the default "proto" codec to pass bytes straight
// through, so no compiled .proto/generated stubs are needed — see
// internal/protocols/grpcserver/server_test.go for the same pattern). It
// asserts the mocked JSON response body is returned, proving cmd/mockly's
// `if cfg.Protocols.GRPC.Enabled` startup block wires config-declared mocks
// into a running gRPC listener.
func TestE2E_GRPC_MockAndRequest(t *testing.T) {
	grpcPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  grpc:
    enabled: true
    port: %d
    services:
      - mocks:
          - id: get-user
            method: GetUser
            response:
              id: "123"
              name: "Alice"
`, grpcPort)
	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", grpcPort)
	// Wait for the gRPC listener to accept TCP connections before dialing
	// with the gRPC client, matching dialWithRetry's intent for non-HTTP
	// protocol ports.
	conn0 := dialWithRetry(t, "tcp", addr, 5*time.Second)
	_ = conn0.Close()

	client, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer client.Close() //nolint:errcheck

	var resp []byte
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Invoke(ctx, "/example.UserService/GetUser", []byte(`{}`), &resp); err != nil {
		t.Fatalf("Invoke GetUser: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(resp, &decoded); err != nil {
		t.Fatalf("unmarshal gRPC response: %v", err)
	}
	if decoded["id"] != "123" || decoded["name"] != "Alice" {
		t.Fatalf("unexpected gRPC response body: %v", decoded)
	}
}
