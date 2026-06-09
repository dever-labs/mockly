// White-box unit and integration tests for ftpserver.
package ftpserver

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
)

// ---------------------------------------------------------------------------
// New / SetFiles / GetFiles
// ---------------------------------------------------------------------------

func newTestFTPServer(t *testing.T, files []config.FTPFile) *Server {
	t.Helper()
	cfg := &config.FTPConfig{Enabled: true, Port: 0, Files: files}
	return New(cfg, scenarios.New(nil), logger.New(100))
}

func TestFTP_New_InitialFiles(t *testing.T) {
	files := []config.FTPFile{{ID: "f1", Path: "/data.csv", Content: "a,b,c"}}
	srv := newTestFTPServer(t, files)
	got := srv.GetFiles()
	if len(got) != 1 || got[0].ID != "f1" {
		t.Fatalf("unexpected files from New: %+v", got)
	}
}

func TestFTP_SetFiles_ReplacesList(t *testing.T) {
	srv := newTestFTPServer(t, nil)
	srv.SetFiles([]config.FTPFile{
		{ID: "a", Path: "/a.txt", Content: "hello"},
		{ID: "b", Path: "/b.txt", Content: "world"},
	})
	got := srv.GetFiles()
	if len(got) != 2 {
		t.Fatalf("want 2 files, got %d", len(got))
	}
}

func TestFTP_SetFiles_IsolatesSlice(t *testing.T) {
	srv := newTestFTPServer(t, nil)
	original := []config.FTPFile{{ID: "orig", Path: "/orig.txt", Content: "x"}}
	srv.SetFiles(original)
	original[0].Content = "mutated"
	if srv.GetFiles()[0].Content != "x" {
		t.Error("SetFiles should copy the slice")
	}
}

func TestFTP_GetFiles_IsolatesSlice(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{{ID: "f1", Path: "/f.txt", Content: "orig"}})
	got := srv.GetFiles()
	got[0].Content = "mutated"
	if srv.GetFiles()[0].Content != "orig" {
		t.Error("GetFiles should return a copy")
	}
}

// ---------------------------------------------------------------------------
// normalizeFTPFiles
// ---------------------------------------------------------------------------

func TestNormalizeFTPFiles_SetsSize(t *testing.T) {
	files := normalizeFTPFiles([]config.FTPFile{{ID: "f1", Path: "/f.txt", Content: "hello"}})
	if files[0].Size != 5 {
		t.Errorf("want size=5, got %d", files[0].Size)
	}
}

func TestNormalizeFTPFiles_PreservesExplicitSize(t *testing.T) {
	files := normalizeFTPFiles([]config.FTPFile{{ID: "f1", Path: "/f.txt", Content: "hello", Size: 999}})
	if files[0].Size != 999 {
		t.Errorf("explicit size should be preserved, got %d", files[0].Size)
	}
}

func TestNormalizeFTPFiles_DefaultPermissions(t *testing.T) {
	files := normalizeFTPFiles([]config.FTPFile{{ID: "f1", Path: "/f.txt"}})
	if files[0].Permissions != "-rw-r--r--" {
		t.Errorf("want default permissions '-rw-r--r--', got %q", files[0].Permissions)
	}
}

func TestNormalizeFTPFiles_PreservesExplicitPermissions(t *testing.T) {
	files := normalizeFTPFiles([]config.FTPFile{{ID: "f1", Path: "/f.txt", Permissions: "-rwxr-xr-x"}})
	if files[0].Permissions != "-rwxr-xr-x" {
		t.Errorf("explicit permissions should be preserved, got %q", files[0].Permissions)
	}
}

func TestNormalizeFTPFiles_AddsMissingLeadingSlash(t *testing.T) {
	files := normalizeFTPFiles([]config.FTPFile{{ID: "f1", Path: "f.txt"}})
	if !strings.HasPrefix(files[0].Path, "/") {
		t.Errorf("path should start with '/', got %q", files[0].Path)
	}
}

func TestNormalizeFTPFiles_PreservesAbsolutePath(t *testing.T) {
	files := normalizeFTPFiles([]config.FTPFile{{ID: "f1", Path: "/data/f.txt"}})
	if files[0].Path != "/data/f.txt" {
		t.Errorf("absolute path should be unchanged, got %q", files[0].Path)
	}
}

// ---------------------------------------------------------------------------
// ftpAbsPath
// ---------------------------------------------------------------------------

func TestFtpAbsPath_EmptyArg(t *testing.T) {
	if got := ftpAbsPath("/cwd", ""); got != "/cwd" {
		t.Errorf("empty arg should return cwd, got %q", got)
	}
}

func TestFtpAbsPath_AbsoluteArg(t *testing.T) {
	if got := ftpAbsPath("/cwd", "/other"); got != "/other" {
		t.Errorf("absolute arg should override cwd, got %q", got)
	}
}

func TestFtpAbsPath_RelativeArg(t *testing.T) {
	if got := ftpAbsPath("/reports", "data.csv"); got != "/reports/data.csv" {
		t.Errorf("relative arg should be joined, got %q", got)
	}
}

func TestFtpAbsPath_DotDot(t *testing.T) {
	if got := ftpAbsPath("/a/b", ".."); got != "/a" {
		t.Errorf("'..' should navigate up, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// listing
// ---------------------------------------------------------------------------

func TestFTP_Listing_ContainsFilesInDir(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/data/file1.txt", Content: "hello"},
		{ID: "f2", Path: "/other/file2.txt", Content: "world"},
	})
	listing := srv.listing("/data", false)
	if !strings.Contains(listing, "file1.txt") {
		t.Errorf("listing should contain file1.txt, got: %q", listing)
	}
	if strings.Contains(listing, "file2.txt") {
		t.Errorf("listing should not contain file2.txt from different dir, got: %q", listing)
	}
}

func TestFTP_Listing_NamesOnly(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/data/file1.txt", Content: "hello"},
	})
	listing := srv.listing("/data", true)
	if !strings.Contains(listing, "file1.txt") {
		t.Errorf("names-only listing should contain filename, got: %q", listing)
	}
	// Names-only should not contain permissions prefix.
	if strings.Contains(listing, "-rw-") {
		t.Errorf("names-only listing should not contain permissions, got: %q", listing)
	}
}

func TestFTP_Listing_EmptyTarget(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/root.txt", Content: "root"},
	})
	listing := srv.listing("", false)
	if !strings.Contains(listing, "root.txt") {
		t.Errorf("empty target should default to root, got: %q", listing)
	}
}

// ---------------------------------------------------------------------------
// findFile / deleteFile
// ---------------------------------------------------------------------------

func TestFTP_FindFile_Found(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/data.csv", Content: "a,b,c"},
	})
	f, ok := srv.findFile("/data.csv")
	if !ok {
		t.Fatal("expected to find file")
	}
	if f.Content != "a,b,c" {
		t.Errorf("unexpected content: %q", f.Content)
	}
}

func TestFTP_FindFile_NotFound(t *testing.T) {
	srv := newTestFTPServer(t, nil)
	_, ok := srv.findFile("/nonexistent.txt")
	if ok {
		t.Fatal("should not find nonexistent file")
	}
}

func TestFTP_DeleteFile(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/a.txt", Content: "a"},
		{ID: "f2", Path: "/b.txt", Content: "b"},
	})
	srv.deleteFile("/a.txt")
	files := srv.GetFiles()
	if len(files) != 1 || files[0].Path != "/b.txt" {
		t.Fatalf("expected only b.txt after delete, got: %+v", files)
	}
}

func TestFTP_DeleteFile_NonExistent(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/a.txt"},
	})
	srv.deleteFile("/nonexistent.txt")
	if len(srv.GetFiles()) != 1 {
		t.Fatal("deleting non-existent file should leave others intact")
	}
}

// ---------------------------------------------------------------------------
// StatusInfo
// ---------------------------------------------------------------------------

func TestFTP_StatusInfo(t *testing.T) {
	srv := newTestFTPServer(t, []config.FTPFile{{ID: "f1"}, {ID: "f2"}})
	info := srv.StatusInfo()
	if info["protocol"] != "ftp" {
		t.Errorf("unexpected protocol %v", info["protocol"])
	}
	if info["files"] != 2 {
		t.Errorf("want files=2, got %v", info["files"])
	}
}

// ---------------------------------------------------------------------------
// Integration: FTP server over TCP
// ---------------------------------------------------------------------------

func startFTPServer(t *testing.T, files []config.FTPFile) (addr string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	time.Sleep(10 * time.Millisecond)

	cfg := &config.FTPConfig{Enabled: true, Port: port, Files: files}
	srv := New(cfg, scenarios.New(nil), logger.New(100))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(100 * time.Millisecond)
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func ftpConn(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial FTP: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	// Read the 220 banner.
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "220") {
		t.Fatalf("unexpected FTP banner: %q err=%v", line, err)
	}
	return conn, r
}

func ftpCmd(t *testing.T, conn net.Conn, r *bufio.Reader, cmd string) string {
	t.Helper()
	_, _ = fmt.Fprintf(conn, "%s\r\n", cmd)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("ftpCmd %q: read error: %v", cmd, err)
	}
	return strings.TrimRight(line, "\r\n")
}

func TestFTPServer_BasicCommands(t *testing.T) {
	addr := startFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/data.txt", Content: "hello"},
	})
	conn, r := ftpConn(t, addr)

	resp := ftpCmd(t, conn, r, "USER anonymous")
	if !strings.HasPrefix(resp, "331") {
		t.Errorf("USER: want 331, got %q", resp)
	}

	resp = ftpCmd(t, conn, r, "PASS x")
	if !strings.HasPrefix(resp, "230") {
		t.Errorf("PASS: want 230, got %q", resp)
	}

	resp = ftpCmd(t, conn, r, "SYST")
	if !strings.HasPrefix(resp, "215") {
		t.Errorf("SYST: want 215, got %q", resp)
	}

	resp = ftpCmd(t, conn, r, "FEAT")
	// FEAT is multi-line; consume until "211 End".
	for !strings.HasPrefix(resp, "211 End") {
		resp, _ = r.ReadString('\n')
		resp = strings.TrimRight(resp, "\r\n")
	}

	resp = ftpCmd(t, conn, r, "PWD")
	if !strings.HasPrefix(resp, "257") {
		t.Errorf("PWD: want 257, got %q", resp)
	}

	resp = ftpCmd(t, conn, r, "TYPE I")
	if !strings.HasPrefix(resp, "200") {
		t.Errorf("TYPE: want 200, got %q", resp)
	}

	resp = ftpCmd(t, conn, r, "CWD /data")
	if !strings.HasPrefix(resp, "250") {
		t.Errorf("CWD: want 250, got %q", resp)
	}

	resp = ftpCmd(t, conn, r, "UNKNOWN_CMD")
	if !strings.HasPrefix(resp, "502") {
		t.Errorf("unknown cmd: want 502, got %q", resp)
	}

	resp = ftpCmd(t, conn, r, "QUIT")
	if !strings.HasPrefix(resp, "221") {
		t.Errorf("QUIT: want 221, got %q", resp)
	}
}

func TestFTPServer_PASV_and_LIST(t *testing.T) {
	addr := startFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/readme.txt", Content: "doc"},
	})
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")

	// PASV
	pasvResp := ftpCmd(t, conn, r, "PASV")
	if !strings.HasPrefix(pasvResp, "227") {
		t.Fatalf("PASV: want 227, got %q", pasvResp)
	}
	dataAddr := parsePASVAddr(t, pasvResp)

	// LIST
	_, _ = fmt.Fprintf(conn, "LIST\r\n")

	dataConn, err := net.DialTimeout("tcp", dataAddr, time.Second)
	if err != nil {
		t.Fatalf("dial data conn: %v", err)
	}
	defer dataConn.Close() //nolint:errcheck

	controlResp, _ := r.ReadString('\n')
	if !strings.HasPrefix(controlResp, "125") {
		t.Fatalf("LIST: want 125, got %q", controlResp)
	}
	var listData strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := dataConn.Read(buf)
		if n > 0 {
			listData.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	finalResp, _ := r.ReadString('\n')
	if !strings.HasPrefix(strings.TrimRight(finalResp, "\r\n"), "226") {
		t.Errorf("LIST complete: want 226, got %q", finalResp)
	}
	if !strings.Contains(listData.String(), "readme.txt") {
		t.Errorf("LIST data should contain readme.txt, got: %q", listData.String())
	}
}

func TestFTPServer_RETR(t *testing.T) {
	addr := startFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/data.txt", Content: "file-content"},
	})
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")

	pasvResp := ftpCmd(t, conn, r, "PASV")
	dataAddr := parsePASVAddr(t, pasvResp)

	_, _ = fmt.Fprintf(conn, "RETR data.txt\r\n")
	dataConn, err := net.DialTimeout("tcp", dataAddr, time.Second)
	if err != nil {
		t.Fatalf("dial data conn: %v", err)
	}
	defer dataConn.Close() //nolint:errcheck

	controlResp, _ := r.ReadString('\n')
	if !strings.HasPrefix(controlResp, "150") {
		t.Fatalf("RETR: want 150, got %q", controlResp)
	}
	var content strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := dataConn.Read(buf)
		if n > 0 {
			content.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	if content.String() != "file-content" {
		t.Errorf("RETR content want 'file-content', got %q", content.String())
	}
}

func TestFTPServer_RETR_NoPASV(t *testing.T) {
	addr := startFTPServer(t, []config.FTPFile{{ID: "f1", Path: "/data.txt"}})
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")
	resp := ftpCmd(t, conn, r, "RETR data.txt")
	if !strings.HasPrefix(resp, "425") {
		t.Errorf("RETR without PASV: want 425, got %q", resp)
	}
}

func TestFTPServer_RETR_NotFound(t *testing.T) {
	addr := startFTPServer(t, nil)
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")
	ftpCmd(t, conn, r, "PASV")
	resp := ftpCmd(t, conn, r, "RETR nonexistent.txt")
	if !strings.HasPrefix(resp, "550") {
		t.Errorf("RETR not found: want 550, got %q", resp)
	}
}

func TestFTPServer_SIZE(t *testing.T) {
	addr := startFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/data.txt", Content: "hello"},
	})
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")
	resp := ftpCmd(t, conn, r, "SIZE data.txt")
	if !strings.HasPrefix(resp, "213") {
		t.Errorf("SIZE: want 213, got %q", resp)
	}
	if !strings.Contains(resp, "5") {
		t.Errorf("SIZE response should contain content length 5, got %q", resp)
	}
}

func TestFTPServer_SIZE_NotFound(t *testing.T) {
	addr := startFTPServer(t, nil)
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")
	resp := ftpCmd(t, conn, r, "SIZE nonexistent.txt")
	if !strings.HasPrefix(resp, "550") {
		t.Errorf("SIZE not found: want 550, got %q", resp)
	}
}

func TestFTPServer_DELE(t *testing.T) {
	addr := startFTPServer(t, []config.FTPFile{
		{ID: "f1", Path: "/todelete.txt", Content: "bye"},
	})
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")
	resp := ftpCmd(t, conn, r, "DELE todelete.txt")
	if !strings.HasPrefix(resp, "250") {
		t.Errorf("DELE: want 250, got %q", resp)
	}
}

func TestFTPServer_LIST_NoPASV(t *testing.T) {
	addr := startFTPServer(t, nil)
	conn, r := ftpConn(t, addr)
	ftpCmd(t, conn, r, "USER anon")
	ftpCmd(t, conn, r, "PASS x")
	resp := ftpCmd(t, conn, r, "LIST")
	if !strings.HasPrefix(resp, "425") {
		t.Errorf("LIST without PASV: want 425, got %q", resp)
	}
}

// parsePASVAddr extracts the host:port from a "227 Entering Passive Mode (h1,h2,h3,h4,p1,p2)\r\n".
func parsePASVAddr(t *testing.T, resp string) string {
	t.Helper()
	start := strings.Index(resp, "(")
	end := strings.Index(resp, ")")
	if start < 0 || end < 0 {
		t.Fatalf("invalid PASV response: %q", resp)
	}
	parts := strings.Split(resp[start+1:end], ",")
	if len(parts) != 6 {
		t.Fatalf("invalid PASV parts: %v", parts)
	}
	var p1, p2 int
	fmt.Sscanf(parts[4], "%d", &p1)
	fmt.Sscanf(parts[5], "%d", &p2)
	port := p1*256 + p2
	return fmt.Sprintf("127.0.0.1:%d", port)
}
