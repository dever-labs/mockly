package imapserver_test

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
	"github.com/dever-labs/mockly/internal/protocols/imapserver"
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

func readIMAPUntilTag(t *testing.T, reader *bufio.Reader, tag string) string {
	t.Helper()
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

func sendIMAP(t *testing.T, conn net.Conn, reader *bufio.Reader, cmd string, tag string) string {
	t.Helper()
	if _, err := io.WriteString(conn, cmd); err != nil {
		t.Fatalf("write IMAP command %q: %v", cmd, err)
	}
	return readIMAPUntilTag(t, reader, tag)
}

func TestIMAPServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := imapserver.New(&config.IMAPConfig{
		Enabled: true,
		Port:    port,
		Users:   []config.IMAPUser{{Username: "user", Password: "pass"}},
		Mailboxes: []config.IMAPMailbox{{
			ID:   "mb",
			Name: "INBOX",
			Messages: []config.IMAPMessage{{
				SeqNum:  1,
				From:    "a@b.com",
				To:      "c@d.com",
				Subject: "hi",
				Body:    "hello",
			}},
		}},
	}, sc, logger.New(100))
	startServer(t, srv)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial IMAP: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	banner, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read banner: %v", err)
	}
	if !strings.HasPrefix(banner, "* OK") {
		t.Fatalf("banner = %q, want * OK", banner)
	}
	if resp := sendIMAP(t, conn, reader, "a1 LOGIN user pass\r\n", "a1"); !strings.Contains(resp, "a1 OK") {
		t.Fatalf("LOGIN response = %q", resp)
	}
	if resp := sendIMAP(t, conn, reader, "a2 SELECT INBOX\r\n", "a2"); !strings.Contains(resp, "a2 OK") {
		t.Fatalf("SELECT response = %q", resp)
	}

	sc.SetDirectFaults(config.ProtocolFaults{IMAP: &config.IMAPFault{ErrorRate: 0}})
	faultResp := sendIMAP(t, conn, reader, "a3 FETCH 1 BODY[]\r\n", "a3")
	if !strings.Contains(faultResp, "a3 NO fault injected") {
		t.Fatalf("fault response = %q", faultResp)
	}

	sc.ClearDirectFaults()
	normalResp := sendIMAP(t, conn, reader, "a4 FETCH 1 BODY[]\r\n", "a4")
	if !strings.Contains(normalResp, "* 1 FETCH") {
		t.Fatalf("normal response = %q, want FETCH data", normalResp)
	}
	if !strings.Contains(normalResp, "a4 OK FETCH completed") {
		t.Fatalf("normal response = %q, want tagged OK", normalResp)
	}
}

func TestIMAPServer_IMAPFault_BadResponse(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := imapserver.New(&config.IMAPConfig{Enabled: true, Port: port, Users: []config.IMAPUser{{Username: "user", Password: "pass"}}, Mailboxes: []config.IMAPMailbox{{ID: "mb", Name: "INBOX"}}}, sc, logger.New(100))
	startServer(t, srv)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial IMAP: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	_, _ = reader.ReadString('\n')
	_ = sendIMAP(t, conn, reader, "a1 LOGIN user pass\r\n", "a1")
	_ = sendIMAP(t, conn, reader, "a2 SELECT INBOX\r\n", "a2")
	sc.SetDirectFaults(config.ProtocolFaults{IMAP: &config.IMAPFault{Response: "BAD", Message: "broken", ErrorRate: 0}})
	resp := sendIMAP(t, conn, reader, "a3 SEARCH ALL\r\n", "a3")
	if !strings.Contains(resp, "a3 BAD broken") {
		t.Fatalf("fault response = %q", resp)
	}
}
