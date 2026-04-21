//go:build !js

package serve

import (
	"context"
	"os/exec"

	"github.com/charmbracelet/x/xpty"
)

// runCommand starts an external command attached to the session PTY.
func runCommand(ctx context.Context, sess *ptySession, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(),
		"TERM=ghostty",
		"COLORTERM=truecolor",
	)

	if err := startCommandInPty(cmd, sess); err != nil {
		return err
	}

	err := xpty.WaitProcess(ctx, cmd)
	if ctx.Err() != nil {
		return nil // Context cancelled (client disconnected)
	}
	return err
}
