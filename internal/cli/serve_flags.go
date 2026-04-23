package cli

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/go-booba/serve"
	"github.com/spf13/pflag"
)

// PasswordEnvVar is the environment variable consulted as a last-resort
// source for the Basic Auth password when neither --password nor
// --password-file is set.
const PasswordEnvVar = "BOOBA_PASSWORD"

// ServeOptions holds CLI flags for configuring the booba HTTP/WebTransport server.
type ServeOptions struct {
	Listen       string
	HTTP3Port    int
	Idle         time.Duration
	CertFile     string
	KeyFile      string
	ReadOnly     bool
	Debug        bool
	Origins      string
	Username     string
	Password     string
	PasswordFile string
}

// AddServeFlags registers standard booba server flags on the provided FlagSet.
func AddServeFlags(fs *pflag.FlagSet, opts *ServeOptions, defaultListen string) {
	fs.StringVarP(&opts.Listen, "listen", "l", defaultListen, "start the web server on this address (e.g. 127.0.0.1:8080)")
	fs.IntVar(&opts.HTTP3Port, "http3-port", 0, "HTTP/3 WebTransport port (default: same as --listen, -1 to disable)")
	fs.DurationVar(&opts.Idle, "idle-timeout", 0, "close idle HTTP/WebSocket sessions after this duration (0 disables)")
	fs.StringVar(&opts.CertFile, "cert-file", "", "TLS certificate file path for HTTPS/WSS/WebTransport")
	fs.StringVar(&opts.KeyFile, "key-file", "", "TLS key file path for HTTPS/WSS/WebTransport")
	fs.BoolVar(&opts.ReadOnly, "read-only", false, "disable client input")
	fs.BoolVar(&opts.Debug, "debug", false, "verbose logging")
	fs.StringVar(&opts.Origins, "origin", "", "comma-separated additional allowed browser origins (path.Match shell globs, e.g. '*.example.com' or 'https://app.example.com'; NOT regex)")
	fs.StringVar(&opts.Username, "username", "", "Basic Auth username")
	fs.StringVar(&opts.Password, "password", "", "Basic Auth password (prefer --password-file or $"+PasswordEnvVar+" to keep secrets off argv)")
	fs.StringVar(&opts.PasswordFile, "password-file", "", "path to a file containing the Basic Auth password (trailing whitespace is trimmed)")
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

	password, err := opts.resolvePassword()
	if err != nil {
		return config, err
	}
	config.BasicPassword = password

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

// resolvePassword applies the documented precedence: --password flag,
// then --password-file contents (trimmed), then the $BOOBA_PASSWORD
// environment variable. Missing password files are an error so a
// misconfigured secret path doesn't silently fall through to an empty
// password.
func (opts ServeOptions) resolvePassword() (string, error) {
	if opts.Password != "" {
		return opts.Password, nil
	}
	if opts.PasswordFile != "" {
		data, err := os.ReadFile(opts.PasswordFile)
		if err != nil {
			return "", fmt.Errorf("read --password-file: %w", err)
		}
		return strings.TrimRight(string(data), " \t\r\n"), nil
	}
	return os.Getenv(PasswordEnvVar), nil
}
