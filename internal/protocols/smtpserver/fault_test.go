package smtpserver_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
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

func readSMTPResponse(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var b strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SMTP response: %v", err)
		}
		b.WriteString(line)
		if len(line) >= 4 && line[3] == ' ' {
			return b.String()
		}
	}
}

func sendSMTP(t *testing.T, conn net.Conn, reader *bufio.Reader, cmd string) string {
	t.Helper()
	if _, err := io.WriteString(conn, cmd); err != nil {
		t.Fatalf("write SMTP command %q: %v", strings.TrimSpace(cmd), err)
	}
	return readSMTPResponse(t, reader)
}

func smtpTransaction(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial SMTP: %v", err)
	}
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	if resp := readSMTPResponse(t, reader); !strings.HasPrefix(resp, "220 ") {
		t.Fatalf("banner = %q, want 220", resp)
	}
	if resp := sendSMTP(t, conn, reader, "EHLO test\r\n"); !strings.HasPrefix(resp, "250") {
		t.Fatalf("EHLO response = %q", resp)
	}
	if resp := sendSMTP(t, conn, reader, "MAIL FROM:<a@b.com>\r\n"); !strings.HasPrefix(resp, "250 ") {
		t.Fatalf("MAIL FROM response = %q", resp)
	}
	if resp := sendSMTP(t, conn, reader, "RCPT TO:<c@d.com>\r\n"); !strings.HasPrefix(resp, "250 ") {
		t.Fatalf("RCPT TO response = %q", resp)
	}
	if resp := sendSMTP(t, conn, reader, "DATA\r\n"); !strings.HasPrefix(resp, "354 ") {
		t.Fatalf("DATA response = %q", resp)
	}
	return conn, reader
}

func TestSMTPServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := smtpserver.New(&config.SMTPConfig{
		Enabled: true,
		Port:    port,
		Rules:   []config.SMTPRule{{ID: "r", From: "*", To: "*", Action: "accept"}},
	}, sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{SMTP: &config.SMTPFault{ErrorRate: 0}})
	conn, reader := smtpTransaction(t, addr)
	faultResp := sendSMTP(t, conn, reader, "Subject: test\r\n\r\nhello\r\n.\r\n")
	_ = conn.Close()
	if !strings.HasPrefix(faultResp, "421 ") {
		t.Fatalf("fault response = %q, want 421", faultResp)
	}

	sc.ClearDirectFaults()
	conn, reader = smtpTransaction(t, addr)
	defer conn.Close() //nolint:errcheck
	normalResp := sendSMTP(t, conn, reader, "Subject: test\r\n\r\nhello\r\n.\r\n")
	if !strings.HasPrefix(normalResp, "250 ") {
		t.Fatalf("normal response = %q, want 250", normalResp)
	}
}

func TestSMTPServer_SMTPFault_CustomCode(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := smtpserver.New(&config.SMTPConfig{Enabled: true, Port: port, Rules: []config.SMTPRule{{ID: "r", From: "*", To: "*", Action: "accept"}}}, sc, logger.New(100))
	startServer(t, srv)

	sc.SetDirectFaults(config.ProtocolFaults{SMTP: &config.SMTPFault{Code: 550, Message: "mailbox unavailable", ErrorRate: 0}})
	conn, reader := smtpTransaction(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close() //nolint:errcheck
	resp := sendSMTP(t, conn, reader, "Subject: test\r\n\r\nhello\r\n.\r\n")
	if !strings.HasPrefix(resp, "550 4.3.0 mailbox unavailable") {
		t.Fatalf("fault response = %q", resp)
	}
}
