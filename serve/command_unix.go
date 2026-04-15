//go:build !windows && !js

package serve

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/x/xpty"
)

func startCommandInPty(cmd *exec.Cmd, sess *ptySession) error {
	upty, ok := sess.Pty().(*xpty.UnixPty)
	if !ok {
		return fmt.Errorf("command mode requires Unix PTY")
	}

	// Let xpty configure the child process and controlling terminal correctly
	// for the current Unix platform.
	return upty.Start(cmd)
}
