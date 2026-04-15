//go:build !js

package serve

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/xpty"
)

// Handler creates a tea.Model for each new session.
type Handler func(sess Session) tea.Model

// ProgramHandler creates a fully configured tea.Program for each new session.
type ProgramHandler func(sess Session) *tea.Program

// MakeTeaOptions returns tea.ProgramOption values that wire a BubbleTea program
// to the PTY session. Sets TERM=ghostty and COLORTERM=truecolor.
func MakeTeaOptions(sess Session) []tea.ProgramOption {
	ps, ok := sess.(*ptySession)
	if !ok {
		return []tea.ProgramOption{
			tea.WithInput(sess.OutputReader()),
			tea.WithOutput(sess.InputWriter()),
			tea.WithEnvironment([]string{"TERM=xterm-256color", "COLORTERM=truecolor"}),
		}
	}

	opts := []tea.ProgramOption{
		tea.WithEnvironment([]string{"TERM=ghostty", "COLORTERM=truecolor"}),
	}

	// Use the PTY slave fd for BubbleTea I/O so it gets proper terminal semantics.
	if upty, ok := ps.Pty().(*xpty.UnixPty); ok {
		slave := upty.Slave()
		opts = append(opts,
			tea.WithInput(slave),
			tea.WithOutput(slave),
		)
	} else {
		opts = append(opts,
			tea.WithInput(ps.Pty()),
			tea.WithOutput(ps.Pty()),
		)
	}

	return opts
}

// runBubbleTea starts a BubbleTea program attached to the session PTY.
func runBubbleTea(ctx context.Context, sess Session, handler Handler, extraOpts []tea.ProgramOption) error {
	model := handler(sess)
	opts := MakeTeaOptions(sess)
	opts = append(opts, extraOpts...)

	prog := tea.NewProgram(model, opts...)

	go func() {
		ws := sess.WindowSize()
		prog.Send(tea.WindowSizeMsg{Width: ws.Width, Height: ws.Height})
	}()

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("bubbletea: %w", err)
	}
	return nil
}

// runBubbleTeaProgram starts a pre-configured tea.Program.
func runBubbleTeaProgram(ctx context.Context, sess Session, handler ProgramHandler) error {
	prog := handler(sess)

	go func() {
		ws := sess.WindowSize()
		prog.Send(tea.WindowSizeMsg{Width: ws.Width, Height: ws.Height})
	}()

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("bubbletea: %w", err)
	}
	return nil
}
