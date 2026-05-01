// Package tlsutil provides helpers for optionally wrapping net.Listeners with TLS.
package tlsutil

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/dever-labs/mockly/internal/config"
)

// WrapListener wraps ln with TLS when cfg is non-nil and enabled.
// Returns the original listener unchanged when TLS is not configured.
func WrapListener(ln net.Listener, cfg *config.TLSConfig) (net.Listener, error) {
	if cfg == nil || !cfg.Enabled {
		return ln, nil
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS cert/key: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	return tls.NewListener(ln, tlsCfg), nil
}
