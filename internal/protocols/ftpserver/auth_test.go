package ftpserver_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/ftpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
)

func dialFTP(t *testing.T, port int) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial FTP: %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	if banner := readFTPLine(t, reader); !strings.HasPrefix(banner, "220 ") {
		t.Fatalf("banner = %q, want 220", banner)
	}
	return conn, reader
}

func TestFTPServer_Auth_UnconfiguredUsers_AnyCredentialsSucceed(t *testing.T) {
	port := freePort(t)
	srv := ftpserver.New(&config.FTPConfig{Enabled: true, Port: port}, scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn, reader := dialFTP(t, port)
	defer conn.Close() //nolint:errcheck

	if resp := sendFTPCommand(t, conn, reader, "USER anyone"); !strings.HasPrefix(resp, "331 ") {
		t.Fatalf("USER response = %q, want 331", resp)
	}
	if resp := sendFTPCommand(t, conn, reader, "PASS whatever"); !strings.HasPrefix(resp, "230 ") {
		t.Fatalf("PASS response = %q, want 230 (unconfigured users should always succeed)", resp)
	}
}

func TestFTPServer_Auth_WrongCredentials_Rejected(t *testing.T) {
	port := freePort(t)
	srv := ftpserver.New(&config.FTPConfig{
		Enabled: true,
		Port:    port,
		Users:   []config.FTPUser{{Username: "alice", Password: "secret123"}},
	}, scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn, reader := dialFTP(t, port)
	defer conn.Close() //nolint:errcheck

	_ = sendFTPCommand(t, conn, reader, "USER alice")
	if resp := sendFTPCommand(t, conn, reader, "PASS wrong"); !strings.HasPrefix(resp, "530 ") {
		t.Fatalf("PASS response = %q, want 530 Login incorrect", resp)
	}
}

func TestFTPServer_Auth_CorrectCredentials_Succeeds(t *testing.T) {
	port := freePort(t)
	srv := ftpserver.New(&config.FTPConfig{
		Enabled: true,
		Port:    port,
		Users:   []config.FTPUser{{Username: "alice", Password: "secret123"}},
	}, scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn, reader := dialFTP(t, port)
	defer conn.Close() //nolint:errcheck

	_ = sendFTPCommand(t, conn, reader, "USER alice")
	if resp := sendFTPCommand(t, conn, reader, "PASS secret123"); !strings.HasPrefix(resp, "230 ") {
		t.Fatalf("PASS response = %q, want 230", resp)
	}
}

func TestFTPServer_Auth_CommandsBeforeLogin_Rejected(t *testing.T) {
	port := freePort(t)
	srv := ftpserver.New(&config.FTPConfig{
		Enabled: true,
		Port:    port,
		Users:   []config.FTPUser{{Username: "alice", Password: "secret123"}},
		Files:   []config.FTPFile{{ID: "f1", Path: "/test.txt", Content: "hello"}},
	}, scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn, reader := dialFTP(t, port)
	defer conn.Close() //nolint:errcheck

	if resp := sendFTPCommand(t, conn, reader, "PASV"); !strings.HasPrefix(resp, "227 ") {
		t.Fatalf("PASV response = %q, want 227 (not login-gated)", resp)
	}
	if resp := sendFTPCommand(t, conn, reader, "RETR /test.txt"); !strings.HasPrefix(resp, "530 ") {
		t.Fatalf("RETR response = %q, want 530 (login required once Users is configured)", resp)
	}
}

func TestFTPServer_Auth_AllowedFiles_RestrictsAccess(t *testing.T) {
	port := freePort(t)
	srv := ftpserver.New(&config.FTPConfig{
		Enabled: true,
		Port:    port,
		Users: []config.FTPUser{
			{Username: "alice", Password: "secret123", AllowedFiles: []string{"f-alice"}},
		},
		Files: []config.FTPFile{
			{ID: "f-alice", Path: "/alice.txt", Content: "alice-data"},
			{ID: "f-bob", Path: "/bob.txt", Content: "bob-data"},
		},
	}, scenarios.New(nil), logger.New(100))
	startServer(t, srv)
	conn, reader := dialFTP(t, port)
	defer conn.Close() //nolint:errcheck

	_ = sendFTPCommand(t, conn, reader, "USER alice")
	if resp := sendFTPCommand(t, conn, reader, "PASS secret123"); !strings.HasPrefix(resp, "230 ") {
		t.Fatalf("PASS response = %q, want 230", resp)
	}

	// Alice can retrieve her own file.
	pasvResp := sendFTPCommand(t, conn, reader, "PASV")
	dataConn, err := net.DialTimeout("tcp", parsePASVAddr(t, pasvResp), time.Second)
	if err != nil {
		t.Fatalf("dial PASV: %v", err)
	}
	_ = dataConn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(conn, "RETR /alice.txt\r\n"); err != nil {
		t.Fatalf("write RETR: %v", err)
	}
	if resp := readFTPLine(t, reader); !strings.HasPrefix(resp, "150 ") {
		t.Fatalf("RETR /alice.txt response = %q, want 150", resp)
	}
	body, _ := io.ReadAll(dataConn)
	_ = dataConn.Close()
	if string(body) != "alice-data" {
		t.Fatalf("RETR body = %q, want alice-data", body)
	}
	_ = readFTPLine(t, reader) // 226 Transfer complete

	// Alice cannot retrieve bob's file.
	pasvResp = sendFTPCommand(t, conn, reader, "PASV")
	if resp := sendFTPCommand(t, conn, reader, "RETR /bob.txt"); !strings.HasPrefix(resp, "550 ") {
		t.Fatalf("RETR /bob.txt response = %q, want 550 No such file", resp)
	}
	_ = pasvResp
}
