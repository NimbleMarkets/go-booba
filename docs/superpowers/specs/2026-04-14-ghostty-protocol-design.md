# Booba Protocol v2: Ghostty-Centric Terminal Protocol

## Goal

Replace booba's current ad-hoc binary protocol with a Sip-compatible terminal protocol that uses real PTYs, supports WebSocket and WebTransport, and is designed around ghostty-web as the terminal frontend. This enables both BubbleTea library mode and CLI command mode (wrapping arbitrary programs like `htop`, `vim`, `bash`).

## Design Principles

- **Sip-compatible**: Message types `'0'`-`'7'` match Sip's wire format exactly. A booba ghostty-web frontend can connect to a Sip server, and a Sip xterm.js frontend can connect to a booba server (minus Ghostty-specific extensions).
- **Ghostty-native**: Sets `TERM=ghostty` on PTYs and leverages Ghostty's native capabilities (Kitty keyboard protocol, OSC 8 hyperlinks) via standard terminal escape sequence negotiation — not custom protocol messages.
- **PTY-based**: Every session gets a real pseudo-terminal, eliminating the need for LNM hacks, TERM workarounds, and newline mapping fixes.
- **Transport-agnostic**: Same protocol over WebSocket and WebTransport, with auto-detection and fallback.

## Wire Format

### WebSocket

```
[type_byte][payload...]
```

Single binary WebSocket frame per message. Type byte is ASCII `'0'`-`'8'`. Payload is raw bytes (input/output) or UTF-8 JSON (structured messages). Max message size: 1MB.

### WebTransport

```
[4-byte big-endian length][type_byte][payload...]
```

Length field includes the type byte. Uses a single bidirectional stream per session. Same 1MB max message size.

## Message Types

### Sip-Compatible Messages (`'0'`-`'7'`)

| Type | Name | Direction | Payload | Description |
|------|------|-----------|---------|-------------|
| `'0'` | Input | client → server | Raw bytes | Terminal input (keystrokes, paste, mouse escape sequences) |
| `'1'` | Output | server → client | Raw bytes | Terminal output (program output, escape sequences) |
| `'2'` | Resize | client → server | `{"cols":N,"rows":M}` | Terminal dimensions changed. Client sends with 50ms debounce. |
| `'3'` | Ping | client → server | Empty | Keepalive, sent every 30 seconds |
| `'4'` | Pong | server → client | Empty | Keepalive response |
| `'5'` | Title | server → client | UTF-8 string | Window title (from OSC 0/1/2 sequences) |
| `'6'` | Options | server → client | `{"readOnly":bool}` | Session configuration, sent on connect |
| `'7'` | Close | server → client | UTF-8 string (optional) | Session ended. Payload is optional reason. |

### Ghostty Extension (`'8'`)

| Type | Name | Direction | Payload | Description |
|------|------|-----------|---------|-------------|
| `'8'` | KittyKbd | client → server | `{"flags":N}` | Kitty keyboard protocol flag state change |

The Kitty keyboard protocol flags are set by the backend program via escape sequences (`\x1b[>Nm`). ghostty-web processes these natively. The client sends `'8'` to inform the server of the current mode, enabling the server to track what key encoding the client is using. Non-Ghostty clients silently ignore `'8'` messages (forward-compatible).

### Forward Compatibility

Unknown message types are silently ignored by both client and server. This allows future extensions without breaking existing implementations.

## Connection Lifecycle

### Initial Handshake

```
Client                          Server
  |                               |
  |  [connect WebSocket/WT]       |
  |------------------------------>|
  |                               |
  |  '2' Resize {cols, rows}      |  Client sends actual terminal dimensions
  |------------------------------>|
  |                               |  Server creates PTY with these dimensions
  |  '6' Options {readOnly}       |  Server sends session config
  |<------------------------------|
  |                               |
  |  '1' Output (program output)  |  Server starts streaming PTY output
  |<------------------------------|
  |                               |
  |  '0' Input (keystrokes)       |  Client sends user input
  |------------------------------>|
  |                               |
  |  '3' Ping (every 30s)         |  Client keepalive
  |------------------------------>|
  |  '4' Pong                     |  Server responds
  |<------------------------------|
```

### Transport Selection

The client attempts WebTransport first (lower latency via QUIC), falling back to WebSocket if unavailable. WebTransport requires TLS.

```
1. Try WebTransport on port P+1 (requires TLS)
2. If failed or unsupported, use WebSocket on port P
```

The server exposes a `/cert-hash` endpoint returning the self-signed certificate hash for WebTransport pinning.

### Reconnection

On unexpected disconnect, the client reconnects with exponential backoff:
- Base delay: 1 second
- Multiplier: 1.5x per attempt
- Max attempts: 5
- Connection status exposed via `onStatusChange` callback with states: `connecting`, `connected`, `disconnected`, `reconnecting`

### Session Teardown

- Client disconnects: server kills PTY, cleans up session
- Server sends `'7'` (Close): client shows disconnect message, may attempt reconnect
- PTY process exits: server sends `'7'` (Close) with exit status

## Server Architecture

### Session Modes

**BubbleTea mode (library):**
```go
server := booba.NewServer(booba.Config{
    Host: "0.0.0.0",
    Port: 8080,
})
server.Serve(func(session booba.Session) tea.Model {
    return myModel{width: session.WindowSize().Cols}
})
```

The handler receives a `Session` with the PTY, window size, and context. BubbleTea runs attached to the PTY slave fd with `tea.WithInput` and `tea.WithOutput` pointing at the PTY.

**Command mode (CLI):**
```go
server.ServeCommand("htop", "--delay=10")
```

Or from the command line:
```bash
booba serve -- htop --delay=10
```

Each connection spawns the command in its own PTY. Process lifecycle is tied to the WebSocket/WebTransport connection.

### PTY Management

Each session gets a Unix pseudo-terminal pair:
- **Master fd**: Read by the WebSocket/WebTransport handler, writes to the client as `'1'` (Output). Client input (`'0'`) is written to the master.
- **Slave fd**: Attached to the BubbleTea program or spawned command as stdin/stdout/stderr.

PTY environment:
- `TERM=ghostty` (falls back to `xterm-256color` if ghostty terminfo not installed)
- `COLORTERM=truecolor`
- `LANG` inherited from server process

Resize (`'2'`) updates PTY window size via `ioctl(TIOCSWINSZ)`, which sends `SIGWINCH` to the foreground process.

### Server Configuration

```go
type Config struct {
    Host           string        // Bind address (default "0.0.0.0")
    Port           int           // WebSocket port (default 8080)
    MaxConnections int           // 0 = unlimited
    IdleTimeout    time.Duration // 0 = no timeout
    ReadOnly       bool          // Disable client input
    Debug          bool          // Verbose logging
    TLSCert        string        // Optional TLS cert path
    TLSKey         string        // Optional TLS key path
}
```

WebTransport listens on `Port + 1` when TLS is configured or using auto-generated self-signed certs.

### Static Assets

All frontend assets (ghostty-web WASM, JS, HTML, CSS) are embedded via `go:embed` for a self-contained binary. No symlinks, no npm at runtime.

## Client Architecture

### TypeScript Adapters

The adapter layer is replaced with two new implementations:

**`BoobaProtocolAdapter`** (WebSocket):
- Speaks `'0'`-`'8'` message protocol
- Manages ping/pong keepalive
- Handles reconnection with exponential backoff
- Dispatches title, options, and close messages to callbacks

**`BoobaWebTransportAdapter`** (WebTransport):
- Same protocol with length-prefixed framing
- Requires TLS certificate hash from `/cert-hash`
- Falls back to WebSocket on failure

**`BoobaAutoAdapter`** (default):
- Tries WebTransport first, falls back to WebSocket
- Transparent to the `BoobaTerminal` consumer

### OSC 52 Clipboard Support

When the backend program sends an OSC 52 sequence (`\x1b]52;c;<base64-data>\a`), ghostty-web's VT parser processes it. The client hooks into the terminal's output to detect OSC 52 and calls `navigator.clipboard.writeText()` with the decoded payload.

For paste (clipboard read), the existing `BoobaTerminal.paste()` method and bracketed paste handling remain unchanged — the user pastes via Cmd+V / Ctrl+V, which flows through ghostty-web's input handler as regular input.

### Connection Status

The existing `onStatusChange` callback gains a new state:

```typescript
type BoobaConnectionState = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';
```

## Capability Negotiation

There is no custom capability negotiation in the protocol. The server sets `TERM=ghostty` on the PTY, and backend programs discover terminal capabilities through standard escape sequence queries:

- **DECRQM** (`\x1b[?Nm$p`) — query DEC private modes
- **DSR** (`\x1b[6n`) — cursor position report
- **DA1/DA2/DA3** — device attributes
- **XTVERSION** (`\x1b[>q`) — terminal version

ghostty-web responds to these queries natively. BubbleTea uses this mechanism to detect and enable Kitty keyboard protocol, synchronized output, and other features.

The `'8'` (KittyKbd) message is sent by the client to keep the server informed of the current Kitty keyboard mode state. This is informational — the server doesn't negotiate modes, it just tracks what the client has enabled so it can correctly interpret input encoding.

## Interoperability

### With Sip

- A booba client connecting to a Sip server: works for `'0'`-`'7'` messages. `'8'` messages are silently ignored by Sip. Terminal renders with ghostty-web instead of xterm.js.
- A Sip client connecting to a booba server: works for `'0'`-`'7'` messages. xterm.js won't enable Kitty keyboard protocol, so `'8'` is never sent. Terminal renders with xterm.js instead of ghostty-web.

### TERM Considerations

If `TERM=ghostty` terminfo is not installed on the system, the server falls back to `TERM=xterm-256color`. Backend programs that check `TERM` will use xterm escape sequences, which ghostty-web handles correctly (it's a superset).

## File Structure

| File | Responsibility |
|------|----------------|
| `protocol.go` | Message type constants, encode/decode functions, `ResizeMessage`/`OptionsMessage`/`KittyKbdMessage` types |
| `server.go` | HTTP server, static file serving, WebSocket/WebTransport endpoints, TLS cert management |
| `session.go` | Session interface, PTY lifecycle, BubbleTea session |
| `session_unix.go` | Unix PTY creation (master/slave fd pair) |
| `cmd_session.go` | Command mode session (spawn process in PTY) |
| `cmd_unix.go` | Unix command PTY with process group and signal handling |
| `config.go` | Server configuration, defaults |
| `handlers.go` | WebSocket/WebTransport message routing, I/O bridging |
| `cert.go` | Self-signed certificate generation for WebTransport |
| `ts/adapter.ts` | Protocol adapter (WebSocket), reconnection logic |
| `ts/webtransport.ts` | WebTransport adapter with length-prefixed framing |
| `ts/auto.ts` | Auto-detecting adapter (WebTransport → WebSocket fallback) |
| `ts/booba.ts` | BoobaTerminal (mostly unchanged, uses new adapters) |
| `ts/clipboard.ts` | OSC 52 clipboard handler |

## Out of Scope

- **Windows support**: PTY implementation is Unix-only (master/slave fd pair). Windows ConPty support (like Sip's `session_windows.go`) can be added later.
- **WASM mode**: The `BoobaWasmAdapter` communicates via global JS functions (`bubbletea_write`/`bubbletea_read`), not WebSocket. It is unaffected by this protocol change and continues to work as-is.
- **Sixel / Kitty graphics**: ghostty-web doesn't support image protocols.
- **Session recording / playback**
- **Authentication / access control**
- **Settings panel UI**

## Migration from Current Protocol

The current `0x01`/`0x02` binary protocol, `BoobaWebSocketAdapter`, `BoobaWasmAdapter`, and `booba_server` package are replaced entirely. The WASM adapter is unaffected by the protocol change (it uses a different communication mechanism via global JS functions).

The `internal/booba_server/` package is superseded by the new top-level server architecture. The LNM hack (`\x1b[20h`), manual TERM setting, and `waitForInitialSize` workaround are all eliminated by using real PTYs.
