# Protocol v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace booba's ad-hoc binary protocol with a Sip-compatible terminal protocol using real PTYs, WebSocket + WebTransport, and ghostty-web as the terminal frontend.

**Architecture:** The Go server creates a Unix PTY per connection, bridges I/O to WebSocket/WebTransport using the `'0'`-`'8'` message protocol, and embeds ghostty-web static assets via `go:embed`. The TypeScript client speaks the same protocol with auto-detection of WebTransport → WebSocket fallback and exponential backoff reconnection.

**Tech Stack:** Go (PTY via `charmbracelet/x/xpty`, WebSocket via `coder/websocket`, WebTransport via `quic-go/webtransport-go`), TypeScript/ES2020, ghostty-web 0.4.0-next

**Reference:** Sip source is available at `sip/` in the repo for comparison. Design spec at `docs/superpowers/specs/2026-04-14-ghostty-protocol-design.md`.

---

## File Structure

### Go — new top-level `serve` package

| File | Responsibility |
|------|----------------|
| `serve/protocol.go` | Message type constants, encode/decode, message structs |
| `serve/protocol_test.go` | Protocol encode/decode tests |
| `serve/session.go` | Session interface, base session lifecycle |
| `serve/pty_unix.go` | Unix PTY creation (master/slave fd pair) |
| `serve/bubbletea.go` | BubbleTea session (wires tea.Program to PTY) |
| `serve/command.go` | Command session (spawns process in PTY) |
| `serve/command_unix.go` | Unix process group / signal handling for commands |
| `serve/handlers.go` | WebSocket + WebTransport message routing, I/O bridge |
| `serve/server.go` | HTTP server, static files, mux, config |
| `serve/cert.go` | Self-signed TLS cert generation for WebTransport |
| `serve/cert_test.go` | Cert generation tests |

### TypeScript — replace adapter layer

| File | Responsibility |
|------|----------------|
| `ts/protocol.ts` | Message type constants, encode/decode (shared by adapters) |
| `ts/websocket_adapter.ts` | WebSocket adapter speaking `'0'`-`'8'` protocol |
| `ts/webtransport_adapter.ts` | WebTransport adapter with length-prefixed framing |
| `ts/auto_adapter.ts` | Auto-detecting adapter (WT → WS fallback) |
| `ts/clipboard.ts` | OSC 52 clipboard write handler |
| `ts/adapter.ts` | Modify: keep BoobaAdapter interface + BoobaWasmAdapter, remove BoobaWebSocketAdapter |
| `ts/booba.ts` | Modify: use new auto adapter, add reconnecting state, wire OSC 52 |

### Static assets for `go:embed`

| File | Responsibility |
|------|----------------|
| `serve/static/index.html` | Embedded HTML page (replaces `assets/index.html` for serve mode) |
| `serve/static/booba/` | Compiled TypeScript + ghostty-web assets (copied at build time) |

### Updated existing files

| File | Change |
|------|--------|
| `go.mod` | Add `coder/websocket`, `quic-go/webtransport-go`, `charmbracelet/x/xpty` |
| `Taskfile.yml` | Add build task for `serve/static/` asset embedding |
| `cmd/booba-view-example/booba-view-example.go` | Update to use `serve` package |

---

## Task 1: Protocol Layer (Go)

**Files:**
- Create: `serve/protocol.go`
- Create: `serve/protocol_test.go`

- [ ] **Step 1: Write protocol_test.go with message encoding tests**

```go
package serve

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestMessageTypes(t *testing.T) {
	// Verify Sip-compatible ASCII values
	if MsgInput != '0' {
		t.Errorf("MsgInput = %d, want %d", MsgInput, '0')
	}
	if MsgClose != '7' {
		t.Errorf("MsgClose = %d, want %d", MsgClose, '7')
	}
	if MsgKittyKbd != '8' {
		t.Errorf("MsgKittyKbd = %d, want %d", MsgKittyKbd, '8')
	}
}

func TestEncodeWebSocketMessage(t *testing.T) {
	payload := []byte("hello")
	msg := EncodeWSMessage(MsgInput, payload)
	if msg[0] != MsgInput {
		t.Errorf("type byte = %d, want %d", msg[0], MsgInput)
	}
	if !bytes.Equal(msg[1:], payload) {
		t.Errorf("payload = %q, want %q", msg[1:], payload)
	}
}

func TestDecodeWebSocketMessage(t *testing.T) {
	raw := append([]byte{MsgOutput}, []byte("world")...)
	msgType, payload, err := DecodeWSMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msgType != MsgOutput {
		t.Errorf("type = %d, want %d", msgType, MsgOutput)
	}
	if !bytes.Equal(payload, []byte("world")) {
		t.Errorf("payload = %q, want %q", payload, "world")
	}
}

func TestDecodeWSMessageEmpty(t *testing.T) {
	_, _, err := DecodeWSMessage([]byte{})
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestEncodeWTMessage(t *testing.T) {
	payload := []byte("data")
	msg := EncodeWTMessage(MsgResize, payload)
	// 4-byte length prefix (includes type byte) + type byte + payload
	length := binary.BigEndian.Uint32(msg[:4])
	if length != uint32(1+len(payload)) {
		t.Errorf("length = %d, want %d", length, 1+len(payload))
	}
	if msg[4] != MsgResize {
		t.Errorf("type = %d, want %d", msg[4], MsgResize)
	}
	if !bytes.Equal(msg[5:], payload) {
		t.Errorf("payload = %q, want %q", msg[5:], payload)
	}
}

func TestResizeMessageJSON(t *testing.T) {
	rm := ResizeMessage{Cols: 80, Rows: 24}
	data, err := json.Marshal(rm)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ResizeMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Cols != 80 || decoded.Rows != 24 {
		t.Errorf("decoded = %+v, want {80, 24}", decoded)
	}
}

func TestOptionsMessageJSON(t *testing.T) {
	om := OptionsMessage{ReadOnly: true}
	data, err := json.Marshal(om)
	if err != nil {
		t.Fatal(err)
	}
	var decoded OptionsMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.ReadOnly {
		t.Error("expected ReadOnly=true")
	}
}

func TestKittyKbdMessageJSON(t *testing.T) {
	km := KittyKbdMessage{Flags: 3}
	data, err := json.Marshal(km)
	if err != nil {
		t.Fatal(err)
	}
	var decoded KittyKbdMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Flags != 3 {
		t.Errorf("flags = %d, want 3", decoded.Flags)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./serve/ -v`
Expected: Compilation errors — package and types don't exist yet.

- [ ] **Step 3: Implement protocol.go**

```go
package serve

import (
	"encoding/binary"
	"fmt"
)

// Message type constants — Sip-compatible ('0'-'7') plus Ghostty extension ('8').
const (
	MsgInput    byte = '0' // Terminal input (client → server)
	MsgOutput   byte = '1' // Terminal output (server → client)
	MsgResize   byte = '2' // Resize terminal (client → server)
	MsgPing     byte = '3' // Keepalive (client → server)
	MsgPong     byte = '4' // Keepalive response (server → client)
	MsgTitle    byte = '5' // Window title (server → client)
	MsgOptions  byte = '6' // Session config (server → client)
	MsgClose    byte = '7' // Session ended (server → client)
	MsgKittyKbd byte = '8' // Kitty keyboard state (bidirectional)
)

// MaxMessageSize is the maximum allowed message size (1MB).
const MaxMessageSize = 1 << 20

// ResizeMessage carries terminal dimensions.
type ResizeMessage struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// OptionsMessage carries session configuration sent on connect.
type OptionsMessage struct {
	ReadOnly bool `json:"readOnly"`
}

// KittyKbdMessage carries Kitty keyboard protocol flag state.
type KittyKbdMessage struct {
	Flags int `json:"flags"`
}

// EncodeWSMessage encodes a WebSocket protocol message: [type][payload].
func EncodeWSMessage(msgType byte, payload []byte) []byte {
	msg := make([]byte, 1+len(payload))
	msg[0] = msgType
	copy(msg[1:], payload)
	return msg
}

// DecodeWSMessage decodes a WebSocket protocol message.
func DecodeWSMessage(data []byte) (msgType byte, payload []byte, err error) {
	if len(data) == 0 {
		return 0, nil, fmt.Errorf("empty message")
	}
	return data[0], data[1:], nil
}

// EncodeWTMessage encodes a WebTransport protocol message:
// [4-byte big-endian length][type][payload].
// Length includes the type byte.
func EncodeWTMessage(msgType byte, payload []byte) []byte {
	bodyLen := 1 + len(payload)
	msg := make([]byte, 4+bodyLen)
	binary.BigEndian.PutUint32(msg[:4], uint32(bodyLen))
	msg[4] = msgType
	copy(msg[5:], payload)
	return msg
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./serve/ -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add serve/protocol.go serve/protocol_test.go
git commit -m "feat(serve): add protocol layer with Sip-compatible message types"
```

---

## Task 2: Session Interface and Unix PTY (Go)

**Files:**
- Create: `serve/session.go`
- Create: `serve/pty_unix.go`

- [ ] **Step 1: Create serve/session.go with Session interface**

```go
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

	// Close cleans up the session.
	Close() error
}
```

- [ ] **Step 2: Create serve/pty_unix.go with Unix PTY creation**

```go
//go:build !windows

package serve

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// ptySession implements Session using a Unix pseudo-terminal pair.
type ptySession struct {
	master  *os.File
	slave   *os.File
	winSize WindowSize
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
}

// newPtySession creates a new PTY session with the given initial size.
func newPtySession(ctx context.Context, size WindowSize) (*ptySession, error) {
	master, slave, err := openPty()
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &ptySession{
		master:  master,
		slave:   slave,
		winSize: size,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	if err := s.Resize(size.Width, size.Height); err != nil {
		master.Close()
		slave.Close()
		cancel()
		return nil, fmt.Errorf("initial resize: %w", err)
	}

	return s, nil
}

func openPty() (master, slave *os.File, err error) {
	masterFd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open ptmx: %w", err)
	}

	if _, err := unix.IoctlGetInt(masterFd, unix.TIOCGPTN); err != nil {
		unix.Close(masterFd)
		return nil, nil, fmt.Errorf("TIOCGPTN: %w", err)
	}

	// Unlock slave
	if err := unix.IoctlSetPointerInt(masterFd, unix.TIOCSPTLCK, 0); err != nil {
		unix.Close(masterFd)
		return nil, nil, fmt.Errorf("TIOCSPTLCK: %w", err)
	}

	// Get slave path
	slavePath, err := unix.IoctlGetPtpeer(masterFd, unix.TIOCGPTPEER)
	if err != nil {
		// Fallback: read ptsname
		ptsNum, _ := unix.IoctlGetInt(masterFd, unix.TIOCGPTN)
		slavePath2 := fmt.Sprintf("/dev/pts/%d", ptsNum)
		slaveFd, err2 := unix.Open(slavePath2, unix.O_RDWR|unix.O_NOCTTY, 0)
		if err2 != nil {
			unix.Close(masterFd)
			return nil, nil, fmt.Errorf("open slave %s: %w", slavePath2, err2)
		}
		master = os.NewFile(uintptr(masterFd), "ptmx")
		slave = os.NewFile(uintptr(slaveFd), slavePath2)
		return master, slave, nil
	}

	master = os.NewFile(uintptr(masterFd), "ptmx")
	slave = os.NewFile(uintptr(slavePath), "pts")
	return master, slave, nil
}

func (s *ptySession) Context() context.Context { return s.ctx }
func (s *ptySession) OutputReader() io.Reader   { return s.master }
func (s *ptySession) InputWriter() io.Writer    { return s.master }
func (s *ptySession) Done() <-chan struct{}      { return s.done }

func (s *ptySession) WindowSize() WindowSize {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.winSize
}

func (s *ptySession) Resize(cols, rows int) error {
	s.mu.Lock()
	s.winSize = WindowSize{Width: cols, Height: rows}
	s.mu.Unlock()

	ws := &unix.Winsize{
		Col: uint16(cols),
		Row: uint16(rows),
	}
	return unix.IoctlSetWinsize(int(s.master.Fd()), unix.TIOCSWINSZ, ws)
}

// Slave returns the slave file descriptor for attaching to a process.
func (s *ptySession) Slave() *os.File {
	return s.slave
}

func (s *ptySession) Close() error {
	s.cancel()
	close(s.done)
	s.slave.Close()
	return s.master.Close()
}
```

**Note:** The PTY opening code above is Linux-specific (`TIOCGPTPEER`). On macOS, the approach differs — `posix_openpt` + `grantpt` + `unlockpt` + `ptsname`. Rather than handling this ourselves, we should use `github.com/charmbracelet/x/xpty` which abstracts this. However, to keep Task 2 self-contained and testable, we implement a minimal version here and can swap to xpty in Task 3. **Actually — let's use xpty from the start.** Revised step 2:

- [ ] **Step 2 (revised): Create serve/pty_unix.go using xpty**

```go
//go:build !windows

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
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
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
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}, nil
}

func (s *ptySession) Context() context.Context { return s.ctx }
func (s *ptySession) OutputReader() io.Reader   { return s.pty }
func (s *ptySession) InputWriter() io.Writer    { return s.pty }
func (s *ptySession) Done() <-chan struct{}      { return s.done }

func (s *ptySession) WindowSize() WindowSize {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.winSize
}

func (s *ptySession) Resize(cols, rows int) {
	s.mu.Lock()
	s.winSize = WindowSize{Width: cols, Height: rows}
	s.mu.Unlock()
	_ = s.pty.Resize(cols, rows)
}

// Pty returns the underlying PTY for attaching to processes.
func (s *ptySession) Pty() xpty.Pty { return s.pty }

func (s *ptySession) Close() error {
	s.cancel()
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return s.pty.Close()
}
```

- [ ] **Step 3: Add xpty dependency**

Run: `go get github.com/charmbracelet/x/xpty@latest`

- [ ] **Step 4: Verify it compiles**

Run: `go build ./serve/`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add serve/session.go serve/pty_unix.go go.mod go.sum
git commit -m "feat(serve): add Session interface and Unix PTY session"
```

---

## Task 3: BubbleTea Session (Go)

**Files:**
- Create: `serve/bubbletea.go`

- [ ] **Step 1: Create serve/bubbletea.go**

This wires a BubbleTea program to a PTY session, similar to Sip's `session.go` + `MakeOptions()`.

```go
package serve

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/xpty"
)

// Handler creates a tea.Model for each new session.
type Handler func(sess Session) tea.Model

// ProgramHandler creates a fully configured tea.Program for each new session.
type ProgramHandler func(sess Session) *tea.Program

// MakeTeaOptions returns tea.ProgramOption values that wire a BubbleTea program
// to the PTY session. Sets TERM=ghostty and COLORTERM=truecolor.
func MakeTeaOptions(sess Session) []tea.ProgramOption {
	ps, ok := sess.(*ptySession)
	if !ok {
		// Fallback for non-PTY sessions
		return []tea.ProgramOption{
			tea.WithInput(sess.OutputReader()),
			tea.WithOutput(sess.InputWriter()),
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
		// Fallback for non-Unix PTY (e.g., ConPty on Windows pipes)
		opts = append(opts,
			tea.WithInput(ps.Pty()),
			tea.WithOutput(ps.Pty()),
		)
	}

	return opts
}

// runBubbleTea starts a BubbleTea program attached to the session PTY.
func runBubbleTea(ctx context.Context, sess *ptySession, handler Handler, extraOpts []tea.ProgramOption) error {
	model := handler(sess)
	opts := MakeTeaOptions(sess)
	opts = append(opts, extraOpts...)

	prog := tea.NewProgram(model, opts...)

	// Send initial window size
	go func() {
		ws := sess.WindowSize()
		prog.Send(tea.WindowSizeMsg{Width: ws.Width, Height: ws.Height})
	}()

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("bubbletea: %w", err)
	}
	return nil
}

// runBubbleTeaProgram starts a pre-configured tea.Program attached to the session PTY.
func runBubbleTeaProgram(ctx context.Context, sess *ptySession, handler ProgramHandler) error {
	prog := handler(sess)

	go func() {
		ws := sess.WindowSize()
		prog.Send(tea.WindowSizeMsg{Width: ws.Width, Height: ws.Height})
	}()

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("bubbletea: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./serve/`
Expected: No errors. (May need to adjust xpty API — check `xpty.UnixPty` type and `Slave()` method.)

- [ ] **Step 3: Commit**

```bash
git add serve/bubbletea.go
git commit -m "feat(serve): add BubbleTea session with PTY integration"
```

---

## Task 4: Command Session (Go)

**Files:**
- Create: `serve/command.go`
- Create: `serve/command_unix.go`

- [ ] **Step 1: Create serve/command.go**

```go
package serve

import (
	"context"
	"fmt"
	"os/exec"
)

// runCommand starts an external command attached to the session PTY.
// The command runs in its own process group.
func runCommand(ctx context.Context, sess *ptySession, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(),
		"TERM=ghostty",
		"COLORTERM=truecolor",
	)

	if err := startCommandInPty(cmd, sess); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	err := cmd.Wait()
	if ctx.Err() != nil {
		// Context cancelled (client disconnected) — not an error
		return nil
	}
	return err
}
```

- [ ] **Step 2: Create serve/command_unix.go**

```go
//go:build !windows

package serve

import (
	"os/exec"
	"syscall"

	"github.com/charmbracelet/x/xpty"
)

func startCommandInPty(cmd *exec.Cmd, sess *ptySession) error {
	upty, ok := sess.Pty().(*xpty.UnixPty)
	if !ok {
		return fmt.Errorf("command mode requires Unix PTY")
	}

	slave := upty.Slave()
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    int(slave.Fd()),
	}

	return cmd.Start()
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./serve/`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add serve/command.go serve/command_unix.go
git commit -m "feat(serve): add command session for wrapping CLI programs"
```

---

## Task 5: TLS Certificate Generation (Go)

**Files:**
- Create: `serve/cert.go`
- Create: `serve/cert_test.go`

- [ ] **Step 1: Write cert_test.go**

```go
package serve

import (
	"crypto/sha256"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	info, err := GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatal(err)
	}
	if info.TLSConfig == nil {
		t.Error("TLSConfig is nil")
	}
	if len(info.DER) == 0 {
		t.Error("DER is empty")
	}
	if info.Hash == (sha256.Sum256(nil)) {
		t.Error("Hash is zero")
	}
	if len(info.TLSConfig.Certificates) == 0 {
		t.Error("no certificates in TLS config")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./serve/ -run TestGenerateSelfSignedCert -v`
Expected: Compilation error — `GenerateSelfSignedCert` doesn't exist.

- [ ] **Step 3: Implement cert.go**

```go
package serve

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// CertInfo holds a self-signed TLS certificate and its metadata.
type CertInfo struct {
	TLSConfig *tls.Config
	DER       []byte     // Raw DER-encoded certificate
	Hash      [32]byte   // SHA-256 hash of DER (for WebTransport pinning)
}

// GenerateSelfSignedCert creates a self-signed ECDSA P-256 certificate.
// Valid for 10 days (Chrome WebTransport requires < 14 days for
// serverCertificateHashes validation).
func GenerateSelfSignedCert(host string) (*CertInfo, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}

	return &CertInfo{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		},
		DER:  der,
		Hash: sha256.Sum256(der),
	}, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./serve/ -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add serve/cert.go serve/cert_test.go
git commit -m "feat(serve): add self-signed TLS cert generation for WebTransport"
```

---

## Task 6: WebSocket and WebTransport Handlers (Go)

**Files:**
- Create: `serve/handlers.go`

This is the I/O bridge between protocol messages and PTY sessions. Reference: `sip/handlers.go`.

- [ ] **Step 1: Create serve/handlers.go**

```go
package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	readBufSize  = 4096
	writeBufSize = 32768
	pingInterval = 30 * time.Second
)

// handleWebSocket handles a single WebSocket connection for a session.
func handleWebSocket(ctx context.Context, conn *websocket.Conn, sess Session, opts OptionsMessage) {
	defer conn.CloseNow()

	// Send options message
	optBytes, _ := json.Marshal(opts)
	writeWSMessage(ctx, conn, MsgOptions, optBytes)

	var wg sync.WaitGroup

	// Stream PTY output → client
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamOutputWS(ctx, conn, sess)
	}()

	// Read client input → PTY
	handleInputWS(ctx, conn, sess)

	wg.Wait()
	conn.Close(websocket.StatusNormalClosure, "session ended")
}

// streamOutputWS reads from PTY and sends as MsgOutput over WebSocket.
func streamOutputWS(ctx context.Context, conn *websocket.Conn, sess Session) {
	buf := make([]byte, writeBufSize)
	for {
		n, err := sess.OutputReader().Read(buf)
		if n > 0 {
			if err := writeWSMessage(ctx, conn, MsgOutput, buf[:n]); err != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("pty read error: %v", err)
			}
			// Send close message
			writeWSMessage(ctx, conn, MsgClose, nil)
			return
		}
	}
}

// handleInputWS reads messages from WebSocket and dispatches them.
func handleInputWS(ctx context.Context, conn *websocket.Conn, sess Session) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		msgType, payload, err := DecodeWSMessage(data)
		if err != nil {
			continue
		}
		processMessage(msgType, payload, conn, sess, ctx)
	}
}

// processMessage dispatches a protocol message.
func processMessage(msgType byte, payload []byte, conn *websocket.Conn, sess Session, ctx context.Context) {
	switch msgType {
	case MsgInput:
		if len(payload) > 0 {
			sess.InputWriter().Write(payload)
		}
	case MsgResize:
		var rm ResizeMessage
		if err := json.Unmarshal(payload, &rm); err == nil && rm.Cols > 0 && rm.Rows > 0 {
			sess.Resize(rm.Cols, rm.Rows)
		}
	case MsgPing:
		writeWSMessage(ctx, conn, MsgPong, nil)
	case MsgKittyKbd:
		// Informational — server can track state if needed
		log.Printf("kitty keyboard flags: %s", payload)
	default:
		// Unknown message types silently ignored (forward compatibility)
	}
}

func writeWSMessage(ctx context.Context, conn *websocket.Conn, msgType byte, payload []byte) error {
	msg := EncodeWSMessage(msgType, payload)
	return conn.Write(ctx, websocket.MessageBinary, msg)
}
```

- [ ] **Step 2: Add coder/websocket dependency**

Run: `go get github.com/coder/websocket@latest`

- [ ] **Step 3: Verify it compiles**

Run: `go build ./serve/`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add serve/handlers.go go.mod go.sum
git commit -m "feat(serve): add WebSocket message handlers and PTY I/O bridge"
```

---

## Task 7: HTTP Server and Config (Go)

**Files:**
- Create: `serve/server.go`
- Create: `serve/config.go`

- [ ] **Step 1: Create serve/config.go**

```go
package serve

import "time"

// Config holds server configuration.
type Config struct {
	Host           string        // Bind address (default "0.0.0.0")
	Port           int           // WebSocket port (default 8080)
	MaxConnections int           // 0 = unlimited
	IdleTimeout    time.Duration // 0 = no timeout
	ReadOnly       bool          // Disable client input
	Debug          bool          // Verbose logging
	TLSCert        string        // Optional TLS cert file path
	TLSKey         string        // Optional TLS key file path
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host: "0.0.0.0",
		Port: 8080,
	}
}
```

- [ ] **Step 2: Create serve/server.go**

```go
package serve

import (
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/coder/websocket"
)

//go:embed static/*
var staticFiles embed.FS

// Server serves terminal sessions over WebSocket and WebTransport.
type Server struct {
	config     Config
	handler    Handler
	progHandler ProgramHandler
	cmdName    string
	cmdArgs    []string
	connCount  atomic.Int32
	certInfo   *CertInfo
}

// NewServer creates a new server with the given config.
func NewServer(config Config) *Server {
	return &Server{config: config}
}

// Serve starts the server with a BubbleTea handler.
func (s *Server) Serve(ctx context.Context, handler Handler) error {
	s.handler = handler
	return s.start(ctx)
}

// ServeWithProgram starts the server with a ProgramHandler.
func (s *Server) ServeWithProgram(ctx context.Context, handler ProgramHandler) error {
	s.progHandler = handler
	return s.start(ctx)
}

// ServeCommand starts the server wrapping an external command.
func (s *Server) ServeCommand(ctx context.Context, name string, args ...string) error {
	s.cmdName = name
	s.cmdArgs = args
	return s.start(ctx)
}

func (s *Server) start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Static files (ghostty-web assets, HTML)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", s.handleIndex)

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWS)

	// Certificate hash endpoint for WebTransport
	if s.certInfo != nil {
		mux.HandleFunc("/cert-hash", s.handleCertHash)
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	logHostName := s.config.Host
	if logHostName == "" {
		logHostName = "localhost"
	}
	log.Printf("Starting server on http://%s:%d", logHostName, s.config.Port)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return server.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Connection limit check
	if s.config.MaxConnections > 0 {
		if int(s.connCount.Load()) >= s.config.MaxConnections {
			http.Error(w, "max connections reached", http.StatusServiceUnavailable)
			return
		}
	}
	s.connCount.Add(1)
	defer s.connCount.Add(-1)

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow all origins for dev
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}
	conn.SetReadLimit(MaxMessageSize)

	ctx := r.Context()

	// Wait for initial resize from client
	_, data, err := conn.Read(ctx)
	if err != nil {
		conn.CloseNow()
		return
	}
	msgType, payload, err := DecodeWSMessage(data)
	if err != nil || msgType != MsgResize {
		conn.CloseNow()
		return
	}
	var rm ResizeMessage
	if err := json.Unmarshal(payload, &rm); err != nil || rm.Cols <= 0 || rm.Rows <= 0 {
		conn.CloseNow()
		return
	}

	// Create PTY session
	sess, err := newPtySession(ctx, WindowSize{Width: rm.Cols, Height: rm.Rows})
	if err != nil {
		log.Printf("create session: %v", err)
		conn.CloseNow()
		return
	}
	defer sess.Close()

	log.Printf("New session: %dx%d", rm.Cols, rm.Rows)

	opts := OptionsMessage{ReadOnly: s.config.ReadOnly}

	// Start the session workload in a goroutine
	go func() {
		defer sess.Close()
		var runErr error
		switch {
		case s.handler != nil:
			runErr = runBubbleTea(ctx, sess, s.handler, nil)
		case s.progHandler != nil:
			runErr = runBubbleTeaProgram(ctx, sess, s.progHandler)
		case s.cmdName != "":
			runErr = runCommand(ctx, sess, s.cmdName, s.cmdArgs...)
		}
		if runErr != nil {
			log.Printf("session error: %v", runErr)
		}
	}()

	// Handle WebSocket protocol messages
	handleWebSocket(ctx, conn, sess, opts)
}

func (s *Server) handleCertHash(w http.ResponseWriter, r *http.Request) {
	if s.certInfo == nil {
		http.Error(w, "no certificate", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"hash": hex.EncodeToString(s.certInfo.Hash[:]),
	})
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./serve/`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add serve/config.go serve/server.go
git commit -m "feat(serve): add HTTP server with WebSocket endpoint and static file serving"
```

---

## Task 8: Protocol Layer (TypeScript)

**Files:**
- Create: `ts/protocol.ts`

- [ ] **Step 1: Create ts/protocol.ts**

```typescript
/**
 * Booba Protocol v2 — Sip-compatible message encoding/decoding.
 *
 * Wire format (WebSocket):  [type_byte][payload...]
 * Wire format (WebTransport): [4-byte big-endian length][type_byte][payload...]
 */

// Message type constants (ASCII, Sip-compatible)
export const MsgInput     = 0x30; // '0'
export const MsgOutput    = 0x31; // '1'
export const MsgResize    = 0x32; // '2'
export const MsgPing      = 0x33; // '3'
export const MsgPong      = 0x34; // '4'
export const MsgTitle     = 0x35; // '5'
export const MsgOptions   = 0x36; // '6'
export const MsgClose     = 0x37; // '7'
export const MsgKittyKbd  = 0x38; // '8'

export interface ResizeMessage {
    cols: number;
    rows: number;
}

export interface OptionsMessage {
    readOnly: boolean;
}

export interface KittyKbdMessage {
    flags: number;
}

/** Encode a WebSocket protocol message: [type][payload] */
export function encodeWSMessage(msgType: number, payload?: Uint8Array | string): Uint8Array {
    const payloadBytes = payload
        ? (typeof payload === 'string' ? new TextEncoder().encode(payload) : payload)
        : new Uint8Array(0);
    const msg = new Uint8Array(1 + payloadBytes.length);
    msg[0] = msgType;
    msg.set(payloadBytes, 1);
    return msg;
}

/** Decode a WebSocket protocol message. Returns [type, payload]. */
export function decodeWSMessage(data: Uint8Array): [number, Uint8Array] {
    if (data.length === 0) throw new Error('empty message');
    return [data[0], data.subarray(1)];
}

/** Encode a WebTransport protocol message: [4-byte length][type][payload] */
export function encodeWTMessage(msgType: number, payload?: Uint8Array | string): Uint8Array {
    const payloadBytes = payload
        ? (typeof payload === 'string' ? new TextEncoder().encode(payload) : payload)
        : new Uint8Array(0);
    const bodyLen = 1 + payloadBytes.length;
    const msg = new Uint8Array(4 + bodyLen);
    new DataView(msg.buffer).setUint32(0, bodyLen, false); // big-endian
    msg[4] = msgType;
    msg.set(payloadBytes, 5);
    return msg;
}

/** Encode a JSON payload as UTF-8 bytes */
export function jsonPayload(obj: unknown): Uint8Array {
    return new TextEncoder().encode(JSON.stringify(obj));
}

/** Decode a UTF-8 JSON payload */
export function parseJsonPayload<T>(data: Uint8Array): T {
    return JSON.parse(new TextDecoder().decode(data));
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add ts/protocol.ts
git commit -m "feat(ts): add protocol v2 message encoding/decoding"
```

---

## Task 9: WebSocket Protocol Adapter (TypeScript)

**Files:**
- Create: `ts/websocket_adapter.ts`
- Modify: `ts/adapter.ts` — remove `BoobaWebSocketAdapter`, keep `BoobaAdapter` interface and `BoobaWasmAdapter`

- [ ] **Step 1: Create ts/websocket_adapter.ts**

```typescript
/**
 * WebSocket adapter speaking the Booba/Sip protocol ('0'-'8' message types).
 */
import { BoobaAdapter, BoobaConnectionState } from './adapter.js';
import {
    MsgInput, MsgOutput, MsgResize, MsgPing, MsgPong,
    MsgTitle, MsgOptions, MsgClose, MsgKittyKbd,
    encodeWSMessage, decodeWSMessage, jsonPayload, parseJsonPayload,
    type ResizeMessage, type OptionsMessage,
} from './protocol.js';

export interface WebSocketAdapterCallbacks {
    onTitle?: (title: string) => void;
    onOptions?: (opts: OptionsMessage) => void;
    onClose?: (reason: string) => void;
}

export class BoobaProtocolAdapter implements BoobaAdapter {
    private ws: WebSocket | null = null;
    private onDataCallback: ((data: string | Uint8Array) => void) | null = null;
    private pingInterval: number | null = null;
    private reconnectAttempts = 0;
    private maxReconnectAttempts = 5;
    private reconnectBaseDelay = 1000;
    private reconnectMultiplier = 1.5;
    private onStateChangeCallback: ((state: BoobaConnectionState, message: string) => void) | null = null;
    private callbacks: WebSocketAdapterCallbacks;
    private shouldReconnect = true;

    constructor(private url: string, callbacks: WebSocketAdapterCallbacks = {}) {
        this.callbacks = callbacks;
    }

    boobaRead(): string | Uint8Array | null {
        return null; // Push-based
    }

    boobaWrite(data: string | Uint8Array): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
        const bytes = typeof data === 'string' ? new TextEncoder().encode(data) : data;
        this.ws.send(encodeWSMessage(MsgInput, bytes));
    }

    boobaResize(cols: number, rows: number): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
        const payload = jsonPayload({ cols, rows } as ResizeMessage);
        this.ws.send(encodeWSMessage(MsgResize, payload));
    }

    connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): void {
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        this.shouldReconnect = true;
        this._connect();
    }

    private _connect(): void {
        this.onStateChangeCallback?.('connecting', 'Connecting...');

        this.ws = new WebSocket(this.url);
        this.ws.binaryType = 'arraybuffer';

        this.ws.onopen = () => {
            this.reconnectAttempts = 0;
            this.onStateChangeCallback?.('connected', 'Connected');
            this._startPing();
        };

        this.ws.onmessage = (e: MessageEvent) => {
            const data = new Uint8Array(e.data as ArrayBuffer);
            const [msgType, payload] = decodeWSMessage(data);

            switch (msgType) {
                case MsgOutput:
                    this.onDataCallback?.(payload);
                    break;
                case MsgPong:
                    // Keepalive acknowledged
                    break;
                case MsgTitle:
                    this.callbacks.onTitle?.(new TextDecoder().decode(payload));
                    break;
                case MsgOptions:
                    this.callbacks.onOptions?.(parseJsonPayload<OptionsMessage>(payload));
                    break;
                case MsgClose:
                    this.shouldReconnect = false;
                    const reason = payload.length > 0 ? new TextDecoder().decode(payload) : 'Session ended';
                    this.callbacks.onClose?.(reason);
                    break;
                default:
                    // Unknown message types silently ignored
                    break;
            }
        };

        this.ws.onclose = () => {
            this._stopPing();
            if (this.shouldReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
                this._reconnect();
            } else {
                this.onStateChangeCallback?.('disconnected', 'Disconnected');
            }
        };

        this.ws.onerror = () => {
            // onclose will fire after this
        };
    }

    private _reconnect(): void {
        this.reconnectAttempts++;
        const delay = this.reconnectBaseDelay * Math.pow(this.reconnectMultiplier, this.reconnectAttempts - 1);
        this.onStateChangeCallback?.('reconnecting', `Reconnecting (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);
        setTimeout(() => this._connect(), delay);
    }

    private _startPing(): void {
        this.pingInterval = window.setInterval(() => {
            if (this.ws?.readyState === WebSocket.OPEN) {
                this.ws.send(encodeWSMessage(MsgPing));
            }
        }, 30000);
    }

    private _stopPing(): void {
        if (this.pingInterval !== null) {
            clearInterval(this.pingInterval);
            this.pingInterval = null;
        }
    }

    disconnect(): void {
        this.shouldReconnect = false;
        this._stopPing();
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.onDataCallback = null;
    }
}
```

- [ ] **Step 2: Modify ts/adapter.ts — remove BoobaWebSocketAdapter**

Remove the `BoobaWebSocketAdapter` class entirely from `ts/adapter.ts`. Keep the `BoobaAdapter` interface, `BoobaConnectionState` type, and `BoobaWasmAdapter` class. Update the connection state type:

```typescript
export type BoobaConnectionState = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add ts/websocket_adapter.ts ts/adapter.ts
git commit -m "feat(ts): add protocol v2 WebSocket adapter with reconnection"
```

---

## Task 10: OSC 52 Clipboard Handler (TypeScript)

**Files:**
- Create: `ts/clipboard.ts`

- [ ] **Step 1: Create ts/clipboard.ts**

```typescript
/**
 * OSC 52 clipboard handler.
 *
 * Intercepts OSC 52 escape sequences from terminal output and writes
 * the decoded content to the system clipboard.
 *
 * OSC 52 format: \x1b]52;c;<base64-data>\a  (or \x1b\\ as terminator)
 */

/**
 * Install an OSC 52 handler on a ghostty-web Terminal instance.
 * Watches terminal output for OSC 52 sequences and copies decoded
 * content to the system clipboard.
 *
 * @param term The ghostty-web Terminal instance (typed as any since
 *             ghostty-web doesn't export granular types)
 * @returns A dispose function to remove the handler
 */
export function installOSC52Handler(term: any): () => void {
    let buffer = '';
    let inOSC52 = false;

    // Hook into the terminal's write path by wrapping onRender.
    // We parse OSC 52 from the raw output data instead, by
    // intercepting writes before they reach the terminal.
    //
    // Actually, ghostty-web processes escape sequences internally
    // in WASM — we can't intercept them from JS. Instead, we scan
    // the raw data stream before it reaches term.write().
    //
    // The caller should wrap their data callback to pass through
    // this scanner before calling term.write().

    // This is a no-op placeholder. The actual integration happens
    // in booba.ts where we intercept the adapter's onData callback.
    return () => {};
}

// OSC 52 sequence parser states
const OSC_START = '\x1b]52;';
const ST_BEL = '\x07';
const ST_ESC = '\x1b\\';

/**
 * Scan a chunk of terminal output data for OSC 52 sequences.
 * Returns the data unchanged (for passing to term.write) and
 * asynchronously copies any OSC 52 payloads to the clipboard.
 *
 * This is designed to be called on every output chunk. It handles
 * sequences that span multiple chunks via internal buffering.
 */
export class OSC52Scanner {
    private buffer = '';

    /** Process a chunk of output. Extracts and handles OSC 52, returns data as-is. */
    scan(data: Uint8Array): void {
        // Convert to string for sequence scanning
        const text = new TextDecoder().decode(data);
        this.buffer += text;

        // Look for complete OSC 52 sequences
        let startIdx: number;
        while ((startIdx = this.buffer.indexOf(OSC_START)) !== -1) {
            // Find the terminator (BEL or ST)
            const afterStart = startIdx + OSC_START.length;
            let endIdx = this.buffer.indexOf(ST_BEL, afterStart);
            let endLen = 1;
            if (endIdx === -1) {
                endIdx = this.buffer.indexOf(ST_ESC, afterStart);
                endLen = 2;
            }
            if (endIdx === -1) {
                // Incomplete sequence — keep buffering
                // Only keep from the start of the OSC sequence
                this.buffer = this.buffer.substring(startIdx);
                return;
            }

            // Extract the base64 payload (after "c;" selection parameter)
            const payload = this.buffer.substring(afterStart, endIdx);
            // payload format: "c;<base64>" where c is the selection clipboard
            const semiIdx = payload.indexOf(';');
            if (semiIdx !== -1) {
                const base64Data = payload.substring(semiIdx + 1);
                if (base64Data.length > 0) {
                    try {
                        const decoded = atob(base64Data);
                        navigator.clipboard.writeText(decoded).catch(() => {
                            // Clipboard write failed (no user gesture, permissions, etc.)
                        });
                    } catch {
                        // Invalid base64
                    }
                }
            }

            // Remove the processed sequence from the buffer
            this.buffer = this.buffer.substring(endIdx + endLen);
        }

        // If no OSC 52 start found, clear the buffer (no partial sequence)
        if (this.buffer.indexOf('\x1b]') === -1) {
            this.buffer = '';
        }
    }
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add ts/clipboard.ts
git commit -m "feat(ts): add OSC 52 clipboard write handler"
```

---

## Task 11: Update BoobaTerminal to Use New Protocol (TypeScript)

**Files:**
- Modify: `ts/booba.ts`

- [ ] **Step 1: Update imports and adapter wiring in booba.ts**

Replace the import of `BoobaWebSocketAdapter` with the new protocol adapter. Update `connectWebSocket` to use `BoobaProtocolAdapter`. Wire up OSC 52 scanner. Add `connectAuto` method.

Key changes to `ts/booba.ts`:

1. Update imports:
```typescript
import { BoobaAdapter, BoobaConnectionState, BoobaWasmAdapter } from './adapter.js';
import { BoobaProtocolAdapter, type WebSocketAdapterCallbacks } from './websocket_adapter.js';
import { OSC52Scanner } from './clipboard.js';
```

2. Add `osc52Scanner` property:
```typescript
private osc52Scanner: OSC52Scanner = new OSC52Scanner();
```

3. Update `connectWebSocket` method:
```typescript
connectWebSocket(url: string) {
    const callbacks: WebSocketAdapterCallbacks = {
        onTitle: (title) => { this.onTitleChange?.(title); },
        onOptions: (opts) => { /* store readOnly state if needed */ },
        onClose: (reason) => {
            this.term?.write(`\r\n${reason}\r\n`);
        },
    };
    this.adapter = new BoobaProtocolAdapter(url, callbacks);
    this._setupAdapter();
}
```

4. Update `_setupAdapter` to pipe output through OSC 52 scanner:
```typescript
private _setupAdapter() {
    if (!this.adapter) return;
    this.adapter.connect(
        (data: string | Uint8Array) => {
            // Scan for OSC 52 clipboard sequences
            if (data instanceof Uint8Array) {
                this.osc52Scanner.scan(data);
            }
            this.term.write(data);
        },
        (state: BoobaConnectionState, message: string) => {
            this._updateStatus(state, message);
            if (state === 'connected' && this.term) {
                this.adapter?.boobaResize(this.term.cols, this.term.rows);
            }
            if (state === 'disconnected') {
                this.term.write('\r\nConnection closed.\r\n');
            }
        }
    );
}
```

5. Update re-exports at bottom:
```typescript
export { BoobaAdapter, BoobaWasmAdapter, BoobaConnectionState };
export { BoobaProtocolAdapter } from './websocket_adapter.js';
export { OSC52Scanner } from './clipboard.js';
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat(ts): wire BoobaTerminal to protocol v2 adapter with OSC 52"
```

---

## Task 12: Embedded Static Assets and Build Integration

**Files:**
- Create: `serve/static/index.html`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Create serve/static/index.html**

This is the embedded HTML page served by the Go server. It loads ghostty-web and the compiled booba TypeScript from `/static/`.

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>booba</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #1e1e1e;
            min-height: 100vh;
            display: flex;
            align-items: stretch;
        }
        #terminal-container {
            flex: 1;
            padding: 4px;
        }
        .status {
            position: fixed;
            top: 8px;
            right: 12px;
            font-family: monospace;
            font-size: 12px;
            color: #888;
            display: flex;
            align-items: center;
            gap: 6px;
            z-index: 10;
        }
        .status-dot {
            width: 6px;
            height: 6px;
            border-radius: 50%;
            background: #666;
            transition: background-color 0.3s;
        }
        .status-dot.connected { background: #27c93f; }
        .status-dot.disconnected { background: #dc3545; }
        .status-dot.connecting, .status-dot.reconnecting { background: #e6b800; }
    </style>
</head>
<body>
    <div class="status">
        <span class="status-dot connecting" id="status-dot"></span>
        <span id="status-text">Connecting...</span>
    </div>
    <div id="terminal-container"></div>

    <script type="module">
        import { BoobaTerminal } from './booba/booba.js';

        const dot = document.getElementById('status-dot');
        const text = document.getElementById('status-text');

        async function main() {
            const booba = new BoobaTerminal('terminal-container');

            booba.onStatusChange = (state, message) => {
                dot.className = `status-dot ${state}`;
                text.textContent = message;
            };

            booba.onTitleChange = (title) => {
                document.title = title || 'booba';
            };

            booba.onBell = () => {
                const c = document.getElementById('terminal-container');
                c.style.outline = '2px solid #ffbd2e';
                setTimeout(() => { c.style.outline = 'none'; }, 150);
            };

            await booba.init();

            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws`;
            booba.connectWebSocket(wsUrl);
            booba.focus();
        }

        main().catch(console.error);
    </script>
</body>
</html>
```

- [ ] **Step 2: Add build task for embedding assets**

Add to `Taskfile.yml`:

```yaml
  build-serve-assets:
    desc: 'Copy compiled TS + ghostty-web into serve/static/ for go:embed'
    deps: [build-assets]
    cmds:
      - mkdir -p serve/static/booba
      - cp -r assets/booba/* serve/static/booba/
      - mkdir -p serve/static/ghostty-web
      - cp -r node_modules/ghostty-web/dist/* serve/static/ghostty-web/
    sources:
      - assets/booba/*.js
      - node_modules/ghostty-web/dist/*
    generates:
      - serve/static/booba/*.js
      - serve/static/ghostty-web/*
```

Update the `build` task deps to include `build-serve-assets`.

- [ ] **Step 3: Run the build task**

Run: `task build-serve-assets`
Expected: `serve/static/booba/` and `serve/static/ghostty-web/` populated.

- [ ] **Step 4: Verify Go embedding compiles**

Run: `go build ./serve/`
Expected: No errors.

- [ ] **Step 5: Add serve/static/ to .gitignore** (it's generated)

Add to `.gitignore`:
```
serve/static/booba/
serve/static/ghostty-web/
```

- [ ] **Step 6: Commit**

```bash
git add serve/static/index.html Taskfile.yml .gitignore
git commit -m "feat(serve): add embedded HTML page and build task for static assets"
```

---

## Task 13: Update Example App

**Files:**
- Modify: `cmd/booba-view-example/booba-view-example.go`

- [ ] **Step 1: Update the example to use the new serve package**

Replace the `startWebServer` function to use the new `serve.Server`:

```go
func startWebServer(addr string) {
	config := serve.Config{
		Host: "",
		Port: 8080,
	}

	// Parse addr to extract host:port
	if host, port, err := net.SplitHostPort(addr); err == nil {
		config.Host = host
		p, _ := strconv.Atoi(port)
		config.Port = p
	}

	server := serve.NewServer(config)

	ctx := context.Background()
	if err := server.Serve(ctx, func(sess serve.Session) tea.Model {
		return model{0, false, 3600, 0, 0, false, false}
	}); err != nil {
		log.Fatal("Server error:", err)
	}
}
```

Update imports to include `"context"`, `"net"`, `"strconv"`, and `"github.com/NimbleMarkets/go-booba/serve"`. Remove the import of `"github.com/NimbleMarkets/go-booba/internal/booba_server"`.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/booba-view-example/`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/booba-view-example/booba-view-example.go
git commit -m "feat: update example app to use serve package with protocol v2"
```

---

## Task 14: Integration Test — End to End

**Files:**
- No new files — manual verification

- [ ] **Step 1: Full build**

Run: `task build`
Expected: All tasks succeed — TypeScript compiled, assets copied, Go binaries built.

- [ ] **Step 2: Run the example**

Run: `./bin/booba-view-example -web :8080`
Open: `http://localhost:8080`

Expected:
- Terminal renders correctly (no staircase, no LNM hack needed)
- Keyboard input works (arrow keys navigate menu)
- Window resize works (resize browser, terminal reflows)
- Connection status indicator shows "Connected"
- Server logs show `New session: NxM` with correct dimensions

- [ ] **Step 3: Test reconnection**

1. Stop and restart the server while the browser page is open
2. Expected: status shows "Reconnecting", then "Connected" after server comes back

- [ ] **Step 4: Test command mode (manual)**

Temporarily modify the example or create a quick test:
```go
server.ServeCommand(ctx, "bash")
```
Expected: Full interactive shell in the browser.

- [ ] **Step 5: Commit any fixes discovered during integration**

```bash
git add -A
git commit -m "fix: integration fixes from end-to-end testing"
```

---

## Deferred to Follow-Up Plan

**WebTransport**: The spec calls for WebTransport alongside WebSocket. This requires `quic-go/webtransport-go`, a separate TLS listener on port+1, bidirectional QUIC streams with length-prefixed framing, and a TypeScript `BoobaWebTransportAdapter`. The protocol layer (Tasks 1, 8) already includes WT encoding functions (`EncodeWTMessage`). The remaining work is:
- Go: WebTransport listener and handler in `serve/handlers.go` and `serve/server.go`
- TS: `ts/webtransport_adapter.ts` and `ts/auto_adapter.ts` (auto-detect WT → WS fallback)

This is deferred because: (1) WebSocket alone is fully functional, (2) WebTransport requires TLS which adds dev friction, (3) it can be added without changing any existing code — just new handlers and a new adapter class.

---

## Summary

| Task | What it builds | Key files |
|------|---------------|-----------|
| 1 | Protocol layer (Go) | `serve/protocol.go`, tests |
| 2 | Session + Unix PTY | `serve/session.go`, `serve/pty_unix.go` |
| 3 | BubbleTea session | `serve/bubbletea.go` |
| 4 | Command session | `serve/command.go`, `serve/command_unix.go` |
| 5 | TLS cert generation | `serve/cert.go`, tests |
| 6 | WebSocket handlers | `serve/handlers.go` |
| 7 | HTTP server + config | `serve/server.go`, `serve/config.go` |
| 8 | Protocol layer (TS) | `ts/protocol.ts` |
| 9 | WebSocket adapter (TS) | `ts/websocket_adapter.ts` |
| 10 | OSC 52 clipboard | `ts/clipboard.ts` |
| 11 | Wire BoobaTerminal | `ts/booba.ts` |
| 12 | Embedded assets + build | `serve/static/index.html`, Taskfile |
| 13 | Update example app | `cmd/booba-view-example/` |
| 14 | Integration testing | Manual verification |
