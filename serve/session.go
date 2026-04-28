package serve

import (
	"context"
	"io"
)

// WindowSize represents terminal dimensions. WidthPx and HeightPx are the
// canvas dimensions in pixels and are optional. When non-zero on a resize
// they're forwarded to the PTY's TIOCSWINSZ ws_xpixel/ws_ypixel fields, which
// kitty graphics tools (e.g. kitten icat) read to size images.
type WindowSize struct {
	Width    int
	Height   int
	WidthPx  int
	HeightPx int
}

// WindowResizer is an optional capability for Sessions that can apply pixel
// dimensions in addition to character dimensions. The Sip protocol's resize
// message carries optional widthPx/heightPx; when present and the session
// implements this interface, the pixel size is forwarded to the underlying
// PTY so TIOCGWINSZ returns real values to client tools.
type WindowResizer interface {
	ResizeWindow(size WindowSize)
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
