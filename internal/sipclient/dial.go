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
	"github.com/quic-go/webtransport-go"
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
func Dial(ctx context.Context, opts DialOptions) (FrameConn, error) {
	switch opts.Target.Scheme {
	case "ws", "wss":
		return dialWS(ctx, opts)
	case "https":
		return dialWT(ctx, opts)
	default:
		return nil, fmt.Errorf("%w: unsupported scheme %q (want ws, wss, or https)", ErrConnect, opts.Target.Scheme)
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

func dialWT(ctx context.Context, opts DialOptions) (*wtFrameConn, error) {
	headers := opts.Headers.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	// Origin handling: WT expects a same-origin hint via the Origin header
	// for some servers, but booba's server honors the --origin check; we
	// set the same computed Origin as the WS path for consistency.
	origin := opts.Origin
	if origin == "" {
		origin = "https://" + opts.Target.Host
	}
	headers.Set("Origin", origin)

	tlsCfg := opts.TLS
	if tlsCfg == nil {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		tlsCfg = tlsCfg.Clone()
	}
	// webtransport-go requires the h3 ALPN; the library does not set it
	// automatically.
	if len(tlsCfg.NextProtos) == 0 {
		tlsCfg.NextProtos = []string{"h3"}
	}

	dialer := webtransport.Dialer{
		TLSClientConfig: tlsCfg,
	}

	dialCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	_, session, err := dialer.Dial(dialCtx, opts.Target.String(), headers)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", opts.Target, err)
	}
	stream, err := session.OpenStreamSync(dialCtx)
	if err != nil {
		_ = session.CloseWithError(1, "open stream failed")
		return nil, fmt.Errorf("open stream %s: %w", opts.Target, err)
	}
	return newWTFrameConn(session, stream), nil
}
