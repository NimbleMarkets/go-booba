package cli

import (
	"context"
	"fmt"

	"github.com/NimbleMarkets/go-booba/serve"
	"github.com/spf13/cobra"
)

// rootServeOpts holds the flags bound to booba's root command.
var rootServeOpts ServeOptions

var rootCmd = &cobra.Command{
	Use:   "booba [flags] -- <command> [args...]",
	Short: "Wrap a local CLI command and serve it through a browser terminal",
	Long: `booba wraps any local CLI program and serves it in the browser
through the same embedded Ghostty terminal stack that the go-booba library
uses. Everything after -- is treated as the wrapped command and its
arguments.

Examples:
  booba --listen 127.0.0.1:8080 -- htop
  booba --listen 127.0.0.1:8080 -- bash
  booba --listen 127.0.0.1:8080 --origin https://app.example.com -- vim README.md`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing wrapped command; pass it after --")
		}
		config, err := rootServeOpts.Config()
		if err != nil {
			return fmt.Errorf("invalid server flags: %w", err)
		}
		server := serve.NewServer(config)
		return server.ServeCommand(cmd.Context(), args[0], args[1:]...)
	},
}

func init() {
	AddServeFlags(rootCmd.Flags(), &rootServeOpts, "127.0.0.1:8080")
	rootCmd.AddCommand(docsCmd)
}

// RootCmd exposes the configured root command for tests and external tooling.
func RootCmd() *cobra.Command { return rootCmd }

// Execute runs the booba root command with ctx, returning any error.
func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}
