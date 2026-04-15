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
	flagWeb      = pflag.String("web", "", "start web server on this address (e.g. :8080)")
	flagWTPort   = pflag.Int("wt-port", 0, "WebTransport port (default: web port + 1, -1 to disable)")
	flagTLSCert  = pflag.String("tls-cert", "", "TLS certificate file path")
	flagTLSKey   = pflag.String("tls-key", "", "TLS key file path")
	flagReadOnly = pflag.Bool("read-only", false, "disable client input")
	flagDebug    = pflag.Bool("debug", false, "verbose logging")
	flagOrigins  = pflag.String("origin", "", "comma-separated additional allowed browser origins (host patterns or scheme://host)")
	flagUsername = pflag.String("username", "", "Basic Auth username")
	flagPassword = pflag.String("password", "", "Basic Auth password")
)

func startWebServerIfRequested() bool {
	if *flagWeb == "" {
		return false
	}

	config := serve.DefaultConfig()

	if host, port, err := net.SplitHostPort(*flagWeb); err == nil {
		config.Host = host
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}

	config.WTPort = *flagWTPort
	config.TLSCert = *flagTLSCert
	config.TLSKey = *flagTLSKey
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
