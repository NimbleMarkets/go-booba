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
func runBubbleTea(ctx context.Context, sess, rawSess Session, handler Handler) error {
	if sess.Context().Err() != nil {
		return nil
	}
	model, extraOpts := handler(sess)
	if ps, ok := rawSess.(*ptySession); ok {
		release, open := ps.beginBubbleTea()
		if !open {
			return nil
		}
		defer release()
	}

	opts := MakeOptions(rawSess)
	opts = append(opts, extraOpts...)

	prog := tea.NewProgram(model, opts...)
	// Quit shuts down gracefully and joins the input reader. WithContext/Kill
	// uses Bubble Tea's forced shutdown path, which skips that join.
	stop := context.AfterFunc(sess.Context(), prog.Quit)
	defer stop()
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
