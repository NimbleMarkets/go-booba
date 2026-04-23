# `booba-sip-client` WebTransport Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add WebTransport client support to `booba-sip-client` so it can dial `https://host/wt` URLs in addition to the existing `ws://` / `wss://` paths, with no new CLI flags and full feature parity between transports.

**Architecture:** Introduce a `FrameConn` interface inside `internal/sipclient/` that abstracts the framed transport. The existing `*websocket.Conn` call sites (`RunDump`, `runInteractive`, `Router.Pong`, send helpers) are refactored to depend on this interface. A `wsFrameConn` wraps the current WebSocket path; a new `wtFrameConn` wraps `*webtransport.Session` plus its bidirectional stream. `Dial` dispatches by URL scheme.

**Tech Stack:**
- Go 1.25, module `github.com/NimbleMarkets/go-booba`
- `github.com/coder/websocket` v1.8.14 (already direct)
- `github.com/quic-go/webtransport-go` v0.10.0 (already indirect via `serve/`; promoted to direct by this work)
- Standard Go testing

**Design spec:** `docs/superpowers/specs/2026-04-22-booba-sip-client-webtransport-design.md`

**Relevant API facts (verified against the installed deps):**
- `webtransport.Dialer.Dial(ctx, urlStr, reqHdr) (*http.Response, *Session, error)` — **three** return values; the HTTP response is discarded for our purposes.
- `webtransport.Dialer.TLSClientConfig` must have `NextProtos` including `"h3"` (the library expects the HTTP/3 ALPN; it does not set it for us).
- `session.OpenStreamSync(ctx) (*Stream, error)` opens the bidi stream.
- `session.CloseWithError(code SessionErrorCode, msg string) error` closes the session; `SessionErrorCode` is `uint32`.
- `Stream` satisfies `io.ReadWriter` and has `SetReadDeadline(t time.Time) error`.
- `webtransport.ErrConnUnknownSession`, `quic.ErrServerClosed`, and friends live in the error paths — but simpler: we rely on `IsNormalClose` matching well-known sentinels.

---

## Task 1: Add `DecodeWTMessage` to the `sip/` package

A one-frame decoder mirroring the existing `EncodeWTMessage`. The server reads frames inline via `io.ReadFull`; the client needs the same logic in a helper for readability and test coverage.

**Files:**
- Modify: `sip/protocol.go`
- Modify: `sip/protocol_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `sip/protocol_test.go`:

```go
func TestDecodeWTMessage(t *testing.T) {
	// Normal round-trip.
	encoded := EncodeWTMessage(MsgOutput, []byte("hello"))
	msgType, payload, err := DecodeWTMessage(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != MsgOutput {
		t.Errorf("msgType = %q; want %q", msgType, MsgOutput)
	}
	if string(payload) != "hello" {
		t.Errorf("payload = %q; want %q", payload, "hello")
	}

	// Empty payload (length=1, just type byte).
	encoded = EncodeWTMessage(MsgPing, nil)
	msgType, payload, err = DecodeWTMessage(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != MsgPing {
		t.Errorf("msgType = %q; want MsgPing", msgType)
	}
	if len(payload) != 0 {
		t.Errorf("payload = %v; want empty", payload)
	}
}

func TestDecodeWTMessage_Errors(t *testing.T) {
	cases := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{"too short for length", []byte{0, 0, 0}, "too short"},
		{"length zero", []byte{0, 0, 0, 0}, "zero length"},
		{"body shorter than length", []byte{0, 0, 0, 10, 'a'}, "truncated body"},
		{"oversize length", func() []byte {
			b := []byte{0, 0, 0, 0}
			binary.BigEndian.PutUint32(b, uint32(MaxMessageSize)+1)
			return b
		}(), "exceeds"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := DecodeWTMessage(c.data)
			if err == nil {
				t.Fatalf("want error containing %q, got nil", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %q; want contains %q", err.Error(), c.wantErr)
			}
		})
	}
}
```

Add `"strings"` to the test file's import block if not already there.

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./sip/ -run TestDecodeWTMessage -v
```
Expected: FAIL — `undefined: DecodeWTMessage`.

- [ ] **Step 3: Implement**

Append to `sip/protocol.go`:

```go
// DecodeWTMessage decodes a single WebTransport protocol message from data.
// The expected layout is [4-byte big-endian length][type][payload], where
// length includes the type byte. Returns an error if data is malformed or
// the declared length exceeds MaxMessageSize.
func DecodeWTMessage(data []byte) (msgType byte, payload []byte, err error) {
	if len(data) < 4 {
		return 0, nil, fmt.Errorf("too short for length prefix: %d bytes", len(data))
	}
	length := binary.BigEndian.Uint32(data[:4])
	if length == 0 {
		return 0, nil, fmt.Errorf("zero length message")
	}
	if uint64(length) > uint64(MaxMessageSize) {
		return 0, nil, fmt.Errorf("message length %d exceeds MaxMessageSize %d", length, MaxMessageSize)
	}
	if uint64(len(data)-4) < uint64(length) {
		return 0, nil, fmt.Errorf("truncated body: have %d bytes, need %d", len(data)-4, length)
	}
	body := data[4 : 4+length]
	return body[0], body[1:], nil
}
```

- [ ] **Step 4: Run to confirm pass**

```bash
go test ./sip/ -run TestDecodeWTMessage -v
go test ./sip/ -count=1
```
Expected: every subtest PASSES; full `sip/` package passes.

- [ ] **Step 5: Commit**

```bash
git add sip/protocol.go sip/protocol_test.go
git commit -m "feat(sip): add DecodeWTMessage helper

Mirrors EncodeWTMessage with a single-frame decoder suitable for callers
that have already read a complete [length][type][payload] buffer into
memory. The server reads frames inline via io.ReadFull; this helper
gives the forthcoming WebTransport client path a single canonical
decoder and test coverage for the framing rules."
```

---

## Task 2: Define the `FrameConn` interface

Pure declarations: the interface, the `StatusCode` type, the `IsNormalClose` helper. No implementations yet.

**Files:**
- Create: `internal/sipclient/transport.go`

- [ ] **Step 1: Implement**

Create `internal/sipclient/transport.go`:

```go
package sipclient

import (
	"context"
	"errors"
)

// FrameConn is a transport-agnostic framed connection. Each call to
// ReadFrame returns exactly one decoded [type][payload] message; each call
// to WriteFrame sends exactly one. Implementations wrap WebSocket and
// WebTransport transports, hiding their framing differences from callers.
type FrameConn interface {
	ReadFrame(ctx context.Context) (msgType byte, payload []byte, err error)
	WriteFrame(ctx context.Context, msgType byte, payload []byte) error
	// Close severs the connection cleanly with the given status. Safe to
	// call more than once; subsequent calls are no-ops.
	Close(status StatusCode, reason string) error
	// CloseNow forcibly closes without a close handshake. Safe to use in
	// defer and safe to call more than once.
	CloseNow() error
}

// StatusCode is a transport-agnostic close code. WS implementations map
// directly to websocket.StatusCode values; WT maps to CloseWithError's
// uint32 application code.
type StatusCode uint16

const (
	StatusNormal   StatusCode = 1000
	StatusProtocol StatusCode = 1002
	StatusInternal StatusCode = 1011
)

// errNormalClose is a sentinel used by implementations to signal a
// peer-initiated clean close. IsNormalClose recognizes it and the
// transport-specific equivalents (e.g., websocket.StatusNormalClosure).
var errNormalClose = errors.New("normal close")

// IsNormalClose reports whether err was a clean peer-initiated close. The
// WS implementation returns errors wrapping errNormalClose when it detects
// StatusNormalClosure; the WT implementation does the same for
// CloseWithError(0).
func IsNormalClose(err error) bool {
	return errors.Is(err, errNormalClose)
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/sipclient/
```
Expected: success (the new file has no consumers yet).

- [ ] **Step 3: Commit**

```bash
git add internal/sipclient/transport.go
git commit -m "feat(sip-client): add FrameConn transport interface

Declares the framed transport abstraction used by dump and interactive
modes. WS and WT implementations follow in subsequent commits."
```

---

## Task 3: Implement `wsFrameConn`

Wrap `*websocket.Conn` into the `FrameConn` interface. No call-site changes yet — this task only introduces the wrapper.

**Files:**
- Create: `internal/sipclient/transport_ws.go`
- Create: `internal/sipclient/transport_ws_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/sipclient/transport_ws_test.go`:

```go
package sipclient

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/NimbleMarkets/go-booba/sip"
)

// newWSPair returns a paired (server, client) FrameConn for testing. The
// server handler accepts one connection; the returned pair is ready to read
// and write frames.
func newWSPair(t *testing.T) (server, client FrameConn, cleanup func()) {
	t.Helper()
	srvCh := make(chan *websocket.Conn, 1)
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		srvCh <- conn
		<-r.Context().Done()
	}))
	wsURL := "ws" + strings.TrimPrefix(hs.URL, "http") + "/"
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	clientConn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	cancel()
	if err != nil {
		hs.Close()
		t.Fatalf("dial: %v", err)
	}
	serverConn := <-srvCh
	cleanup = func() {
		_ = clientConn.CloseNow()
		_ = serverConn.CloseNow()
		hs.Close()
	}
	return newWSFrameConn(serverConn), newWSFrameConn(clientConn), cleanup
}

func TestWSFrameConn_RoundTrip(t *testing.T) {
	server, client, cleanup := newWSPair(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cases := []struct {
		name    string
		msgType byte
		payload []byte
	}{
		{"output", sip.MsgOutput, []byte("hello")},
		{"input", sip.MsgInput, []byte("")},
		{"ping", sip.MsgPing, nil},
		{"large", sip.MsgOutput, bytes.Repeat([]byte("x"), 64*1024)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := client.WriteFrame(ctx, c.msgType, c.payload); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}
			typ, payload, err := server.ReadFrame(ctx)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}
			if typ != c.msgType {
				t.Errorf("type = %q; want %q", typ, c.msgType)
			}
			if !bytes.Equal(payload, c.payload) && !(len(payload) == 0 && len(c.payload) == 0) {
				t.Errorf("payload mismatch: got %d bytes, want %d", len(payload), len(c.payload))
			}
		})
	}
}

func TestWSFrameConn_NormalCloseDetected(t *testing.T) {
	server, client, cleanup := newWSPair(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Close(StatusNormal, "bye"); err != nil {
		t.Fatalf("server.Close: %v", err)
	}
	_, _, err := client.ReadFrame(ctx)
	if err == nil {
		t.Fatalf("expected error after peer close")
	}
	if !IsNormalClose(err) {
		t.Errorf("IsNormalClose(%v) = false; want true", err)
	}
}

func TestWSFrameConn_CanceledContext(t *testing.T) {
	_, client, cleanup := newWSPair(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately
	_, _, err := client.ReadFrame(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v; want context.Canceled", err)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/sipclient/ -run TestWSFrameConn -v
```
Expected: FAIL — `undefined: newWSFrameConn`.

- [ ] **Step 3: Implement**

Create `internal/sipclient/transport_ws.go`:

```go
package sipclient

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/websocket"

	"github.com/NimbleMarkets/go-booba/sip"
)

// wsFrameConn wraps a *websocket.Conn into the FrameConn interface.
type wsFrameConn struct {
	conn      *websocket.Conn
	closeOnce sync.Once
}

func newWSFrameConn(conn *websocket.Conn) *wsFrameConn { return &wsFrameConn{conn: conn} }

func (w *wsFrameConn) ReadFrame(ctx context.Context) (byte, []byte, error) {
	_, data, err := w.conn.Read(ctx)
	if err != nil {
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			return 0, nil, fmt.Errorf("%w: %v", errNormalClose, err)
		}
		return 0, nil, err
	}
	msgType, payload, derr := sip.DecodeWSMessage(data)
	if derr != nil {
		return 0, nil, derr
	}
	return msgType, payload, nil
}

func (w *wsFrameConn) WriteFrame(ctx context.Context, msgType byte, payload []byte) error {
	return w.conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(msgType, payload))
}

func (w *wsFrameConn) Close(status StatusCode, reason string) error {
	var err error
	w.closeOnce.Do(func() {
		err = w.conn.Close(websocket.StatusCode(status), reason)
	})
	return err
}

func (w *wsFrameConn) CloseNow() error {
	var err error
	w.closeOnce.Do(func() {
		err = w.conn.CloseNow()
	})
	return err
}

// compile-time assertion
var _ FrameConn = (*wsFrameConn)(nil)
```

`errNormalClose` is declared in `transport.go` (same package, no import needed here). `fmt.Errorf("%w: %v", errNormalClose, err)` uses `%w` to preserve the sentinel in the wrap chain, so the `IsNormalClose` helper from `transport.go` can recognize it via `errors.Is`.

- [ ] **Step 4: Run to confirm pass**

```bash
go test ./internal/sipclient/ -run TestWSFrameConn -v -count=1
go test ./internal/sipclient/ -count=1
```
Expected: all three subtests PASS; full package passes (no call sites yet use the new type, so existing tests are undisturbed).

- [ ] **Step 5: Commit**

```bash
git add internal/sipclient/transport_ws.go internal/sipclient/transport_ws_test.go
git commit -m "feat(sip-client): add wsFrameConn wrapping coder/websocket

Satisfies FrameConn for the existing WebSocket transport. Adds a paired
conformance test suite (round-trip, normal close detection, context
cancellation) that the forthcoming WT implementation will mirror."
```

---

## Task 4: Refactor the WS path to go through `FrameConn`

One atomic commit: change `Dial` to return `FrameConn`; update `RunDump`, `runInteractive`, `RunInteractive`, and every test helper that handles the connection type. All existing behavior preserved.

**Files:**
- Modify: `internal/sipclient/dial.go`
- Modify: `internal/sipclient/dump.go`
- Modify: `internal/sipclient/client.go`
- Modify: `internal/sipclient/client_test.go`
- Modify: `internal/sipclient/client_resize_test.go`

- [ ] **Step 1: Change `Dial` signature and split out `dialWS`**

In `internal/sipclient/dial.go`, replace the current `Dial` function with:

```go
// Dial opens a framed connection to opts.Target, dispatching by scheme.
// Currently only ws/wss are supported; a future commit adds https/WT.
func Dial(ctx context.Context, opts DialOptions) (FrameConn, error) {
	switch opts.Target.Scheme {
	case "ws", "wss":
		return dialWS(ctx, opts)
	default:
		return nil, fmt.Errorf("%w: unsupported scheme %q (want ws or wss)", ErrConnect, opts.Target.Scheme)
	}
}

func dialWS(ctx context.Context, opts DialOptions) (*wsFrameConn, error) {
	headers := opts.Headers.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	origin := opts.Origin
	if origin == "" {
		httpScheme := "http"
		if opts.Target.Scheme == "wss" {
			httpScheme = "https"
		}
		origin = httpScheme + "://" + opts.Target.Host
	}
	headers.Set("Origin", origin)

	httpClient := &http.Client{}
	if opts.Target.Scheme == "wss" {
		httpClient.Transport = &http.Transport{TLSClientConfig: opts.TLS}
	}

	dialCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	conn, _, err := websocket.Dial(dialCtx, opts.Target.String(), &websocket.DialOptions{
		HTTPHeader: headers,
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", opts.Target, err)
	}
	return newWSFrameConn(conn), nil
}
```

- [ ] **Step 2: Update `RunDump` in `dump.go`**

Find the three spots that encode/decode frames directly and replace them with FrameConn calls:

Replace this block (initial resize after dial):
```go
body, err := json.Marshal(sip.ResizeMessage{Cols: 80, Rows: 24})
if err != nil {
    return fmt.Errorf("marshal resize: %w", err)
}
if err := conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgResize, body)); err != nil {
    return fmt.Errorf("%w: send initial resize: %v", ErrTransport, err)
}
```
with:
```go
body, err := json.Marshal(sip.ResizeMessage{Cols: 80, Rows: 24})
if err != nil {
    return fmt.Errorf("marshal resize: %w", err)
}
if err := conn.WriteFrame(ctx, sip.MsgResize, body); err != nil {
    return fmt.Errorf("%w: send initial resize: %v", ErrTransport, err)
}
```

Replace the `--dump-input` send:
```go
if err := conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgInput, data)); err != nil {
    return fmt.Errorf("send dump-input: %w", err)
}
```
with:
```go
if err := conn.WriteFrame(ctx, sip.MsgInput, data); err != nil {
    return fmt.Errorf("send dump-input: %w", err)
}
```

Replace the Pong callback wiring:
```go
Pong: func() error {
    _ = conn.Write(pumpCtx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgPong, nil))
    return nil
},
```
with:
```go
Pong: func() error {
    _ = conn.WriteFrame(pumpCtx, sip.MsgPong, nil)
    return nil
},
```

Replace the read loop:
```go
for {
    _, data, err := conn.Read(pumpCtx)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return nil
        }
        if errors.Is(err, context.Canceled) {
            return nil
        }
        if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
            return nil
        }
        return fmt.Errorf("%w: read frame: %v", ErrTransport, err)
    }
    msgType, payload, derr := sip.DecodeWSMessage(data)
    if derr != nil {
        return fmt.Errorf("%w: decode: %v", ErrProtocol, derr)
    }
    if err := router.Route(msgType, payload); err != nil {
        if errors.Is(err, ErrSessionClosed) {
            return nil
        }
        return fmt.Errorf("%w: %v", ErrProtocol, err)
    }
}
```
with:
```go
for {
    msgType, payload, err := conn.ReadFrame(pumpCtx)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return nil
        }
        if errors.Is(err, context.Canceled) {
            return nil
        }
        if IsNormalClose(err) {
            return nil
        }
        return fmt.Errorf("%w: read frame: %v", ErrTransport, err)
    }
    if err := router.Route(msgType, payload); err != nil {
        if errors.Is(err, ErrSessionClosed) {
            return nil
        }
        return fmt.Errorf("%w: %v", ErrProtocol, err)
    }
}
```

Change the local variable type. Where today we have `conn, err := Dial(...)` followed by `defer conn.CloseNow()`, change to `defer func() { _ = conn.CloseNow() }()` (the FrameConn CloseNow returns an error that errcheck will flag).

Remove unused imports: `github.com/coder/websocket` likely becomes unused in `dump.go` once all direct `websocket.*` calls are replaced. `sip.DecodeWSMessage` becomes unused from this file (moved into `wsFrameConn.ReadFrame`). Confirm with `go build` after the edit — the compiler will fail the build on any unused import.

- [ ] **Step 3: Update `runInteractive` and `RunInteractive` in `client.go`**

Change the `runInteractive` signature from:
```go
func runInteractive(ctx context.Context, conn *websocket.Conn, tty TTY, opts *Options, stderr io.Writer) error {
```
to:
```go
func runInteractive(ctx context.Context, conn FrameConn, tty TTY, opts *Options, stderr io.Writer) error {
```

Replace the frame-level calls inside `runInteractive`:

- The initial resize `sendResize(ctx, conn, cols, rows)` — keep the helper, but change its signature (see below).
- The server→client goroutine's `conn.Read(ctx)` + `DecodeWSMessage` pair — replace with `conn.ReadFrame(ctx)`.
- The client→server pump's `sendInput(ctx, conn, chunk)` — keep the helper, update its signature.
- The Pong callback:
  ```go
  Pong: func() error {
      _ = conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgPong, nil))
      return nil
  },
  ```
  becomes:
  ```go
  Pong: func() error {
      _ = conn.WriteFrame(ctx, sip.MsgPong, nil)
      return nil
  },
  ```

Update the helper functions:

```go
func sendInput(ctx context.Context, conn FrameConn, p []byte) error {
    return conn.WriteFrame(ctx, sip.MsgInput, p)
}

func sendResize(ctx context.Context, conn FrameConn, cols, rows int) error {
    body, err := json.Marshal(sip.ResizeMessage{Cols: cols, Rows: rows})
    if err != nil {
        return err
    }
    return conn.WriteFrame(ctx, sip.MsgResize, body)
}
```

Update the Kitty push-and-send block inside `RunInteractive`:
```go
_ = conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgKittyKbd, body))
```
becomes:
```go
_ = conn.WriteFrame(ctx, sip.MsgKittyKbd, body)
```

Update the final close at the bottom of `RunInteractive`:
```go
_ = conn.Close(websocket.StatusNormalClosure, "")
```
becomes:
```go
_ = conn.Close(StatusNormal, "")
```

Update the server-read goroutine:
```go
_, data, err := conn.Read(ctx)
if err != nil { cancel(err); return }
msgType, payload, derr := sip.DecodeWSMessage(data)
if derr != nil { cancel(derr); return }
```
becomes:
```go
msgType, payload, err := conn.ReadFrame(ctx)
if err != nil { cancel(err); return }
```

Update the trailing cause-classification at the bottom of `runInteractive`. The current check `if websocket.CloseStatus(cause) == websocket.StatusNormalClosure { return nil }` becomes:
```go
if IsNormalClose(cause) { return nil }
```

Remove unused imports after the refactor: `github.com/coder/websocket` likely becomes unused in `client.go`.

- [ ] **Step 4: Update test helpers**

In `internal/sipclient/client_test.go`, find the `dialTest` helper. Change it from returning `*websocket.Conn` to returning `FrameConn`:

```go
func dialTest(t *testing.T, h http.Handler) (FrameConn, func()) {
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
    fc := newWSFrameConn(conn)
    return fc, func() { _ = fc.CloseNow(); hs.Close() }
}
```

The tests that call `dialTest(...)` are already shaped `conn, cleanup := dialTest(t, mux)` — they do not use `conn` directly beyond passing it to `runInteractive`. No further test changes needed.

In `client_resize_test.go`, the existing test dials manually and then passes a raw `*websocket.Conn` to `runInteractive`. Wrap the conn:

```go
// Existing:
// _ = runInteractive(ctx, conn, tty, opts, nopWriter{})
// New:
_ = runInteractive(ctx, newWSFrameConn(conn), tty, opts, nopWriter{})
```

Repeat for every call site in that file.

- [ ] **Step 5: Build and run all tests**

```bash
go build ./...
go test ./... -count=1
```
Expected: success. If an import is unused, remove it.

- [ ] **Step 6: Commit**

```bash
git add internal/sipclient/ sip/
git commit -m "refactor(sip-client): route WS path through FrameConn

RunDump, runInteractive, and their test helpers now depend on the
transport-agnostic FrameConn interface rather than *websocket.Conn
directly. No behavior change — this isolates the pump logic from
transport specifics so the upcoming WebTransport implementation can
plug into the same surface."
```

---

## Task 5: Implement `wtFrameConn`

Wrap `*webtransport.Session` + its bidi stream into the `FrameConn` interface. Add conformance tests matching Task 3's shape.

**Files:**
- Create: `internal/sipclient/transport_wt.go`
- Create: `internal/sipclient/transport_wt_test.go`

- [ ] **Step 1: Promote `webtransport-go` to a direct dep**

```bash
go get github.com/quic-go/webtransport-go@latest
```

Confirm `go.mod` now lists `github.com/quic-go/webtransport-go` in the direct `require` block.

- [ ] **Step 2: Write the failing tests**

Create `internal/sipclient/transport_wt_test.go`:

```go
package sipclient

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/quic-go/webtransport-go"

	"github.com/NimbleMarkets/go-booba/sip"
)

// newWTPair returns a paired (server, client) FrameConn backed by a real
// in-process WebTransport session on a loopback UDP socket.
func newWTPair(t *testing.T) (server, client FrameConn, cleanup func()) {
	t.Helper()
	tlsCert := selfSignedWTCert(t)

	srvCh := make(chan *webtransport.Session, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	wtSrv := &webtransport.Server{
		H3: http3H3Server(tlsCert, mux),
	}
	mux.HandleFunc("/wt", func(w http.ResponseWriter, r *http.Request) {
		sess, err := wtSrv.Upgrade(w, r)
		if err != nil {
			errCh <- err
			return
		}
		srvCh <- sess
	})

	// Bind a QUIC listener on a random UDP port.
	udpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	go func() { _ = wtSrv.ServePacketConn(udpConn) }()

	port := udpConn.LocalAddr().(*net.UDPAddr).Port
	url := "https://127.0.0.1:" + strconv.Itoa(port) + "/wt"

	dialer := webtransport.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"h3"},
		},
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, clientSess, err := dialer.Dial(dialCtx, url, nil)
	cancel()
	if err != nil {
		_ = udpConn.Close()
		t.Fatalf("dial: %v", err)
	}

	clientStream, err := clientSess.OpenStreamSync(context.Background())
	if err != nil {
		_ = clientSess.CloseWithError(1, "stream")
		t.Fatalf("open stream: %v", err)
	}

	var serverSess *webtransport.Session
	select {
	case serverSess = <-srvCh:
	case err := <-errCh:
		t.Fatalf("upgrade error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("server never got session")
	}
	serverStream, err := serverSess.AcceptStream(context.Background())
	if err != nil {
		t.Fatalf("accept stream: %v", err)
	}

	cleanup = func() {
		_ = clientSess.CloseWithError(0, "")
		_ = serverSess.CloseWithError(0, "")
		_ = wtSrv.Close()
		_ = udpConn.Close()
	}
	return newWTFrameConn(serverSess, serverStream), newWTFrameConn(clientSess, clientStream), cleanup
}

func selfSignedWTCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// http3H3Server is a tiny adapter so newWTPair doesn't need to import
// quic-go/http3 directly. The webtransport.Server builds its own H3 server
// when H3 is left nil, but to bind a pre-existing UDP socket we hand it one.
func http3H3Server(cert tls.Certificate, handler http.Handler) *webtransport.Server {
	// Using the zero-value Server and its H3 field's defaults would require
	// the server to open a new UDP socket via ListenAndServe. Our helper
	// supplies a PacketConn via ServePacketConn and lets the library
	// construct the H3 server with the default config.
	return nil
}

func TestWTFrameConn_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("WT test requires UDP loopback; skipping in -short mode")
	}
	server, client, cleanup := newWTPair(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cases := []struct {
		name    string
		msgType byte
		payload []byte
	}{
		{"output", sip.MsgOutput, []byte("hello")},
		{"input", sip.MsgInput, []byte("")},
		{"large", sip.MsgOutput, bytes.Repeat([]byte("x"), 64*1024)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := client.WriteFrame(ctx, c.msgType, c.payload); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}
			typ, payload, err := server.ReadFrame(ctx)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}
			if typ != c.msgType {
				t.Errorf("type = %q; want %q", typ, c.msgType)
			}
			if !bytes.Equal(payload, c.payload) && !(len(payload) == 0 && len(c.payload) == 0) {
				t.Errorf("payload mismatch: got %d bytes, want %d", len(payload), len(c.payload))
			}
		})
	}
}

func TestWTFrameConn_NormalCloseDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("WT test requires UDP loopback; skipping in -short mode")
	}
	server, client, cleanup := newWTPair(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := server.Close(StatusNormal, "bye"); err != nil {
		t.Fatalf("server.Close: %v", err)
	}
	_, _, err := client.ReadFrame(ctx)
	if err == nil {
		t.Fatalf("expected error after peer close")
	}
	if !IsNormalClose(err) {
		t.Errorf("IsNormalClose(%v) = false; want true", err)
	}
}
```

**IMPORTANT:** The `http3H3Server` placeholder in this test file is a stub — the actual test harness needs a live `webtransport.Server` bound to the UDP socket. Implementers working this task should inspect `serve/server.go`'s `newWebTransportServer` and `startWebTransport` functions for the canonical way to construct and bind a `webtransport.Server` against an existing `net.PacketConn`. The `wtSrv.H3` field accepts an `*http3.Server`; you will need to import `github.com/quic-go/quic-go/http3` and construct one with the given TLS cert and mux handler. **Ship this test with a working harness or mark the failing tests `t.Skip("harness not yet wired — see TODO(WT)")`** — do NOT ship a silently-broken test.

- [ ] **Step 3: Run to confirm failure**

```bash
go test ./internal/sipclient/ -run TestWTFrameConn -v
```
Expected: FAIL — `undefined: newWTFrameConn`.

- [ ] **Step 4: Implement**

Create `internal/sipclient/transport_wt.go`:

```go
package sipclient

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/quic-go/webtransport-go"

	"github.com/NimbleMarkets/go-booba/sip"
)

// wtFrameConn wraps a *webtransport.Session plus its single bidirectional
// stream into the FrameConn interface.
type wtFrameConn struct {
	session *webtransport.Session
	stream  *webtransport.Stream

	readMu    sync.Mutex // serializes concurrent ReadFrame callers
	closeOnce sync.Once
	closed    bool
}

func newWTFrameConn(session *webtransport.Session, stream *webtransport.Stream) *wtFrameConn {
	return &wtFrameConn{session: session, stream: stream}
}

func (w *wtFrameConn) ReadFrame(ctx context.Context) (byte, []byte, error) {
	// Enforce ctx by racing the read against ctx.Done().
	done := make(chan struct{})
	var result struct {
		msgType byte
		payload []byte
		err     error
	}
	go func() {
		defer close(done)
		result.msgType, result.payload, result.err = w.readFrame()
	}()
	select {
	case <-done:
		return result.msgType, result.payload, result.err
	case <-ctx.Done():
		// Unblock the background read by canceling the stream read side.
		w.stream.CancelRead(0)
		<-done
		return 0, nil, ctx.Err()
	}
}

func (w *wtFrameConn) readFrame() (byte, []byte, error) {
	w.readMu.Lock()
	defer w.readMu.Unlock()
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(w.stream, lenBuf); err != nil {
		if w.closed {
			return 0, nil, fmt.Errorf("%w: %v", errNormalClose, err)
		}
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return 0, nil, errors.New("zero length message")
	}
	if uint64(length) > uint64(sip.MaxMessageSize) {
		return 0, nil, fmt.Errorf("message length %d exceeds MaxMessageSize %d", length, sip.MaxMessageSize)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(w.stream, body); err != nil {
		return 0, nil, err
	}
	return body[0], body[1:], nil
}

func (w *wtFrameConn) WriteFrame(_ context.Context, msgType byte, payload []byte) error {
	_, err := w.stream.Write(sip.EncodeWTMessage(msgType, payload))
	return err
}

func (w *wtFrameConn) Close(status StatusCode, reason string) error {
	var err error
	w.closeOnce.Do(func() {
		w.closed = true
		err = w.session.CloseWithError(webtransport.SessionErrorCode(status), reason)
	})
	return err
}

func (w *wtFrameConn) CloseNow() error {
	var err error
	w.closeOnce.Do(func() {
		w.closed = true
		err = w.session.CloseWithError(0, "")
	})
	return err
}

// compile-time assertion
var _ FrameConn = (*wtFrameConn)(nil)
```

Note: `WriteFrame` ignores `ctx`. QUIC streams don't expose a per-write context the way coder/websocket does. If `ctx` is canceled, the next `ReadFrame` will catch it; if the caller really needs write-side cancellation, they can `w.stream.CancelWrite(0)` — but none of the existing call sites do this, and adding it now would be premature.

- [ ] **Step 5: Resolve the test harness**

Work the `newWTPair` helper until both tests pass. Specifically: replace the `http3H3Server` stub with code that constructs a real `*http3.Server` referencing the TLS cert and the supplied mux, and assigns it to `wtSrv.H3` before calling `ServePacketConn`. Reference: `serve/server.go:newWebTransportServer` / `startWebTransport`. If this proves time-consuming, gate the two tests behind `t.Skip("TODO(WT harness)")` and open a follow-up; do not commit a broken test.

- [ ] **Step 6: Run to confirm pass**

```bash
go test ./internal/sipclient/ -run TestWTFrameConn -v -count=1
go test ./internal/sipclient/ -count=1
```
Expected: conformance tests PASS (or cleanly SKIP); full package still passes.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/sipclient/transport_wt.go internal/sipclient/transport_wt_test.go
git commit -m "feat(sip-client): add wtFrameConn wrapping webtransport-go

Satisfies FrameConn for the WebTransport transport using a bidirectional
stream. Adds conformance tests paired with a real in-process
webtransport.Server over a loopback UDP socket."
```

---

## Task 6: Wire `https://` scheme into `ParseTargetURL` and `Dial`

**Files:**
- Modify: `internal/sipclient/flags.go`
- Modify: `internal/sipclient/flags_test.go`
- Modify: `internal/sipclient/dial.go`

- [ ] **Step 1: Add failing tests for `https://` URL parsing**

Append to the existing `TestParseTargetURL` table in `internal/sipclient/flags_test.go`:

```go
		{"https with no path defaults to /wt", "https://host:8080", "https://host:8080/wt", ""},
		{"https with custom path preserved", "https://host:8080/custom", "https://host:8080/custom", ""},
		{"http rejected", "http://host:8080", "", "unsupported scheme"},
```

Run:
```bash
go test ./internal/sipclient/ -run TestParseTargetURL -v
```
Expected: FAIL for the three new cases (current code rejects `https`).

- [ ] **Step 2: Update `ParseTargetURL`**

In `internal/sipclient/flags.go`:

```go
switch strings.ToLower(u.Scheme) {
case "ws", "wss", "https":
default:
    return nil, fmt.Errorf("unsupported scheme %q (want ws, wss, or https)", u.Scheme)
}
if u.Host == "" {
    return nil, errors.New("host is required in url")
}
if u.Path == "" {
    if strings.EqualFold(u.Scheme, "https") {
        u.Path = "/wt"
    } else {
        u.Path = "/ws"
    }
}
```

Run:
```bash
go test ./internal/sipclient/ -run TestParseTargetURL -v -count=1
```
Expected: all subtests PASS.

- [ ] **Step 3: Implement `dialWT` and update `Dial` dispatch**

In `internal/sipclient/dial.go`, replace the `Dial` dispatch from Task 4 with:

```go
func Dial(ctx context.Context, opts DialOptions) (FrameConn, error) {
    switch opts.Target.Scheme {
    case "ws", "wss":
        return dialWS(ctx, opts)
    case "https":
        return dialWT(ctx, opts)
    default:
        return nil, fmt.Errorf("%w: unsupported scheme %q (want ws, wss, or https)", ErrConnect, opts.Target.Scheme)
    }
}
```

Append `dialWT` to the same file:

```go
func dialWT(ctx context.Context, opts DialOptions) (*wtFrameConn, error) {
    headers := opts.Headers.Clone()
    if headers == nil {
        headers = http.Header{}
    }
    // Origin handling: WT expects a same-origin hint via the Origin header
    // for some servers, but booba's server honors the --origin check; we
    // set the same computed Origin as the WS path for consistency.
    origin := opts.Origin
    if origin == "" {
        origin = "https://" + opts.Target.Host
    }
    headers.Set("Origin", origin)

    tlsCfg := opts.TLS
    if tlsCfg == nil {
        tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
    } else {
        tlsCfg = tlsCfg.Clone()
    }
    // webtransport-go requires the h3 ALPN; the library does not set it
    // automatically.
    if len(tlsCfg.NextProtos) == 0 {
        tlsCfg.NextProtos = []string{"h3"}
    }

    dialer := webtransport.Dialer{
        TLSClientConfig: tlsCfg,
    }

    dialCtx := ctx
    if opts.Timeout > 0 {
        var cancel context.CancelFunc
        dialCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
        defer cancel()
    }
    _, session, err := dialer.Dial(dialCtx, opts.Target.String(), headers)
    if err != nil {
        return nil, fmt.Errorf("dial %s: %w", opts.Target, err)
    }
    stream, err := session.OpenStreamSync(dialCtx)
    if err != nil {
        _ = session.CloseWithError(1, "open stream failed")
        return nil, fmt.Errorf("open stream %s: %w", opts.Target, err)
    }
    return newWTFrameConn(session, stream), nil
}
```

Add `github.com/quic-go/webtransport-go` to `dial.go`'s import block.

- [ ] **Step 4: Wrap dial errors with `ErrConnect` at the call sites**

In `internal/sipclient/dump.go` and `internal/sipclient/client.go`, find the `Dial(ctx, DialOptions{...})` call. The error returned is already wrapped with `ErrConnect` at the caller level (from the Task 10 error-taxonomy work) — just confirm by reading the code. If not, wrap it:

```go
conn, err := Dial(ctx, DialOptions{...})
if err != nil {
    return fmt.Errorf("%w: %v", ErrConnect, err)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./... -count=1
```
Expected: all tests PASS. The build picks up the new `https://` scheme.

Manual smoke test (requires a running booba server with WT enabled on port 8080 — the default):

```bash
./bin/booba --listen 127.0.0.1:8080 -- sh -c 'printf hello; sleep 1' &
sleep 0.5
./bin/booba-sip-client --dump-frames --dump-timeout 3s --insecure-skip-verify https://127.0.0.1:8080/wt
```
Expected: a JSON frame sequence ending with `{"type":"close"}`.

- [ ] **Step 6: Commit**

```bash
git add internal/sipclient/
git commit -m "feat(sip-client): dial https:// URLs over WebTransport

ParseTargetURL now accepts https:// and defaults the path to /wt. Dial
dispatches ws/wss to dialWS (unchanged) and https to a new dialWT that
constructs a webtransport-go Dialer with the h3 ALPN set and opens a
bidirectional stream immediately after Upgrade."
```

---

## Task 7: Update `--help` text and README for WebTransport

**Files:**
- Modify: `internal/sipclient/root.go`
- Modify: `README.md`

- [ ] **Step 1: Update `--help` Long description**

In `internal/sipclient/root.go`'s `newRootCmd`, replace the existing `Long` string with:

```go
Long: `booba-sip-client connects to a booba server. The URL scheme selects the
transport:

  ws://  / wss://   WebSocket (path defaults to /ws)
  https://          WebTransport (path defaults to /wt)

WebTransport requires the server to speak HTTP/3 over QUIC. Plain-HTTPS
reverse proxies will not work; the server must have HTTP/3 enabled
(default for ` + "`booba`" + ` servers unless HTTP3Port is -1).`,
```

- [ ] **Step 2: Add a WebTransport subsection to README.md**

Find the "Quickstart" or first usage section (wherever `booba-sip-client` is introduced) and append a short subsection:

```markdown
### WebTransport

`booba-sip-client` can dial servers over WebTransport by using an
`https://` URL:

    booba-sip-client https://host:8443/wt

WebTransport uses HTTP/3 over QUIC and offers lower head-of-line-blocking
latency than WebSocket. Requires the server to have HTTP/3 enabled
(`serve.DefaultConfig()` enables it automatically; set `HTTP3Port: -1`
to disable). For self-signed dev certs, use `--insecure-skip-verify`.
```

(Exact heading level and surrounding paragraph shape should follow whatever
convention the existing README uses — inspect it first before pasting.)

- [ ] **Step 3: Verify docs generation still works**

```bash
task build-cmd-booba-sip-client
./bin/booba-sip-client --help
```
Expected: the help output contains the new scheme table and HTTP/3 note.

- [ ] **Step 4: Commit**

```bash
git add internal/sipclient/root.go README.md
git commit -m "docs(sip-client): document WebTransport URL scheme in --help and README

Adds a scheme table to the root command's Long description and a README
subsection explaining when to choose WebTransport and what the server
needs to provide."
```

---

## Task 8: End-to-end test against a real `serve.Server` over WebTransport

Mirror the existing WS e2e test in `cmd/booba-sip-client/e2e_test.go` with a WT variant that exercises the whole stack: real QUIC listener, real `serve.Server`, real `sipclient.RunDump`, real frame decoding.

**Files:**
- Modify: `cmd/booba-sip-client/e2e_test.go`

- [ ] **Step 1: Inspect the existing WT server wiring in `serve/`**

Read `/Users/evan/projects/booba/serve/server.go` lines ~110–250 to understand:
- How `newWebTransportServer()` constructs the `*webtransport.Server`.
- How `startWebTransport(ctx, wtServer)` binds a QUIC listener.
- Where the `HTTP3Port` is read from `Config`.

These define the canonical WT server-side wiring. The test reuses the same flow but drives the ServeHTTP path manually.

- [ ] **Step 2: Add the WT e2e test**

Append to `cmd/booba-sip-client/e2e_test.go`:

```go
func TestE2E_DumpFramesOverWebTransport(t *testing.T) {
    if testing.Short() {
        t.Skip("WT e2e requires UDP loopback; skipping in -short mode")
    }

    // Same hello-session factory as the WS test.
    factory := func(ctx context.Context, size serve.WindowSize) (serve.Session, error) {
        outR, outW := io.Pipe()
        done := make(chan struct{})
        go func() {
            defer close(done)
            defer func() { _ = outW.Close() }()
            _, _ = outW.Write([]byte("hello"))
        }()
        return &helloSession{ctx: ctx, outR: outR, done: done}, nil
    }

    // Find an open UDP port and point the server at it.
    udpListener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
    if err != nil {
        t.Fatalf("listen udp: %v", err)
    }
    port := udpListener.LocalAddr().(*net.UDPAddr).Port
    _ = udpListener.Close() // release; Config.HTTP3Port reopens it

    cfg := serve.DefaultConfig()
    cfg.Port = 0            // disable HTTP/WS listener; not needed for this test
    cfg.HTTP3Port = port
    cfg.Listen = "127.0.0.1"

    srv := serve.NewServer(cfg, serve.WithSessionFactory(factory))

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    serveErrCh := make(chan error, 1)
    go func() { serveErrCh <- srv.Serve(ctx, nil) }() // nil handler = no BubbleTea, session factory handles sessions
    defer cancel()
    time.Sleep(200 * time.Millisecond) // give the server time to bind

    url := "https://127.0.0.1:" + strconv.Itoa(port) + "/wt"
    var stdout, stderr bytes.Buffer
    opts := &sipclient.Options{
        URL:                url,
        EscapeCharRaw:      "^]",
        InsecureSkipVerify: true,
        ConnectTimeout:     5 * time.Second,
        DumpTimeout:        3 * time.Second,
        DumpFrames:         true,
    }
    if err := sipclient.RunDump(ctx, &stdout, &stderr, opts); err != nil {
        t.Fatalf("RunDump: %v (stderr=%s)", err, stderr.String())
    }

    // Confirm an output frame containing "hello" appears.
    sawHello := false
    for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
        if line == "" {
            continue
        }
        var m map[string]any
        if err := json.Unmarshal([]byte(line), &m); err != nil {
            t.Fatalf("bad frame JSON: %v (%q)", err, line)
        }
        if m["type"] == "output" {
            s, _ := m["data"].(string)
            decoded, derr := base64.StdEncoding.DecodeString(s)
            if derr != nil {
                t.Fatalf("bad base64: %v", derr)
            }
            if strings.Contains(string(decoded), "hello") {
                sawHello = true
            }
        }
    }
    if !sawHello {
        t.Errorf("no output frame containing 'hello'. stdout:\n%s", stdout.String())
    }
}
```

Add the needed imports to the test file: `net`, `strconv`. Most others (`bytes`, `context`, `io`, `strings`, `time`, `encoding/base64`, `encoding/json`) should already be there from the WS e2e test.

**Implementer note:** If `srv.Serve(ctx, nil)` is not the right entry point (the WS e2e test uses `srv.HTTPHandler()` inside `httptest.NewServer`), you may need to call `srv.Serve` on a background goroutine with a BubbleTea handler argument, OR extract the WT listener setup and mount it inline. The existing `serve/e2e_test.go` test pattern for WT is the canonical reference — if there isn't one yet, this test is defining the pattern, and that's worth a comment in the test explaining the approach.

- [ ] **Step 3: Run the test**

```bash
go test ./cmd/booba-sip-client/ -run TestE2E_DumpFramesOverWebTransport -v -count=1
```

Expected: PASS. If the test hangs, the most likely cause is `srv.Serve` not binding to the UDP port — inspect the existing WT test pattern in `serve/` for the correct wiring.

- [ ] **Step 4: Commit**

```bash
git add cmd/booba-sip-client/e2e_test.go
git commit -m "test(sip-client): end-to-end test over WebTransport

Drives a real serve.Server over a real QUIC listener with
sipclient.RunDump against an https://...//wt URL. Mirrors the WS e2e
test's shape and assertions so the two transports are held to the same
bar."
```

---

## Self-Review

**Spec coverage:**
- [x] `DecodeWTMessage` helper — Task 1
- [x] `FrameConn` interface, `StatusCode`, `IsNormalClose` — Task 2
- [x] `wsFrameConn` implementation + conformance tests — Task 3
- [x] Refactor WS path through `FrameConn` — Task 4
- [x] `wtFrameConn` implementation + conformance tests — Task 5
- [x] `Dial` dispatch by URL scheme, `ParseTargetURL` accepts `https://` — Task 6
- [x] `--help` and README documentation — Task 7
- [x] WT e2e test — Task 8

**Placeholder scan:**
- Task 5 Step 2 explicitly flags the `http3H3Server` harness stub as needing real wiring or a `t.Skip`. This is an honest acknowledgment of a harness detail the implementer needs to resolve against the webtransport-go API surface — not a handwave.
- Task 7 Step 2 says "exact heading level and surrounding paragraph shape should follow whatever convention the existing README uses — inspect it first" — this is context-dependent formatting, not a placeholder.

**Type consistency:**
- `FrameConn` interface signature (Task 2) matches the method calls in `wsFrameConn` (Task 3), `wtFrameConn` (Task 5), and the refactored call sites (Task 4). Specifically: `ReadFrame(ctx) (byte, []byte, error)`, `WriteFrame(ctx, byte, []byte) error`, `Close(StatusCode, string) error`, `CloseNow() error`.
- `StatusCode` values (1000/1002/1011) are used consistently in Task 4's `conn.Close(StatusNormal, "")` and Task 5's `session.CloseWithError(webtransport.SessionErrorCode(status), reason)`.
- `IsNormalClose` is introduced in Task 2 and used in Tasks 4 and 5.
- `newWSFrameConn` / `newWTFrameConn` are declared in Tasks 3/5 and used in Tasks 4, 6.
- `dialWS` / `dialWT` signatures return `*wsFrameConn` / `*wtFrameConn` (concrete pointers), and `Dial` returns the `FrameConn` interface — Go's interface-satisfaction rule makes the assignment implicit.
- Task 6's error wrapping uses `ErrConnect` and `fmt.Errorf("%w: ...", ErrConnect, err)` consistently with the existing Task 10/C2 sentinel pattern from the prior feature branch.
