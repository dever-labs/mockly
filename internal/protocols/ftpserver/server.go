package ftpserver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
)

type Server struct {
	cfg       *config.FTPConfig
	scenarios *scenarios.Store
	log       *logger.Logger

	mu       sync.RWMutex
	files    []config.FTPFile
	listener net.Listener
}

func New(cfg *config.FTPConfig, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, scenarios: sc, log: log, files: normalizeFTPFiles(cfg.Files)}
}

func (s *Server) SetFiles(files []config.FTPFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = normalizeFTPFiles(files)
}

func (s *Server) GetFiles() []config.FTPFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.FTPFile(nil), s.files...)
}

func normalizeFTPFiles(files []config.FTPFile) []config.FTPFile {
	out := append([]config.FTPFile(nil), files...)
	for i := range out {
		if out[i].Size == 0 {
			out[i].Size = int64(len(out[i].Content))
		}
		if out[i].Permissions == "" {
			out[i].Permissions = "-rw-r--r--"
		}
		if !strings.HasPrefix(out[i].Path, "/") {
			out[i].Path = "/" + out[i].Path
		}
	}
	return out
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("ftp server listen :%d: %w", s.cfg.Port, err)
	}
	s.listener = ln
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close() //nolint:errcheck
	reader := bufio.NewReader(conn)
	cwd := "/"
	var passive net.Listener
	defer func() {
		if passive != nil {
			_ = passive.Close()
		}
	}()
	_, _ = conn.Write([]byte("220 Mockly FTP Server ready\r\n"))
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := strings.ToUpper(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = strings.Join(parts[1:], " ")
		}
		switch cmd {
		case "USER":
			_, _ = conn.Write([]byte("331 Password required\r\n"))
		case "PASS":
			_, _ = conn.Write([]byte("230 User logged in\r\n"))
		case "SYST":
			_, _ = conn.Write([]byte("215 UNIX Type: L8\r\n"))
		case "FEAT":
			_, _ = conn.Write([]byte("211 Features:\r\n PASV\r\n211 End\r\n"))
		case "PWD":
			_, _ = fmt.Fprintf(conn, "257 %q is current directory\r\n", cwd)
		case "CWD":
			cwd = ftpAbsPath(cwd, arg)
			_, _ = conn.Write([]byte("250 Directory changed\r\n"))
		case "TYPE":
			_, _ = conn.Write([]byte("200 Type set\r\n"))
		case "PASV":
			if passive != nil {
				_ = passive.Close()
			}
			passive, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				_, _ = conn.Write([]byte("425 Can't open data connection\r\n"))
				continue
			}
			port := passive.Addr().(*net.TCPAddr).Port
			p1, p2 := port/256, port%256
			_, _ = fmt.Fprintf(conn, "227 Entering Passive Mode (127,0,0,1,%d,%d)\r\n", p1, p2)
		case "LIST", "NLST":
			if s.maybeInjectFault(conn) {
				return
			}
			if passive == nil {
				_, _ = conn.Write([]byte("425 Use PASV first\r\n"))
				continue
			}
			_, _ = conn.Write([]byte("125 Data connection open\r\n"))
			dataConn, err := passive.Accept()
			_ = passive.Close()
			passive = nil
			if err != nil {
				_, _ = conn.Write([]byte("425 Can't open data connection\r\n"))
				continue
			}
			if cmd == "LIST" {
				_, _ = io.WriteString(dataConn, s.listing(ftpAbsPath(cwd, arg), false))
			} else {
				_, _ = io.WriteString(dataConn, s.listing(ftpAbsPath(cwd, arg), true))
			}
			_ = dataConn.Close()
			_, _ = conn.Write([]byte("226 Transfer complete\r\n"))
		case "RETR":
			if s.maybeInjectFault(conn) {
				return
			}
			if passive == nil {
				_, _ = conn.Write([]byte("425 Use PASV first\r\n"))
				continue
			}
			file, ok := s.findFile(ftpAbsPath(cwd, arg))
			if !ok {
				_, _ = conn.Write([]byte("550 No such file\r\n"))
				continue
			}
			_, _ = conn.Write([]byte("150 Opening data connection\r\n"))
			dataConn, err := passive.Accept()
			_ = passive.Close()
			passive = nil
			if err != nil {
				_, _ = conn.Write([]byte("425 Can't open data connection\r\n"))
				continue
			}
			_, _ = io.WriteString(dataConn, file.Content)
			_ = dataConn.Close()
			_, _ = conn.Write([]byte("226 Transfer complete\r\n"))
		case "STOR":
			if s.maybeInjectFault(conn) {
				return
			}
			if passive == nil {
				_, _ = conn.Write([]byte("425 Use PASV first\r\n"))
				continue
			}
			_, _ = conn.Write([]byte("150 Opening data connection\r\n"))
			dataConn, err := passive.Accept()
			_ = passive.Close()
			passive = nil
			if err == nil {
				_, _ = io.Copy(io.Discard, dataConn)
				_ = dataConn.Close()
			}
			_, _ = conn.Write([]byte("226 Transfer complete\r\n"))
		case "DELE":
			s.deleteFile(ftpAbsPath(cwd, arg))
			_, _ = conn.Write([]byte("250 Deleted\r\n"))
		case "SIZE":
			file, ok := s.findFile(ftpAbsPath(cwd, arg))
			if !ok {
				_, _ = conn.Write([]byte("550 Not found\r\n"))
				continue
			}
			_, _ = fmt.Fprintf(conn, "213 %d\r\n", file.Size)
		case "QUIT":
			_, _ = conn.Write([]byte("221 Goodbye\r\n"))
			return
		default:
			_, _ = conn.Write([]byte("502 Command not implemented\r\n"))
		}
	}
}

func (s *Server) maybeInjectFault(conn net.Conn) bool {
	fault := s.scenarios.EffectiveFTPFault()
	if fault == nil {
		return false
	}
	if fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}
	if s.scenarios.RollFault(fault.ErrorRate) {
		code := fault.Code
		if code == 0 {
			code = 421
		}
		msg := fault.Message
		if msg == "" {
			msg = "Service not available"
		}
		_, _ = conn.Write([]byte(fmt.Sprintf("%d %s\r\n", code, msg)))
		return true
	}
	return false
}

func ftpAbsPath(cwd, p string) string {
	if p == "" {
		return cwd
	}
	if strings.HasPrefix(p, "/") {
		return path.Clean(p)
	}
	return path.Clean(path.Join(cwd, p))
}

func (s *Server) listing(target string, namesOnly bool) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if target == "" {
		target = "/"
	}
	dir := target
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	var b strings.Builder
	for _, file := range s.files {
		fileDir := path.Dir(file.Path)
		if !strings.HasSuffix(fileDir, "/") {
			fileDir += "/"
		}
		if fileDir != dir {
			continue
		}
		name := path.Base(file.Path)
		if namesOnly {
			fmt.Fprintf(&b, "%s\r\n", name)
		} else {
			fmt.Fprintf(&b, "%s 1 ftp ftp %d Jan 01 00:00 %s\r\n", file.Permissions, file.Size, name)
		}
	}
	return b.String()
}

func (s *Server) findFile(filePath string) (config.FTPFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, file := range s.files {
		if file.Path == filePath {
			return file, true
		}
	}
	return config.FTPFile{}, false
}

func (s *Server) deleteFile(filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.files[:0]
	for _, file := range s.files {
		if file.Path != filePath {
			filtered = append(filtered, file)
		}
	}
	s.files = append([]config.FTPFile(nil), filtered...)
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "ftp", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "files": len(s.files)}
}
