package sipclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/term"

	"github.com/NimbleMarkets/go-booba/sip"
)

// TTY abstracts the pieces of a local terminal the interactive client needs.
// Production code uses realTTY, tests use a fake implementation.
type TTY interface {
	Read(p []byte) (int, error)  // stdin
	Write(p []byte) (int, error) // stdout
	Size() (cols, rows int, err error)
	MakeRaw() (restore func() error, err error)
}

// interactiveHandler implements FrameHandler by writing output bytes to the
// tty, emitting OSC 2 for titles, and signaling close via a channel.
type interactiveHandler struct {
	tty       TTY
	readOnly  atomic.Bool // server-asserted; OR'd with opts.ReadOnly
	closeOnce sync.Once
	closed    chan struct{}
}

func (h *interactiveHandler) HandleOutput(p []byte) { _, _ = h.tty.Write(p) }
func (h *interactiveHandler) HandleTitle(title string) {
	_, _ = fmt.Fprintf(h.tty, "\x1b]2;%s\x07", title)
}
func (h *interactiveHandler) HandleOptions(o sip.OptionsMessage) { h.readOnly.Store(o.ReadOnly) }
func (h *interactiveHandler) HandleKittyFlags(flags int) {
	// Push the server-advertised flags to the local terminal so it emits
	// keys encoded for those flags. CSI > <flags> u.
	_, _ = fmt.Fprintf(h.tty, "\x1b[>%du", flags)
}
func (h *interactiveHandler) HandleClose(_ []byte) {
	h.closeOnce.Do(func() { close(h.closed) })
}

// watchResize is a platform-specific hook that fires cb whenever the terminal
// resizes. It returns when ctx is canceled. Unix implementations use SIGWINCH;
// others poll. It is nil on unsupported platforms and runInteractive handles
// that gracefully.
var watchResize func(ctx context.Context, cb func())

// runInteractive is the pump loop. It is called with an already-dialed
// connection and a configured tty. It returns when either side ends the
// session, ctx is canceled, or a pump errors.
func runInteractive(ctx context.Context, conn *websocket.Conn, tty TTY, opts *Options, stderr io.Writer) error {
	esc, err := ParseEscapeChar(opts.EscapeCharRaw)
	if err != nil {
		return err
	}
	handler := &interactiveHandler{tty: tty, closed: make(chan struct{})}
	readOnly := func() bool { return opts.ReadOnly || handler.readOnly.Load() }
	router := &Router{
		Handler: handler,
		Pong: func() error {
			_ = conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgPong, nil))
			return nil
		},
	}
	if opts.Debug {
		router.Debug = func(t byte, p []byte) {
			_, _ = fmt.Fprintf(stderr, "debug: frame type=%q len=%d\n", t, len(p))
		}
	}

	// Send initial resize so the server sizes the PTY correctly.
	if cols, rows, err := tty.Size(); err == nil {
		if err := sendResize(ctx, conn, cols, rows); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	if watchResize != nil {
		go func() {
			var timer *time.Timer
			watchResize(ctx, func() {
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(50*time.Millisecond, func() {
					cols, rows, err := tty.Size()
					if err != nil {
						return
					}
					_ = sendResize(ctx, conn, cols, rows)
				})
			})
		}()
	}

	// Server → client pump.
	go func() {
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				cancel(err)
				return
			}
			msgType, payload, derr := sip.DecodeWSMessage(data)
			if derr != nil {
				cancel(derr)
				return
			}
			if err := router.Route(msgType, payload); err != nil {
				cancel(err)
				return
			}
		}
	}()

	// Client → server pump.
	go func() {
		sol := NewSOLTracker()
		buf := make([]byte, 4096)
		for {
			n, err := tty.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					// EOF on stdin: stop forwarding input but let the
					// server→client pump drive the close.
					return
				}
				cancel(err)
				return
			}
			chunk := buf[:n]

			// Escape-char detection: only at start-of-line, only if
			// enabled. Split the chunk around the escape byte.
			if !esc.None {
				if idx := indexByteAtSOL(chunk, esc.Byte, sol); idx >= 0 {
					before := chunk[:idx]
					after := chunk[idx+1:]
					if len(before) > 0 {
						if !readOnly() {
							if err := sendInput(ctx, conn, before); err != nil {
								cancel(err)
								return
							}
						}
						sol.Observe(before)
					}
					// Enter escape prompt. Caller decides the action.
					action, err := RunEscapePrompt(tty, tty, PromptInfo{URL: opts.URL})
					if err != nil {
						cancel(err)
						return
					}
					if action == ActionDisconnect {
						cancel(nil)
						return
					}
					chunk = after
				}
			}
			if len(chunk) == 0 {
				continue
			}
			if !readOnly() {
				if err := sendInput(ctx, conn, chunk); err != nil {
					cancel(err)
					return
				}
			}
			sol.Observe(chunk)
		}
	}()

	<-ctx.Done()
	cause := context.Cause(ctx)
	select {
	case <-handler.closed:
		return nil
	default:
	}
	if cause == nil || errors.Is(cause, context.Canceled) {
		return nil
	}
	if errors.Is(cause, ErrSessionClosed) {
		return nil
	}
	if websocket.CloseStatus(cause) == websocket.StatusNormalClosure {
		return nil
	}
	if errors.Is(cause, ErrConnect) || errors.Is(cause, ErrProtocol) || errors.Is(cause, ErrTransport) {
		return cause
	}
	return fmt.Errorf("%w: %v", ErrTransport, cause)
}

// indexByteAtSOL returns the index of the first occurrence of c in b where
// the SOLTracker reports start-of-line. The tracker is NOT advanced past the
// escape byte — callers split the chunk themselves.
func indexByteAtSOL(b []byte, c byte, sol *SOLTracker) int {
	for i, x := range b {
		if x == c && atSOL(sol, b[:i]) {
			return i
		}
	}
	return -1
}

// atSOL reports whether the next byte of b after pre would be at start-of-line.
// If pre is empty, it defers to sol.AtStart(); otherwise, it peeks at pre's
// last byte (matching SOLTracker.Observe's contract).
func atSOL(sol *SOLTracker, pre []byte) bool {
	if len(pre) == 0 {
		return sol.AtStart()
	}
	last := pre[len(pre)-1]
	return last == '\r' || last == '\n'
}

func sendInput(ctx context.Context, conn *websocket.Conn, p []byte) error {
	return conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgInput, p))
}

func sendResize(ctx context.Context, conn *websocket.Conn, cols, rows int) error {
	body, err := json.Marshal(sip.ResizeMessage{Cols: cols, Rows: rows})
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgResize, body))
}

// realTTY wraps os.Stdin/os.Stdout and x/term for production use.
type realTTY struct {
	fd int
}

func newRealTTY() *realTTY { return &realTTY{fd: int(os.Stdin.Fd())} }

func (r *realTTY) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (r *realTTY) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (r *realTTY) Size() (int, int, error)     { return term.GetSize(r.fd) }
func (r *realTTY) MakeRaw() (func() error, error) {
	if !term.IsTerminal(r.fd) {
		return func() error { return nil }, nil
	}
	state, err := term.MakeRaw(r.fd)
	if err != nil {
		return nil, err
	}
	return func() error { return term.Restore(r.fd, state) }, nil
}

// RunInteractive is called from root.go when --dump-frames is NOT set. It
// dials the server, puts the tty into raw mode, and hands off to
// runInteractive. All stdout writes during interactive mode go to the tty;
// stderr is reserved for status and debug output.
func RunInteractive(ctx context.Context, _, stderr io.Writer, opts *Options) error {
	target, err := ParseTargetURL(opts.URL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnect, err)
	}
	headers, err := ParseHeaders(opts.Headers)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnect, err)
	}
	tlsCfg, err := BuildTLSConfig(opts.InsecureSkipVerify, opts.CAFile)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnect, err)
	}
	conn, err := Dial(ctx, DialOptions{
		Target:  target,
		Origin:  opts.Origin,
		Headers: headers,
		TLS:     tlsCfg,
		Timeout: opts.ConnectTimeout,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnect, err)
	}
	defer func() { _ = conn.CloseNow() }()

	tty := newRealTTY()
	restore, err := tty.MakeRaw()
	if err != nil {
		return err
	}
	defer func() { _ = restore() }()

	if opts.Kitty && !opts.NoKitty {
		flags, ok := QueryKittyFlags(os.Stdin, os.Stdout, 100*time.Millisecond)
		if ok && flags > 0 {
			if err := PushKittyFlags(os.Stdout, flags); err == nil {
				defer func() { _ = PopKittyFlags(os.Stdout) }()
				body, err := json.Marshal(sip.KittyKbdMessage{Flags: flags})
				if err == nil {
					_ = conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgKittyKbd, body))
				}
			}
		} else if opts.Debug {
			_, _ = fmt.Fprintln(stderr, "debug: terminal did not respond to Kitty query")
		}
	}

	err = runInteractive(ctx, conn, tty, opts, stderr)
	_, _ = fmt.Fprintln(stderr, "Connection closed")
	_ = conn.Close(websocket.StatusNormalClosure, "")
	return err
}
