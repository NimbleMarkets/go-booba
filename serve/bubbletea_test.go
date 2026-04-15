//go:build !js

package serve

import (
	"context"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestForwardResizeEventsSendsInitialAndSubsequentSizes(t *testing.T) {
	sess := &resizeTestSession{
		done:   make(chan struct{}),
		size:   WindowSize{Width: 80, Height: 24},
		resize: make(chan WindowSize, 4),
	}
	prog := &tea.Program{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgs := make(chan tea.WindowSizeMsg, 4)
	progSend = func(_ *tea.Program, msg tea.Msg) {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			msgs <- ws
		}
	}
	defer func() { progSend = defaultProgSend }()

	forwardResizeEvents(ctx, sess, prog)

	assertWindowSizeMsg(t, msgs, tea.WindowSizeMsg{Width: 80, Height: 24})

	sess.resize <- WindowSize{Width: 120, Height: 40}
	assertWindowSizeMsg(t, msgs, tea.WindowSizeMsg{Width: 120, Height: 40})
}

func assertWindowSizeMsg(t *testing.T, ch <-chan tea.WindowSizeMsg, want tea.WindowSizeMsg) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("WindowSizeMsg = %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for WindowSizeMsg %+v", want)
	}
}

type resizeTestSession struct {
	done   chan struct{}
	size   WindowSize
	resize chan WindowSize
}

func (s *resizeTestSession) Context() context.Context { return context.Background() }
func (s *resizeTestSession) OutputReader() io.Reader  { return nil }
func (s *resizeTestSession) InputWriter() io.Writer   { return io.Discard }
func (s *resizeTestSession) Resize(cols, rows int)    {}
func (s *resizeTestSession) WindowSize() WindowSize   { return s.size }
func (s *resizeTestSession) Done() <-chan struct{}    { return s.done }
func (s *resizeTestSession) Close() error             { close(s.done); return nil }
func (s *resizeTestSession) ResizeEvents() <-chan WindowSize {
	return s.resize
}
