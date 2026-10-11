//go:build !js

package serve

import (
	"context"
	"io"
	"sync"

	"github.com/charmbracelet/x/xpty"
)

// ptySession implements Session using a pseudo-terminal.
type ptySession struct {
	pty          xpty.Pty
	winSize      WindowSize
	resize       chan WindowSize
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	closed       bool
	workloadDone chan struct{}
	mu           sync.Mutex
}

// newPtySession creates a new PTY session with the given initial size.
func newPtySession(ctx context.Context, size WindowSize) (*ptySession, error) {
	pty, err := xpty.NewPty(size.Width, size.Height)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	return &ptySession{
		pty:     pty,
		winSize: size,
		resize:  make(chan WindowSize, 1),
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}, nil
}

func defaultSessionFactory(ctx context.Context, size WindowSize) (Session, error) {
	return newPtySession(ctx, size)
}

func (s *ptySession) Context() context.Context { return s.ctx }
func (s *ptySession) OutputReader() io.Reader  { return s.pty }
func (s *ptySession) InputWriter() io.Writer   { return s.pty }
func (s *ptySession) Done() <-chan struct{}    { return s.done }

func (s *ptySession) WindowSize() WindowSize {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.winSize
}

func (s *ptySession) Resize(cols, rows int) {
	s.applyResize(WindowSize{Width: cols, Height: rows})
}

// ResizeWindow applies a full WindowSize including any pixel dimensions to the
// underlying PTY, so TIOCGWINSZ on the slave side reports ws_xpixel/ws_ypixel
// matching the client's canvas. Implements WindowResizer.
func (s *ptySession) ResizeWindow(size WindowSize) {
	s.applyResize(size)
}

func (s *ptySession) applyResize(size WindowSize) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.winSize = size
	defer s.mu.Unlock()

	select {
	case s.resize <- size:
	default:
		select {
		case <-s.resize:
		default:
		}
		s.resize <- size
	}

	// Use the pixel-aware setWinsize when the underlying PTY supports it
	// (UnixPty does), otherwise fall back to character-only Resize.
	type pixelResizer interface {
		SetWinsize(width, height, x, y int) error
	}
	if pr, ok := s.pty.(pixelResizer); ok {
		_ = pr.SetWinsize(size.Width, size.Height, size.WidthPx, size.HeightPx)
		return
	}
	_ = s.pty.Resize(size.Width, size.Height)
}

// Pty returns the underlying PTY for attaching to processes.
func (s *ptySession) Pty() xpty.Pty { return s.pty }

func (s *ptySession) ResizeEvents() <-chan WindowSize { return s.resize }

func (s *ptySession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	done := s.workloadDone
	s.mu.Unlock()

	s.cancel()
	close(s.done)
	if done != nil {
		// The transport may have stopped reading. Keep draining so the renderer's
		// final writes cannot block while Bubble Tea restores terminal state.
		select {
		case <-done:
		default:
			go func() { _, _ = io.Copy(io.Discard, s.pty) }()
			<-done
		}
	}
	return s.pty.Close()
}

// beginBubbleTea reserves the PTY before terminal setup.
// Close cancels the session first, then waits for the owner to release it.
func (s *ptySession) beginBubbleTea() (func(), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.workloadDone != nil {
		return nil, false
	}
	done := make(chan struct{})
	s.workloadDone = done
	return func() { close(done) }, true
}
