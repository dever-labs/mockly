package mocklydriver

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// getFreePort returns a free TCP port on 127.0.0.1.
// Always call sequentially to avoid TOCTOU races.
func getFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return 0, fmt.Errorf("listener address is not a *net.TCPAddr: %T", ln.Addr())
	}
	port := tcpAddr.Port
	if err := ln.Close(); err != nil {
		// Close error is non-fatal: the port has already been obtained.
		_ = err
	}
	return port, nil
}

func sleep(d time.Duration) {
	time.Sleep(d)
}

var portConflictRe = regexp.MustCompile(`(?i)address already in use|EADDRINUSE`)

func isPortConflict(msg string) bool {
	return portConflictRe.MatchString(msg)
}

// yamlStr returns a single-quoted YAML string, escaping single quotes by doubling them.
func yamlStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
