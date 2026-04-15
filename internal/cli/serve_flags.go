package cli

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/booba/serve"
	"github.com/spf13/pflag"
)

// ServeOptions holds CLI flags for configuring the booba HTTP/WebTransport server.
type ServeOptions struct {
	Listen    string
	HTTP3Port int
	Idle      time.Duration
	CertFile  string
	KeyFile   string
	ReadOnly  bool
	Debug     bool
	Origins   string
	Username  string
	Password  string
}

// AddServeFlags registers standard booba server flags on the provided FlagSet.
func AddServeFlags(fs *pflag.FlagSet, opts *ServeOptions, defaultListen string) {
	fs.StringVar(&opts.Listen, "listen", defaultListen, "start the web server on this address (e.g. 127.0.0.1:8080)")
	fs.IntVar(&opts.HTTP3Port, "http3-port", 0, "HTTP/3 WebTransport port (default: same as --listen, -1 to disable)")
	fs.DurationVar(&opts.Idle, "idle-timeout", 0, "close idle HTTP/WebSocket sessions after this duration (0 disables)")
	fs.StringVar(&opts.CertFile, "cert-file", "", "TLS certificate file path for HTTPS/WSS/WebTransport")
	fs.StringVar(&opts.KeyFile, "key-file", "", "TLS key file path for HTTPS/WSS/WebTransport")
	fs.BoolVar(&opts.ReadOnly, "read-only", false, "disable client input")
	fs.BoolVar(&opts.Debug, "debug", false, "verbose logging")
	fs.StringVar(&opts.Origins, "origin", "", "comma-separated additional allowed browser origins (host patterns or scheme://host)")
	fs.StringVar(&opts.Username, "username", "", "Basic Auth username")
	fs.StringVar(&opts.Password, "password", "", "Basic Auth password")
}

// Config converts CLI options into a serve.Config.
func (opts ServeOptions) Config() (serve.Config, error) {
	config := serve.DefaultConfig()

	if opts.Listen != "" {
		host, port, err := net.SplitHostPort(opts.Listen)
		if err != nil {
			return config, fmt.Errorf("parse --listen: %w", err)
		}
		config.Host = host
		p, err := strconv.Atoi(port)
		if err != nil {
			return config, fmt.Errorf("parse --listen port: %w", err)
		}
		config.Port = p
	}

	config.HTTP3Port = opts.HTTP3Port
	config.IdleTimeout = opts.Idle
	config.CertFile = opts.CertFile
	config.KeyFile = opts.KeyFile
	config.ReadOnly = opts.ReadOnly
	config.Debug = opts.Debug
	config.BasicUsername = opts.Username
	config.BasicPassword = opts.Password

	if opts.Origins != "" {
		for _, pattern := range strings.Split(opts.Origins, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern != "" {
				config.OriginPatterns = append(config.OriginPatterns, pattern)
			}
		}
	}

	return config, nil
}
