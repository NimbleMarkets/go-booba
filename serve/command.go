package serve

import (
	"context"
	"os/exec"
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

	err := cmd.Wait()
	if ctx.Err() != nil {
		return nil // Context cancelled (client disconnected)
	}
	return err
}
