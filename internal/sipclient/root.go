package sipclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
)

// Options holds every flag the client supports. Each field is wired to a pflag
// in newRootCmd() and consumed by the interactive or dump-frames runners.
type Options struct {
	URL                string
	Origin             string
	Headers            []string
	InsecureSkipVerify bool
	CAFile             string
	EscapeCharRaw      string
	ReadOnly           bool
	Kitty              bool
	NoKitty            bool
	Debug              bool
	DumpFrames         bool
	DumpInputPath      string
	DumpTimeout        time.Duration
	ConnectTimeout     time.Duration
}

func newRootCmd() *cobra.Command {
	var opts Options
	cmd := &cobra.Command{
		Use:           "booba-sip-client [flags] <url>",
		Short:         "Connect to a booba server and either run interactively or dump frames",
		Long:          `booba-sip-client connects to a booba server over WebSocket (ws:// or wss://).`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("url is required (e.g., ws://host:8080/ws)")
			}
			opts.URL = args[0]
			return run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), &opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.Origin, "origin", "", "Origin header value (defaults to target URL's scheme+host)")
	f.StringArrayVar(&opts.Headers, "header", nil, "Extra request header, as 'Key: Value' (repeatable)")
	f.BoolVar(&opts.InsecureSkipVerify, "insecure-skip-verify", false, "Accept self-signed TLS certs for wss://")
	f.StringVar(&opts.CAFile, "ca-file", "", "Additional trust anchor PEM file for wss://")
	f.StringVar(&opts.EscapeCharRaw, "escape-char", "^]", "Local escape char (^X notation, or 'none' to disable)")
	f.BoolVar(&opts.ReadOnly, "read-only", false, "Ignore local input; still render server output")
	f.BoolVar(&opts.Kitty, "kitty", true, "Enable Kitty keyboard passthrough (auto-detected)")
	f.BoolVar(&opts.NoKitty, "no-kitty", false, "Force Kitty keyboard passthrough off")
	f.BoolVar(&opts.Debug, "debug", false, "Log decoded frames to stderr")
	f.BoolVar(&opts.DumpFrames, "dump-frames", false, "Non-interactive: print frames as JSON lines to stdout")
	f.StringVar(&opts.DumpInputPath, "dump-input", "", "With --dump-frames: file whose contents are sent as MsgInput after connect")
	f.DurationVar(&opts.DumpTimeout, "dump-timeout", 0, "With --dump-frames: exit after this long (0 = no timeout)")
	f.DurationVar(&opts.ConnectTimeout, "connect-timeout", 10*time.Second, "Dial/upgrade timeout")
	return cmd
}

// run is the dispatcher called after flag parsing. Later tasks fill it in; for
// now it returns a sentinel so the "URL accepted" test case isn't gated on a
// completed implementation.
func run(_ context.Context, _ io.Writer, _ io.Writer, opts *Options) error {
	return fmt.Errorf("not implemented: url=%q", opts.URL)
}

// Execute is the main entry point used by cmd/booba-sip-client/main.go.
func Execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}
