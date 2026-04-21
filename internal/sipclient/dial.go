package sipclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildTLSConfig returns a *tls.Config suitable for the coder/websocket Dial
// options. It is always non-nil so wss:// connections have a config to
// override the default. System roots are used unless caFile is provided.
func BuildTLSConfig(skipVerify bool, caFile string) (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: skipVerify, //nolint:gosec // opt-in via --insecure-skip-verify
		MinVersion:         tls.VersionTLS12,
	}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read ca-file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca-file %q contains no valid PEM certificates", caFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}
