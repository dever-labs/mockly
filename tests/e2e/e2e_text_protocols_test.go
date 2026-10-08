//go:build e2e

package e2e

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/miekg/dns"
)

// ---------------------------------------------------------------------------
// SMTP
// ---------------------------------------------------------------------------

// TestE2E_SMTP_MockAndRequest starts the real mockly binary from a YAML
// config enabling only protocols.smtp, sends a real email to it using Go's
// stdlib net/smtp client (HELO, MAIL FROM, RCPT TO, DATA), and asserts via
// the management API's GET /api/emails that the message was captured with
// the expected sender, recipient, and subject — proving cmd/mockly's
// `if cfg.Protocols.SMTP.Enabled` startup block really wires up a working
// SMTP server from config, not just that the protocol logic works in
// isolation (which internal/protocols/smtpserver already covers).
func TestE2E_SMTP_MockAndRequest(t *testing.T) {
	smtpPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  smtp:
    enabled: true
    port: %d
    domain: mockly.local
    rules:
      - id: accept-all
        action: accept
`, smtpPort)

	apiBase := startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", smtpPort)
	dialWithRetry(t, "tcp", addr, 5*time.Second).Close() //nolint:errcheck

	msg := []byte("Subject: Hello Mockly\r\n" +
		"\r\n" +
		"This is a test email body.\r\n")

	if err := smtp.SendMail(addr, nil, "sender@example.com", []string{"recipient@example.com"}, msg); err != nil {
		t.Fatalf("smtp.SendMail: %v", err)
	}

	var emails []config.ReceivedEmail
	mustGetJSON(t, apiBase+"/api/emails", &emails)
	if len(emails) != 1 {
		t.Fatalf("expected 1 captured email, got %d: %#v", len(emails), emails)
	}
	got := emails[0]
	if got.From != "sender@example.com" {
		t.Errorf("From = %q, want sender@example.com", got.From)
	}
	if len(got.To) != 1 || got.To[0] != "recipient@example.com" {
		t.Errorf("To = %#v, want [recipient@example.com]", got.To)
	}
	if got.Subject != "Hello Mockly" {
		t.Errorf("Subject = %q, want %q", got.Subject, "Hello Mockly")
	}
}

// ---------------------------------------------------------------------------
// IMAP
// ---------------------------------------------------------------------------

// TestE2E_IMAP_MockAndRequest starts the real mockly binary from a YAML
// config enabling only protocols.imap with a pre-configured mailbox and
// message, then issues the raw IMAP wire commands LOGIN, SELECT, and FETCH
// over a plain net.Dial connection (mirroring
// internal/protocols/imapserver/fault_test.go's pattern), asserting the
// fetched message body matches what was configured. This proves
// cmd/mockly's IMAP startup block actually loads config-driven mailboxes
// into a reachable server.
func TestE2E_IMAP_MockAndRequest(t *testing.T) {
	imapPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  imap:
    enabled: true
    port: %d
    users:
      - username: user
        password: pass
    mailboxes:
      - id: inbox
        name: INBOX
        messages:
          - seq_num: 1
            from: "sender@example.com"
            to: "user@example.com"
            subject: "Test email"
            body: "Hello world"
`, imapPort)

	startMocklyConfig(t, cfg)

	conn := dialWithRetry(t, "tcp", fmt.Sprintf("127.0.0.1:%d", imapPort), 5*time.Second)
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)

	banner, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read banner: %v", err)
	}
	if !strings.HasPrefix(banner, "* OK") {
		t.Fatalf("banner = %q, want * OK", banner)
	}

	if resp := sendIMAPCmd(t, conn, reader, "a1 LOGIN user pass\r\n", "a1"); !strings.Contains(resp, "a1 OK") {
		t.Fatalf("LOGIN response = %q", resp)
	}
	if resp := sendIMAPCmd(t, conn, reader, "a2 SELECT INBOX\r\n", "a2"); !strings.Contains(resp, "a2 OK") {
		t.Fatalf("SELECT response = %q", resp)
	}
	fetchResp := sendIMAPCmd(t, conn, reader, "a3 FETCH 1 BODY[]\r\n", "a3")
	if !strings.Contains(fetchResp, "* 1 FETCH") {
		t.Fatalf("FETCH response = %q, want FETCH data", fetchResp)
	}
	if !strings.Contains(fetchResp, "Hello world") {
		t.Fatalf("FETCH response = %q, want body %q", fetchResp, "Hello world")
	}
	if !strings.Contains(fetchResp, "a3 OK FETCH completed") {
		t.Fatalf("FETCH response = %q, want tagged OK", fetchResp)
	}
}

func sendIMAPCmd(t *testing.T, conn net.Conn, reader *bufio.Reader, cmd, tag string) string {
	t.Helper()
	if _, err := io.WriteString(conn, cmd); err != nil {
		t.Fatalf("write IMAP command %q: %v", cmd, err)
	}
	var b strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read IMAP line: %v", err)
		}
		b.WriteString(line)
		if strings.HasPrefix(line, tag+" ") {
			return b.String()
		}
	}
}

// ---------------------------------------------------------------------------
// FTP
// ---------------------------------------------------------------------------

// TestE2E_FTP_MockAndRequest starts the real mockly binary from a YAML
// config enabling only protocols.ftp with a pre-configured file, then logs
// in, enters passive mode, and retrieves the file over raw FTP wire commands
// (mirroring internal/protocols/ftpserver/fault_test.go's pattern), asserting
// the retrieved content matches the configured content. This proves
// cmd/mockly's FTP startup block actually loads config-driven files into a
// reachable, fully-functional (control + data connection) server.
func TestE2E_FTP_MockAndRequest(t *testing.T) {
	ftpPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  ftp:
    enabled: true
    port: %d
    files:
      - id: test-file
        path: /test.txt
        content: "hello from mockly"
`, ftpPort)

	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", ftpPort)
	conn := dialWithRetry(t, "tcp", addr, 5*time.Second)
	defer conn.Close() //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)

	banner := readFTPLineE2E(t, reader)
	if !strings.HasPrefix(banner, "220 ") {
		t.Fatalf("banner = %q, want 220", banner)
	}
	if resp := sendFTPCmd(t, conn, reader, "USER anonymous"); !strings.HasPrefix(resp, "331 ") {
		t.Fatalf("USER response = %q, want 331", resp)
	}
	if resp := sendFTPCmd(t, conn, reader, "PASS x"); !strings.HasPrefix(resp, "230 ") {
		t.Fatalf("PASS response = %q, want 230", resp)
	}

	pasvResp := sendFTPCmd(t, conn, reader, "PASV")
	if !strings.HasPrefix(pasvResp, "227 ") {
		t.Fatalf("PASV response = %q, want 227", pasvResp)
	}
	dataAddr := parsePASVAddrE2E(t, pasvResp)
	dataConn, err := net.DialTimeout("tcp", dataAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial PASV data connection: %v", err)
	}
	defer dataConn.Close() //nolint:errcheck
	_ = dataConn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := io.WriteString(conn, "RETR /test.txt\r\n"); err != nil {
		t.Fatalf("write RETR: %v", err)
	}
	if resp := readFTPLineE2E(t, reader); !strings.HasPrefix(resp, "150 ") {
		t.Fatalf("RETR initial response = %q, want 150", resp)
	}
	body, err := io.ReadAll(dataConn)
	if err != nil {
		t.Fatalf("read data connection: %v", err)
	}
	if string(body) != "hello from mockly" {
		t.Fatalf("retrieved body = %q, want %q", body, "hello from mockly")
	}
	if resp := readFTPLineE2E(t, reader); !strings.HasPrefix(resp, "226 ") {
		t.Fatalf("RETR completion response = %q, want 226", resp)
	}
}

func readFTPLineE2E(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read FTP line: %v", err)
	}
	return line
}

func sendFTPCmd(t *testing.T, conn net.Conn, reader *bufio.Reader, cmd string) string {
	t.Helper()
	if _, err := io.WriteString(conn, cmd+"\r\n"); err != nil {
		t.Fatalf("write FTP command %q: %v", cmd, err)
	}
	return readFTPLineE2E(t, reader)
}

func parsePASVAddrE2E(t *testing.T, line string) string {
	t.Helper()
	start := strings.Index(line, "(")
	end := strings.Index(line, ")")
	if start < 0 || end < 0 || end <= start+1 {
		t.Fatalf("unexpected PASV response: %q", line)
	}
	parts := strings.Split(line[start+1:end], ",")
	if len(parts) != 6 {
		t.Fatalf("unexpected PASV tuple: %q", line)
	}
	host := strings.Join(parts[:4], ".")
	p1, err := strconv.Atoi(parts[4])
	if err != nil {
		t.Fatalf("parse PASV p1: %v", err)
	}
	p2, err := strconv.Atoi(parts[5])
	if err != nil {
		t.Fatalf("parse PASV p2: %v", err)
	}
	return fmt.Sprintf("%s:%d", host, p1*256+p2)
}

// ---------------------------------------------------------------------------
// DNS
// ---------------------------------------------------------------------------

// TestE2E_DNS_MockAndRequest starts the real mockly binary from a YAML
// config enabling only protocols.dns with a mocked A record, then sends a
// real DNS query over UDP using github.com/miekg/dns's client (already a
// go.mod dependency) and asserts the returned answer record matches the
// configured IP. This proves cmd/mockly's DNS startup block actually wires
// a config-driven UDP DNS server, not just that the resolver logic is
// correct in isolation (which internal/protocols/dnsserver already covers).
func TestE2E_DNS_MockAndRequest(t *testing.T) {
	dnsPort := freePort(t)
	cfg := fmt.Sprintf(`protocols:
  dns:
    enabled: true
    port: %d
    mocks:
      - id: api-host
        name: "api.example.com"
        type: A
        records:
          - "127.0.0.1"
        ttl: 60
`, dnsPort)

	startMocklyConfig(t, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", dnsPort)
	// DNS is UDP; wait for the control plane / listener to be ready by
	// dialing TCP first isn't applicable here (dnsserver listens on both
	// udp and tcp on the same port), so retry the actual DNS exchange
	// instead of dialWithRetry (which only dials, it doesn't query).
	var resp *dns.Msg
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		c := &dns.Client{Net: "udp", Timeout: 500 * time.Millisecond}
		m := new(dns.Msg)
		m.SetQuestion("api.example.com.", dns.TypeA)
		r, _, err := c.Exchange(m, addr)
		if err == nil {
			resp = r
			break
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	if resp == nil {
		t.Fatalf("dns exchange: %v", lastErr)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("rcode = %d, want %d", resp.Rcode, dns.RcodeSuccess)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer record, got %d: %#v", len(resp.Answer), resp.Answer)
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer record type = %T, want *dns.A", resp.Answer[0])
	}
	if a.A.String() != "127.0.0.1" {
		t.Errorf("A record = %q, want 127.0.0.1", a.A.String())
	}
}
