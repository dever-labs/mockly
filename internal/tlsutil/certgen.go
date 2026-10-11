package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
)

// defaultValidFor is how long a generated certificate is valid for when
// TLSGenerate.ValidFor is unset.
const defaultValidFor = 365 * 24 * time.Hour

// generateSelfSigned creates a self-signed ECDSA (P-256) certificate and
// private key, PEM-encoded, honoring the optional details in gen. gen may
// be nil, in which case all defaults apply (CommonName "localhost", DNS SAN
// "localhost", IP SANs 127.0.0.1/::1, 1 year validity).
func generateSelfSigned(gen *config.TLSGenerate) (certPEM, keyPEM []byte, err error) {
	commonName := "localhost"
	var organization []string
	dnsNames := []string{"localhost"}
	ipAddresses := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	var uris []*url.URL
	validFor := defaultValidFor
	clientAuth := false

	if gen != nil {
		if gen.CommonName != "" {
			commonName = gen.CommonName
		}
		if gen.Organization != "" {
			organization = []string{gen.Organization}
		}
		if len(gen.DNSNames) > 0 {
			dnsNames = gen.DNSNames
		}
		if len(gen.IPAddresses) > 0 {
			ipAddresses = make([]net.IP, 0, len(gen.IPAddresses))
			for _, s := range gen.IPAddresses {
				ip := net.ParseIP(s)
				if ip == nil {
					return nil, nil, fmt.Errorf("tls.generate.ip_addresses: invalid IP address %q", s)
				}
				ipAddresses = append(ipAddresses, ip)
			}
		}
		for _, s := range gen.URIs {
			u, perr := url.Parse(s)
			if perr != nil {
				return nil, nil, fmt.Errorf("tls.generate.uris: invalid URI %q: %w", s, perr)
			}
			uris = append(uris, u)
		}
		if gen.ValidFor.Duration > 0 {
			validFor = gen.ValidFor.Duration
		}
		clientAuth = gen.ClientAuth
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial number: %w", err)
	}

	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	if clientAuth {
		extKeyUsage = append(extKeyUsage, x509.ExtKeyUsageClientAuth)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(validFor),
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
		URIs:                  uris,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

// certGenMu serializes certificate resolution (the check-exists /
// generate / write sequence below). Protocols are started concurrently
// (see cmd/mockly/start.go), so two listeners/protocols configured to
// share the same not-yet-existing cert_file/key_file pair could otherwise
// race: both would see the files missing, generate different keypairs, and
// write concurrently, leaving a mismatched cert/key pair on disk. This is
// an uncontended, startup-only lock, so a single global mutex is simplest
// and has no meaningful runtime cost.
var certGenMu sync.Mutex

// resolveCertificate returns the tls.Certificate to serve for cfg. It loads
// CertFile/KeyFile from disk when both are set and both files exist.
// Otherwise it generates a self-signed certificate: if CertFile/KeyFile are
// set (but missing), the generated PEM data is written there so subsequent
// starts reuse the same certificate; if both are empty, the certificate is
// kept in-memory only and regenerated fresh on every start.
func resolveCertificate(cfg *config.TLSConfig) (tls.Certificate, error) {
	certGenMu.Lock()
	defer certGenMu.Unlock()

	if cfg.CertFile != "" && cfg.KeyFile != "" && fileExists(cfg.CertFile) && fileExists(cfg.KeyFile) {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("load TLS cert/key: %w", err)
		}
		return cert, nil
	}

	certPEM, keyPEM, err := generateSelfSigned(cfg.Generate)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate self-signed TLS certificate: %w", err)
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		if err := writeFile(cfg.CertFile, certPEM); err != nil {
			return tls.Certificate{}, fmt.Errorf("write generated cert %q: %w", cfg.CertFile, err)
		}
		if err := writeFile(cfg.KeyFile, keyPEM); err != nil {
			return tls.Certificate{}, fmt.Errorf("write generated key %q: %w", cfg.KeyFile, err)
		}
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse generated TLS cert/key: %w", err)
	}
	return cert, nil
}
