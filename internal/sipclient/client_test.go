package sipclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/NimbleMarkets/go-booba/sip"
)

// blockingTTY's Read blocks forever on the pipe until Close is called.
type blockingTTY struct {
	r         *io.PipeReader
	stdout    *bytes.Buffer
	mu        sync.Mutex
	wasClosed bool
}

func (b *blockingTTY) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b *blockingTTY) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stdout.Write(p)
}
func (b *blockingTTY) Size() (int, int, error)        { return 80, 24, nil }
func (b *blockingTTY) MakeRaw() (func() error, error) { return func() error { return nil }, nil }
func (b *blockingTTY) Close() error {
	b.mu.Lock()
	b.wasClosed = true
	b.mu.Unlock()
	return b.r.Close()
}
func (b *blockingTTY) closed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.wasClosed
}

// fakeTTY is an in-memory TTY for tests. Writes go to stdout; reads come from
// stdin. MakeRaw is a no-op; Size returns fixed dimensions.
type fakeTTY struct {
	stdin  io.Reader
	stdout *bytes.Buffer
	mu     sync.Mutex
}

func newFakeTTY(input string) *fakeTTY {
	return &fakeTTY{
		stdin:  strings.NewReader(input),
		stdout: &bytes.Buffer{},
	}
}
func (f *fakeTTY) Read(p []byte) (int, error) { return f.stdin.Read(p) }
func (f *fakeTTY) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stdout.Write(p)
}
func (f *fakeTTY) Size() (int, int, error)        { return 80, 24, nil }
func (f *fakeTTY) MakeRaw() (func() error, error) { return func() error { return nil }, nil }
func (f *fakeTTY) Close() error                   { return nil }
func (f *fakeTTY) Output() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stdout.String()
}

// dialTest opens a real coder/websocket connection against an httptest server
// and returns the client-side conn plus a cleanup callback.
func dialTest(t *testing.T, h http.Handler) (*websocket.Conn, func()) {
	t.Helper()
	hs := httptest.NewServer(h)
	wsURL := "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	cancel()
	if err != nil {
		hs.Close()
		t.Fatalf("dial: %v", err)
	}
	return conn, func() { _ = conn.CloseNow(); hs.Close() }
}

func TestRunInteractive_ServerOutputThenClose(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		// Drain initial resize from client.
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgOutput, []byte("hi\r\n")))
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgClose, nil))
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	conn, cleanup := dialTest(t, mux)
	defer cleanup()

	tty := newFakeTTY("")
	opts := &Options{URL: "ws://test/ws", EscapeCharRaw: "^]"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runInteractive(ctx, conn, tty, opts, io.Discard); err != nil {
		t.Fatalf("runInteractive: %v", err)
	}
	if got := tty.Output(); !strings.Contains(got, "hi") {
		t.Errorf("tty output = %q; want to contain 'hi'", got)
	}
}

func TestRunInteractive_ForwardsInput(t *testing.T) {
	received := make(chan []byte, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		for i := 0; i < 2; i++ {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			typ, payload, _ := sip.DecodeWSMessage(data)
			if typ == sip.MsgInput {
				received <- payload
			}
		}
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgClose, nil))
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	conn, cleanup := dialTest(t, mux)
	defer cleanup()

	tty := newFakeTTY("hello\r")
	opts := &Options{URL: "ws://test/ws", EscapeCharRaw: "^]"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runInteractive(ctx, conn, tty, opts, io.Discard); err != nil {
		t.Fatalf("runInteractive: %v", err)
	}
	select {
	case got := <-received:
		if string(got) != "hello\r" {
			t.Errorf("server got %q; want %q", got, "hello\r")
		}
	default:
		t.Fatalf("server never received MsgInput")
	}
}

func TestRunInteractive_EscapeCharDisconnects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		// Drain frames from the client until it disconnects.
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	})
	conn, cleanup := dialTest(t, mux)
	defer cleanup()

	// Input sequence: Ctrl-] (0x1d) at start-of-line, then "quit\n" for the
	// escape prompt. Expect runInteractive to return nil cleanly.
	tty := newFakeTTY("\x1dquit\n")
	opts := &Options{URL: "ws://test/ws", EscapeCharRaw: "^]"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runInteractive(ctx, conn, tty, opts, io.Discard); err != nil {
		t.Fatalf("runInteractive: %v", err)
	}
	if got := tty.Output(); !strings.Contains(got, "booba-sip-client>") {
		t.Errorf("expected escape prompt in output; got %q", got)
	}
}

func TestRunInteractive_NoGoroutineLeakAfterShutdown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		// Drain initial resize.
		_, _, _ = conn.Read(r.Context())
		// Immediately close so runInteractive shuts down.
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgClose, nil))
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	conn, cleanup := dialTest(t, mux)
	defer cleanup()

	// A blocking stdin simulates a real terminal: its Read never naturally
	// returns. Only tty.Close() can unblock it.
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	blocking := &blockingTTY{r: pr, stdout: &bytes.Buffer{}}

	opts := &Options{URL: "ws://test/ws", EscapeCharRaw: "^]"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runInteractive(ctx, conn, blocking, opts, io.Discard) }()

	select {
	case <-done:
		// runInteractive returned. The stdin goroutine should have exited
		// because tty.Close() unblocked its Read.
		if !blocking.closed() {
			t.Errorf("tty.Close was not called on shutdown")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("runInteractive did not return within 2s")
	}
}

func TestRunInteractive_InitialResize(t *testing.T) {
	var mu sync.Mutex
	var gotCols, gotRows int
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		typ, payload, _ := sip.DecodeWSMessage(data)
		if typ == sip.MsgResize {
			var msg sip.ResizeMessage
			_ = json.Unmarshal(payload, &msg)
			mu.Lock()
			gotCols = msg.Cols
			gotRows = msg.Rows
			mu.Unlock()
		}
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgClose, nil))
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	conn, cleanup := dialTest(t, mux)
	defer cleanup()

	tty := newFakeTTY("")
	opts := &Options{URL: "ws://test/ws", EscapeCharRaw: "^]"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = runInteractive(ctx, conn, tty, opts, io.Discard)

	mu.Lock()
	defer mu.Unlock()
	if gotCols != 80 || gotRows != 24 {
		t.Errorf("resize = %dx%d; want 80x24", gotCols, gotRows)
	}
}
