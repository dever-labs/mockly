// Package tlsutil provides helpers for optionally wrapping net.Listeners with TLS.
package tlsutil

import (
	"crypto/tls"
	"net"
	"os"

	"github.com/dever-labs/mockly/internal/config"
)

// WrapListener wraps ln with TLS when cfg is non-nil and enabled.
// Returns the original listener unchanged when TLS is not configured.
//
// When cfg.CertFile/cfg.KeyFile are both set and both exist on disk, they're
// loaded as-is. Otherwise a self-signed certificate is generated
// automatically (see config.TLSGenerate for the details that can be
// configured via cfg.Generate); if CertFile/KeyFile are set, the generated
// cert/key are written there for reuse on subsequent starts.
func WrapListener(ln net.Listener, cfg *config.TLSConfig) (net.Listener, error) {
	if cfg == nil || !cfg.Enabled {
		return ln, nil
	}
	cert, err := resolveCertificate(cfg)
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	return tls.NewListener(ln, tlsCfg), nil
}

// fileExists reports whether path exists and is a regular file (or at least
// not a directory); it treats any stat error (including "not found") as
// "doesn't exist".
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// writeFile writes data to path with owner-only permissions, since it may
// contain a private key.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
