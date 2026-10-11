//go:build !js

package serve

import (
	"context"
	"io"
	"sync"
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

type shutdownModel struct {
	ready chan struct{}
	once  *sync.Once
}

func (m shutdownModel) Init() tea.Cmd                           { return nil }
func (m shutdownModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m shutdownModel) View() tea.View {
	m.once.Do(func() { close(m.ready) })
	return tea.NewView("ready")
}

func TestBubbleTeaDisconnectStopsBeforePTYClose(t *testing.T) {
	for i := 0; i < 20; i++ {
		sess, err := newPtySession(context.Background(), WindowSize{Width: 80, Height: 24})
		if err != nil {
			t.Fatal(err)
		}
		ready := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- runBubbleTea(context.Background(), sess, sess, func(Session) (tea.Model, []tea.ProgramOption) {
				return shutdownModel{ready: ready, once: new(sync.Once)}, []tea.ProgramOption{tea.WithoutSignalHandler()}
			})
		}()
		select {
		case <-ready:
		case <-time.After(3 * time.Second):
			t.Fatal("program did not render")
		}
		// No consumer drains the terminal output: Close must permit the renderer's
		// final writes, join the reader, and release the PTY without a race or hang.
		closed := make(chan error, 1)
		go func() { closed <- sess.Close() }()
		select {
		case err := <-closed:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("disconnect hung")
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("program outlived PTY")
		}
	}
}

func TestBubbleTeaDoesNotStartAfterSessionCloses(t *testing.T) {
	sess, err := newPtySession(context.Background(), WindowSize{Width: 80, Height: 24})
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Close(); err != nil {
		t.Fatal(err)
	}
	err = runBubbleTea(context.Background(), sess, sess, func(Session) (tea.Model, []tea.ProgramOption) {
		t.Error("constructed model on closed PTY")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBubbleTeaHandlerCanDeclineSession(t *testing.T) {
	sess, err := newPtySession(context.Background(), WindowSize{Width: 80, Height: 24})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- runBubbleTea(context.Background(), sess, sess, func(Session) (tea.Model, []tea.ProgramOption) {
			_ = sess.Close()
			return nil, nil // Closing in the factory must not deadlock or start a program.
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("closing in handler blocked")
	}
}
