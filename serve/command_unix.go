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

	slave := upty.Slave()
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    int(slave.Fd()),
	}

	return cmd.Start()
}
