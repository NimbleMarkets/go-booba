//go:build !js

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/booba/serve"
)

var flagWeb = flag.String("web", "", "start web server on this address (e.g. :8080)")

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

	server := serve.NewServer(config)

	ctx := context.Background()
	if err := server.Serve(ctx, func(sess serve.Session) tea.Model {
		return model{0, false, 3600, 0, 0, false, false}
	}); err != nil {
		log.Fatal("Server error:", err)
	}
	return true
}
