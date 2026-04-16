package serve

import (
	"context"
	"io"
)

// WindowSize represents terminal dimensions.
type WindowSize struct {
	Width  int
	Height int
}

// Session represents a single terminal session.
type Session interface {
	// Context returns the session context, cancelled on disconnect.
	Context() context.Context

	// OutputReader returns a reader for terminal output (PTY master read side).
	OutputReader() io.Reader

	// InputWriter returns a writer for terminal input (PTY master write side).
	InputWriter() io.Writer

	// Resize updates the PTY window size.
	Resize(cols, rows int)

	// WindowSize returns the current terminal dimensions.
	WindowSize() WindowSize

	// Done returns a channel that's closed when the session ends.
	Done() <-chan struct{}

	// Close cleans up the session. Close MUST be idempotent: subsequent
	// calls return nil after the first. SessionMiddleware that holds
	// resources should override Close, release its own resources, then
	// delegate to the embedded Session's Close.
	Close() error
}

// SessionFactory creates a new session for an incoming terminal client.
type SessionFactory func(ctx context.Context, size WindowSize) (Session, error)
