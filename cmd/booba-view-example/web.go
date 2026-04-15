//go:build !js

package main

import (
	"context"
	"log"
	"net"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/booba/serve"
	"github.com/spf13/pflag"
)

var (
	flagListen    = pflag.String("listen", "", "start the web server on this address (e.g. 127.0.0.1:8080)")
	flagHTTP3Port = pflag.Int("http3-port", 0, "HTTP/3 WebTransport port (default: same as --listen, -1 to disable)")
	flagIdle      = pflag.Duration("idle-timeout", 0, "close idle HTTP/WebSocket sessions after this duration (0 disables)")
	flagCertFile  = pflag.String("cert-file", "", "TLS certificate file path for HTTPS/WSS/WebTransport")
	flagKeyFile   = pflag.String("key-file", "", "TLS key file path for HTTPS/WSS/WebTransport")
	flagReadOnly  = pflag.Bool("read-only", false, "disable client input")
	flagDebug     = pflag.Bool("debug", false, "verbose logging")
	flagOrigins   = pflag.String("origin", "", "comma-separated additional allowed browser origins (host patterns or scheme://host)")
	flagUsername  = pflag.String("username", "", "Basic Auth username")
	flagPassword  = pflag.String("password", "", "Basic Auth password")
)

func startWebServerIfRequested() bool {
	if *flagListen == "" {
		return false
	}

	config := serve.DefaultConfig()

	if host, port, err := net.SplitHostPort(*flagListen); err == nil {
		config.Host = host
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}

	config.HTTP3Port = *flagHTTP3Port
	config.IdleTimeout = *flagIdle
	config.CertFile = *flagCertFile
	config.KeyFile = *flagKeyFile
	config.ReadOnly = *flagReadOnly
	config.Debug = *flagDebug
	config.BasicUsername = *flagUsername
	config.BasicPassword = *flagPassword

	if *flagOrigins != "" {
		for _, pattern := range strings.Split(*flagOrigins, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern != "" {
				config.OriginPatterns = append(config.OriginPatterns, pattern)
			}
		}
	}

	server := serve.NewServer(config)

	ctx := context.Background()
	if err := server.Serve(ctx, func(sess serve.Session) tea.Model {
		return model{0, false, 3600, 0, 0, false, false}
	}); err != nil {
		log.Fatal("Server error:", err)
	}
	return true
}
