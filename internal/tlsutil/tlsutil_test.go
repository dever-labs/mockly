package tlsutil_test

import (
	"crypto/tls"
	"net"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/testutil"
	"github.com/dever-labs/mockly/internal/tlsutil"
)

func TestWrapListener_NilConfig(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck

	got, err := tlsutil.WrapListener(ln, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ln {
		t.Error("nil config should return the original listener unchanged")
	}
}

func TestWrapListener_Disabled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck

	cfg := &config.TLSConfig{Enabled: false}
	got, err := tlsutil.WrapListener(ln, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ln {
		t.Error("disabled TLS config should return the original listener unchanged")
	}
}

func TestWrapListener_ValidCert(t *testing.T) {
	dir := t.TempDir()
	certFile := dir + "/cert.pem"
	keyFile := dir + "/key.pem"
	if err := testutil.WriteSelfSignedCert(certFile, keyFile); err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	cfg := &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile}
	wrapped, err := tlsutil.WrapListener(ln, cfg)
	if err != nil {
		t.Fatalf("WrapListener: %v", err)
	}
	defer wrapped.Close() //nolint:errcheck

	// Accept a TLS connection and complete the handshake in the background.
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := wrapped.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		// Trigger the server-side TLS handshake by reading.
		buf := make([]byte, 1)
		_, _ = conn.Read(buf)
		conn.Close() //nolint:errcheck
		acceptErr <- nil
	}()

	// Connect as a TLS client using InsecureSkipVerify (self-signed cert).
	tlsConn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	if err != nil {
		t.Fatalf("TLS dial: %v", err)
	}

	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close() //nolint:errcheck
		t.Fatalf("TLS handshake: %v", err)
	}
	// Close the client so the server goroutine's Read returns EOF.
	tlsConn.Close() //nolint:errcheck

	if err := <-acceptErr; err != nil {
		t.Fatalf("server accept error: %v", err)
	}
}

func TestWrapListener_MissingCert(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck

	cfg := &config.TLSConfig{
		Enabled:  true,
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}
	_, err = tlsutil.WrapListener(ln, cfg)
	if err == nil {
		t.Error("expected an error for missing cert/key files, got nil")
	}
}
