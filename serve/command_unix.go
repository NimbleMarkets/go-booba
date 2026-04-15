//go:build !windows && !js

package serve

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/x/xpty"
)

func startCommandInPty(cmd *exec.Cmd, sess *ptySession) error {
	upty, ok := sess.Pty().(*xpty.UnixPty)
	if !ok {
		return fmt.Errorf("command mode requires Unix PTY")
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Start the child in a new session and make the PTY its controlling
	// terminal so fullscreen TUIs receive SIGWINCH and related terminal events.
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true

	return upty.Start(cmd)
}
