//go:build windows && !js

package serve

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/x/xpty"
)

func startCommandInPty(cmd *exec.Cmd, sess *ptySession) error {
	cpty, ok := sess.Pty().(*xpty.ConPty)
	if !ok {
		return fmt.Errorf("command mode requires ConPTY")
	}

	return cpty.Start(cmd)
}
