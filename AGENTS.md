# Agents

This document defines the agents and personas associated with the `booba` project.

## Project Roles

### 🤖 Antigravity (AI)
- **Role**: Senior Software Engineer & Pair Programmer
- **Responsibilities**:
  - Writing and refactoring Go code
  - Managing WASM build targets
  - Implementing BubbleTea TUI components
  - Ensuring code quality and documentation

### 👤 User
- **Role**: Project Lead & Architect
- **Responsibilities**:
  - Defining requirements and scope
  - Reviewing code and architectural decisions
  - Managing project direction

## Upstream Reference: libghostty-vt Demo Implementations

Booba's VT-to-web bridge is derived from the reference demo implementations
shipped with libghostty-vt (ghostty-web). These demos show how to wire
ghostty-web's WASM-based terminal emulator to WebSocket and WebTransport
backends using the Sip protocol.

We do **not** vendor or directly include the upstream source. Instead, we
took the ghostty-web build artifacts (WASM + JS + types) and adapted the
demo's terminal initialization patterns into our own TypeScript wrapper.
The transport protocol (Sip-compatible binary framing, WebTransport, etc.)
is entirely booba's own — the upstream demo uses raw strings over WebSocket.

As ghostty-web evolves (new Terminal API, bug fixes, renderer improvements),
we should periodically update our build artifacts and adjust our wrapper
code for API changes.

### Upstream tracking

- **Upstream repo:** `~/projects/ghostty-web`
- **No automated tracking.** Parity checks are manual.
- **Last checked:** 2026-04-15

### What comes from upstream

#### Pre-built ghostty-web distribution (used as-is)

| File | Description |
|------|-------------|
| `serve/static/ghostty-web/ghostty-web.js` | Terminal emulation engine compiled from libghostty |
| `serve/static/ghostty-web/ghostty-web.umd.cjs` | UMD build of the same |
| `serve/static/ghostty-web/ghostty-vt.wasm` | WASM binary for VT100 parsing |
| `serve/static/ghostty-web/index.d.ts` | TypeScript definitions for ghostty-web API |

Update by replacing with newer builds from ghostty-web (`bun run build`).

#### Terminal initialization pattern (adapted from demo)

The upstream demo (`demo/index.html`, `demo/bin/demo.js`) shows the
canonical way to initialize and wire ghostty-web. Our `ts/booba.ts`
follows this pattern:

```
init() → new Terminal(opts) → loadAddon(FitAddon) → open(container)
  → fitAddon.fit() → fitAddon.observeResize()
  → term.onData() for input, term.write() for output
  → term.onResize() for dimension changes
```

### What is booba's own (not from upstream)

These files implement booba-specific functionality with no upstream equivalent:

| File | What it does |
|------|-------------|
| `ts/protocol.ts` | Sip-compatible binary protocol (`'0'`-`'8'` message types, WS + WT framing) |
| `ts/websocket_adapter.ts` | WebSocket adapter with binary Sip framing, exponential backoff reconnection, ping/pong |
| `ts/webtransport_adapter.ts` | WebTransport/QUIC adapter with length-prefixed framing and cert pinning |
| `ts/auto_adapter.ts` | Auto-detection: tries WebTransport first, falls back to WebSocket |
| `ts/adapter.ts` | `BoobaAdapter` interface + WASM polling adapter |
| `ts/clipboard.ts` | OSC 52 clipboard sequence scanner |
| `ts/types.ts` | Booba-specific type re-exports and definitions |
| `serve/protocol.go` | Go-side Sip protocol encode/decode |
| `serve/handlers.go` | Go-side WebSocket + WebTransport session handling |

The upstream demo uses raw UTF-8 strings for I/O and JSON
`{ type: 'resize', cols, rows }` for resize — no binary framing.

### Areas to watch for parity

When upstream ghostty-web updates, check:

1. **Terminal constructor options** — New options in `ITerminalOptions`
   (upstream `lib/interfaces.ts`) that `BoobaTerminalOptions` should mirror.
   Currently booba surfaces: `fontSize`, `fontFamily`, `cols`, `rows`,
   `cursorBlink`, `cursorStyle`, `scrollback`, `allowTransparency`,
   `convertEol`, `disableStdin`, `smoothScrollDuration`, `theme`, plus
   booba-specific `allowOSC52`. Watch for additions upstream.

2. **Terminal API surface** — New public methods on `Terminal` (upstream
   `lib/terminal.ts`) that `BoobaTerminal` should proxy. Currently booba
   covers: write, writeln, paste, input, focus, blur, clear, reset,
   selection, scrolling, link providers, mode queries, custom event handlers.

3. **FitAddon changes** — The `fit()` / `observeResize()` API in upstream
   `lib/addons/fit.ts`. Booba uses both correctly.

4. **Event surface** — Booba wires all upstream events: onData, onResize,
   onBell, onSelectionChange, onKey, onTitleChange, onScroll, onRender,
   onCursorMove. Watch for new events upstream.

5. **Build artifacts** — When `ghostty-web.js`, `ghostty-vt.wasm`, or
   `index.d.ts` change, replace the files in `serve/static/ghostty-web/`
   and verify our TypeScript still compiles against the new types.

6. **Mobile viewport handling** — Both booba and upstream use a
   `visualViewport` handler for mobile keyboards. Keep in sync.
