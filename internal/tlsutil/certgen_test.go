package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
)

func TestGenerateSelfSigned_Defaults(t *testing.T) {
	certPEM, keyPEM, err := generateSelfSigned(nil)
	if err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		t.Fatal("expected non-empty cert/key PEM")
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	if cert.Subject.CommonName != "localhost" {
		t.Errorf("CommonName = %q, want %q", cert.Subject.CommonName, "localhost")
	}
	if got := cert.DNSNames; len(got) != 1 || got[0] != "localhost" {
		t.Errorf("DNSNames = %v, want [localhost]", got)
	}
	wantIPs := map[string]bool{"127.0.0.1": true, "::1": true}
	if len(cert.IPAddresses) != 2 {
		t.Errorf("IPAddresses = %v, want 2 entries", cert.IPAddresses)
	}
	for _, ip := range cert.IPAddresses {
		if !wantIPs[ip.String()] {
			t.Errorf("unexpected IP SAN %v", ip)
		}
	}
	gotValidFor := cert.NotAfter.Sub(cert.NotBefore)
	if gotValidFor < defaultValidFor || gotValidFor > defaultValidFor+10*time.Minute {
		t.Errorf("validity period = %v, want ~%v", gotValidFor, defaultValidFor)
	}
	if len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Errorf("ExtKeyUsage = %v, want [ServerAuth]", cert.ExtKeyUsage)
	}
}

func TestGenerateSelfSigned_CustomDetails(t *testing.T) {
	gen := &config.TLSGenerate{
		CommonName:   "mocks.example.com",
		Organization: "Mockly Test Org",
		DNSNames:     []string{"mocks.example.com", "api.example.com"},
		IPAddresses:  []string{"10.0.0.5"},
		URIs:         []string{"spiffe://example.org/ns/default/sa/mockly"},
		ValidFor:     config.Duration{Duration: 2 * time.Hour},
		ClientAuth:   true,
	}
	certPEM, _, err := generateSelfSigned(gen)
	if err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	if cert.Subject.CommonName != "mocks.example.com" {
		t.Errorf("CommonName = %q, want %q", cert.Subject.CommonName, "mocks.example.com")
	}
	if len(cert.Subject.Organization) != 1 || cert.Subject.Organization[0] != "Mockly Test Org" {
		t.Errorf("Organization = %v, want [Mockly Test Org]", cert.Subject.Organization)
	}
	if len(cert.DNSNames) != 2 {
		t.Errorf("DNSNames = %v, want 2 entries", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 1 || cert.IPAddresses[0].String() != "10.0.0.5" {
		t.Errorf("IPAddresses = %v, want [10.0.0.5]", cert.IPAddresses)
	}
	if len(cert.URIs) != 1 || cert.URIs[0].String() != "spiffe://example.org/ns/default/sa/mockly" {
		t.Errorf("URIs = %v, want [spiffe://example.org/ns/default/sa/mockly]", cert.URIs)
	}
	gotValidFor := cert.NotAfter.Sub(cert.NotBefore)
	if gotValidFor < 2*time.Hour || gotValidFor > 2*time.Hour+10*time.Minute {
		t.Errorf("validity period = %v, want ~2h", gotValidFor)
	}
	foundClientAuth := false
	foundServerAuth := false
	for _, u := range cert.ExtKeyUsage {
		if u == x509.ExtKeyUsageClientAuth {
			foundClientAuth = true
		}
		if u == x509.ExtKeyUsageServerAuth {
			foundServerAuth = true
		}
	}
	if !foundClientAuth || !foundServerAuth {
		t.Errorf("ExtKeyUsage = %v, want both ServerAuth and ClientAuth", cert.ExtKeyUsage)
	}
}

func TestGenerateSelfSigned_InvalidIP(t *testing.T) {
	gen := &config.TLSGenerate{IPAddresses: []string{"not-an-ip"}}
	if _, _, err := generateSelfSigned(gen); err == nil {
		t.Fatal("expected an error for invalid IP address, got nil")
	}
}

func TestGenerateSelfSigned_InvalidURI(t *testing.T) {
	gen := &config.TLSGenerate{URIs: []string{"://not-a-uri"}}
	if _, _, err := generateSelfSigned(gen); err == nil {
		t.Fatal("expected an error for invalid URI, got nil")
	}
}

func TestResolveCertificate_InMemoryWhenNoPaths(t *testing.T) {
	cfg := &config.TLSConfig{Enabled: true}
	cert, err := resolveCertificate(cfg)
	if err != nil {
		t.Fatalf("resolveCertificate: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("expected a non-empty certificate chain")
	}
}

func TestResolveCertificate_GeneratesAndPersistsWhenPathsMissing(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	cfg := &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile}
	if _, err := resolveCertificate(cfg); err != nil {
		t.Fatalf("resolveCertificate: %v", err)
	}

	if !fileExists(certFile) {
		t.Error("expected generated cert file to be written to disk")
	}
	if !fileExists(keyFile) {
		t.Error("expected generated key file to be written to disk")
	}
}

func TestResolveCertificate_ReusesExistingFilesInsteadOfRegenerating(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	cfg := &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile}
	if _, err := resolveCertificate(cfg); err != nil {
		t.Fatalf("first resolveCertificate: %v", err)
	}
	firstCertBytes, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("read generated cert: %v", err)
	}

	// A second call with the same cfg must reuse the on-disk cert rather
	// than silently regenerating a new one (which would invalidate
	// anything that pinned/trusted the first cert).
	if _, err := resolveCertificate(cfg); err != nil {
		t.Fatalf("second resolveCertificate: %v", err)
	}
	secondCertBytes, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("read cert after second call: %v", err)
	}

	if string(firstCertBytes) != string(secondCertBytes) {
		t.Error("expected the second resolveCertificate call to reuse the existing cert, but it regenerated one")
	}
}

func TestResolveCertificate_WriteFailureReturnsError(t *testing.T) {
	cfg := &config.TLSConfig{
		Enabled:  true,
		CertFile: "/nonexistent-dir/cert.pem",
		KeyFile:  "/nonexistent-dir/key.pem",
	}
	if _, err := resolveCertificate(cfg); err == nil {
		t.Fatal("expected an error when the generated cert/key can't be written, got nil")
	}
}

// TestResolveCertificate_ConcurrentCallsShareSameConsistentCert verifies
// that multiple goroutines calling resolveCertificate concurrently for the
// same not-yet-existing cert_file/key_file pair (e.g. two listeners/
// protocols intentionally configured to share one generated certificate,
// started concurrently by cmd/mockly/start.go) don't race: every call must
// end up with a valid, self-consistent certificate, and the files on disk
// must themselves parse as a matching cert/key pair afterward.
func TestResolveCertificate_ConcurrentCallsShareSameConsistentCert(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "shared-cert.pem")
	keyFile := filepath.Join(dir, "shared-key.pem")
	cfg := &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile}

	const goroutines = 10
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := resolveCertificate(cfg); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("resolveCertificate: %v", err)
	}

	// The persisted files must themselves form a valid, matching keypair -
	// if two goroutines had raced and written mismatched cert/key halves,
	// this would fail.
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		t.Fatalf("persisted cert/key files are not a valid matching pair: %v", err)
	}
}
