package ftpserver_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/ftpserver"
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

func readFTPLine(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read FTP line: %v", err)
	}
	return line
}

func sendFTPCommand(t *testing.T, conn net.Conn, reader *bufio.Reader, cmd string) string {
	t.Helper()
	if _, err := io.WriteString(conn, cmd+"\r\n"); err != nil {
		t.Fatalf("write FTP command %q: %v", cmd, err)
	}
	return readFTPLine(t, reader)
}

func parsePASVAddr(t *testing.T, line string) string {
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

func loginFTP(t *testing.T, conn net.Conn, reader *bufio.Reader) {
	t.Helper()
	if banner := readFTPLine(t, reader); !strings.HasPrefix(banner, "220 ") {
		t.Fatalf("banner = %q, want 220", banner)
	}
	if resp := sendFTPCommand(t, conn, reader, "USER anonymous"); !strings.HasPrefix(resp, "331 ") {
		t.Fatalf("USER response = %q, want 331", resp)
	}
	if resp := sendFTPCommand(t, conn, reader, "PASS x"); !strings.HasPrefix(resp, "230 ") {
		t.Fatalf("PASS response = %q, want 230", resp)
	}
}

func TestFTPServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := ftpserver.New(&config.FTPConfig{
		Enabled: true,
		Port:    port,
		Files: []config.FTPFile{{
			ID:      "f",
			Path:    "/test.txt",
			Content: "hello",
		}},
	}, sc, logger.New(100))
	startServer(t, srv)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sc.SetDirectFaults(config.ProtocolFaults{FTP: &config.FTPFault{ErrorRate: 0}})

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial FTP: %v", err)
	}
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	loginFTP(t, conn, reader)
	faultResp := sendFTPCommand(t, conn, reader, "RETR /test.txt")
	_ = conn.Close()
	if !strings.HasPrefix(faultResp, "421 ") {
		t.Fatalf("fault response = %q, want 421", faultResp)
	}

	sc.ClearDirectFaults()

	conn, err = net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial FTP normal: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	reader = bufio.NewReader(conn)
	loginFTP(t, conn, reader)
	pasvResp := sendFTPCommand(t, conn, reader, "PASV")
	if !strings.HasPrefix(pasvResp, "227 ") {
		t.Fatalf("PASV response = %q, want 227", pasvResp)
	}
	dataConn, err := net.DialTimeout("tcp", parsePASVAddr(t, pasvResp), time.Second)
	if err != nil {
		t.Fatalf("dial PASV data connection: %v", err)
	}
	dataConn.SetDeadline(time.Now().Add(2 * time.Second))
	defer dataConn.Close() //nolint:errcheck
	if _, err := io.WriteString(conn, "RETR /test.txt\r\n"); err != nil {
		t.Fatalf("write RETR: %v", err)
	}
	if resp := readFTPLine(t, reader); !strings.HasPrefix(resp, "150 ") {
		t.Fatalf("RETR initial response = %q, want 150", resp)
	}
	body, err := io.ReadAll(dataConn)
	if err != nil {
		t.Fatalf("read data connection: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("retrieved body = %q, want %q", body, "hello")
	}
	if resp := readFTPLine(t, reader); !strings.HasPrefix(resp, "226 ") {
		t.Fatalf("RETR completion response = %q, want 226", resp)
	}
}

func TestFTPServer_FTPFault_CustomCode(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := ftpserver.New(&config.FTPConfig{Enabled: true, Port: port, Files: []config.FTPFile{{ID: "f", Path: "/test.txt", Content: "hello"}}}, sc, logger.New(100))
	startServer(t, srv)
	sc.SetDirectFaults(config.ProtocolFaults{FTP: &config.FTPFault{Code: 530, Message: "denied", ErrorRate: 0}})
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial FTP: %v", err)
	}
	defer conn.Close() //nolint:errcheck
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	loginFTP(t, conn, reader)
	resp := sendFTPCommand(t, conn, reader, "LIST")
	if !strings.HasPrefix(resp, "530 denied") {
		t.Fatalf("fault response = %q", resp)
	}
}
