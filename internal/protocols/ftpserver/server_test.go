package ftpserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestFTPAbsPath(t *testing.T) {
	if got := ftpAbsPath("/reports", "data.csv"); got != "/reports/data.csv" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.FTPConfig{Enabled: true, Port: 2121, Files: []config.FTPFile{{ID: "1", Path: "/a.txt", Content: "x"}}}, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "ftp" || info["port"] != 2121 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
