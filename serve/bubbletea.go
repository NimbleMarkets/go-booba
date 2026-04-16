//go:build !js

package serve

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/xpty"
)

// Handler creates a tea.Model and any additional tea.ProgramOption values
// for each new session. The returned options are appended to the defaults
// returned by [MakeOptions].
type Handler func(sess Session) (tea.Model, []tea.ProgramOption)

// ProgramHandler creates a fully configured tea.Program for each new session.
type ProgramHandler func(sess Session) *tea.Program

type resizeEventSource interface {
	ResizeEvents() <-chan WindowSize
}

var progSend = defaultProgSend

func defaultProgSend(prog *tea.Program, msg tea.Msg) {
	prog.Send(msg)
}

// MakeOptions returns tea.ProgramOption values that wire a BubbleTea program
// to the PTY session. Sets TERM=ghostty and COLORTERM=truecolor.
func MakeOptions(sess Session) []tea.ProgramOption {
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
func runBubbleTea(ctx context.Context, sess Session, handler Handler) error {
	model, extraOpts := handler(sess)
	opts := MakeOptions(sess)
	opts = append(opts, extraOpts...)

	prog := tea.NewProgram(model, opts...)
	forwardResizeEvents(ctx, sess, prog)

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("bubbletea: %w", err)
	}
	return nil
}

// runBubbleTeaProgram starts a pre-configured tea.Program.
func runBubbleTeaProgram(ctx context.Context, sess Session, handler ProgramHandler) error {
	prog := handler(sess)
	forwardResizeEvents(ctx, sess, prog)

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("bubbletea: %w", err)
	}
	return nil
}

func forwardResizeEvents(ctx context.Context, sess Session, prog *tea.Program) {
	go func() {
		ws := sess.WindowSize()
		progSend(prog, tea.WindowSizeMsg{Width: ws.Width, Height: ws.Height})

		source, ok := sess.(resizeEventSource)
		if !ok {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-sess.Done():
				return
			case ws, ok := <-source.ResizeEvents():
				if !ok {
					return
				}
				progSend(prog, tea.WindowSizeMsg{Width: ws.Width, Height: ws.Height})
			}
		}
	}()
}
