//go:build !windows && !js

package serve

import (
	"context"
	"io"
	"sync"

	"github.com/charmbracelet/x/xpty"
)

// ptySession implements Session using a pseudo-terminal.
type ptySession struct {
	pty     xpty.Pty
	winSize WindowSize
	resize  chan WindowSize
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	closed  bool
	mu      sync.Mutex
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
	size := WindowSize{Width: cols, Height: rows}

	s.mu.Lock()
	s.winSize = size
	s.mu.Unlock()

	select {
	case s.resize <- size:
	default:
		select {
		case <-s.resize:
		default:
		}
		s.resize <- size
	}

	_ = s.pty.Resize(cols, rows)
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
	s.mu.Unlock()

	s.cancel()
	close(s.done)
	return s.pty.Close()
}
