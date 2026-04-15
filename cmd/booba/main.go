//go:build !js

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/NimbleMarkets/booba/internal/cli"
	"github.com/NimbleMarkets/booba/serve"
	"github.com/spf13/pflag"
)

func main() {
	var serveOpts cli.ServeOptions
	cli.AddServeFlags(pflag.CommandLine, &serveOpts, "127.0.0.1:8080")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] -- <command> [args...]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Wrap a local CLI command and expose it through booba's browser terminal.")
		fmt.Fprintln(os.Stderr)
		pflag.PrintDefaults()
	}
	pflag.Parse()

	args := pflag.Args()
	if len(args) == 0 {
		pflag.Usage()
		os.Exit(2)
	}

	config, err := serveOpts.Config()
	if err != nil {
		log.Fatal("Invalid server flags:", err)
	}

	server := serve.NewServer(config)
	if err := server.ServeCommand(context.Background(), args[0], args[1:]...); err != nil {
		log.Fatal("Server error:", err)
	}
}
