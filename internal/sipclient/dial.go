package sipclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/coder/websocket"
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

// DialOptions groups everything Dial needs.
type DialOptions struct {
	Target  *url.URL
	Origin  string // may be empty → defaults to Target scheme+host
	Headers http.Header
	TLS     *tls.Config
	Timeout time.Duration
}

// Dial opens a framed connection to opts.Target, dispatching by scheme.
// Currently only ws/wss are supported; a future commit adds https/WT.
func Dial(ctx context.Context, opts DialOptions) (FrameConn, error) {
	switch opts.Target.Scheme {
	case "ws", "wss":
		return dialWS(ctx, opts)
	default:
		return nil, fmt.Errorf("%w: unsupported scheme %q (want ws or wss)", ErrConnect, opts.Target.Scheme)
	}
}

func dialWS(ctx context.Context, opts DialOptions) (*wsFrameConn, error) {
	headers := opts.Headers.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	origin := opts.Origin
	if origin == "" {
		httpScheme := "http"
		if opts.Target.Scheme == "wss" {
			httpScheme = "https"
		}
		origin = httpScheme + "://" + opts.Target.Host
	}
	headers.Set("Origin", origin)

	httpClient := &http.Client{}
	if opts.Target.Scheme == "wss" {
		httpClient.Transport = &http.Transport{TLSClientConfig: opts.TLS}
	}

	dialCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	conn, _, err := websocket.Dial(dialCtx, opts.Target.String(), &websocket.DialOptions{
		HTTPHeader: headers,
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", opts.Target, err)
	}
	return newWSFrameConn(conn), nil
}
