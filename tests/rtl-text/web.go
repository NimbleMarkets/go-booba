//go:build !js

package main

import (
	"context"
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/internal/cli"
	"github.com/NimbleMarkets/go-booba/serve"
	"github.com/spf13/pflag"
)

var serveOpts cli.ServeOptions

func init() {
	cli.AddServeFlags(pflag.CommandLine, &serveOpts, "")
}

func startWebServerIfRequested() bool {
	if serveOpts.Listen == "" {
		return false
	}

	config, err := serveOpts.Config()
	if err != nil {
		log.Fatal("Invalid server flags:", err)
	}

	server := serve.NewServer(config)

	ctx := context.Background()
	if err := server.Serve(ctx, func(_ serve.Session) (tea.Model, []tea.ProgramOption) {
		return model{}, nil
	}); err != nil {
		log.Fatal("Server error:", err)
	}
	return true
}
