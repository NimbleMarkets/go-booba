# go-booba - Web-based BubbleTea TUIs using libghostty

<p>
    <a href="https://nimblemarkets.github.io/go-booba/"><img src="https://img.shields.io/badge/Command%20Ref-6B2DAD" alt="Command Reference"></a>
    <a href="https://github.com/NimbleMarkets/go-booba/tags"><img src="https://img.shields.io/github/tag/NimbleMarkets/go-booba.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/NimbleMarkets/go-booba?tab=doc"><img src="https://pkg.go.dev/badge/github.com/NimbleMarkets/go-booba?utm_source=godoc" alt="GoDoc"></a>
    <a href="https://github.com/NimbleMarkets/go-booba/blob/main/CODE_OF_CONDUCT.md"><img src="https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg"  alt="Code Of Conduct"></a>
</p>

`go-booba` is a Golang module that facilitates embedding [BubbleTea](https://github.com/charmbracelet/bubbletea) Terminal User Interfaces (TUIs) into a Web Browser. Generally, these are accessed via a local terminal or via SSH. This module exposes an HTTP-based terminal connection to a BubbleTea program.

There are two facets we address with this package:

 * Running a full BubbleTea program in a Web Browser (via WebAssembly)

 * Running a Terminal in a browser that connects over WebSockets to a BubbleTea backend

## How and What?

The primary enabling technologies of this are:

 * [`libghostty`](https://github.com/ghostty-org/ghostty) - Terminal emulation engine
 * [`ghostty-web`](https://github.com/coder/ghostty-web) - Web-based terminal using Ghostty's VT100 parser via WebAssembly
 * [`BubbleTea`](https://github.com/charmbracelet/bubbletea) - Terminal UI framework for Go
 * [`WebAssembly`](https://webassembly.org) - For running Go code in browsers

The name `booba` is a portmanteau of the words *Boba* and *Boo!*: the [key ingredient of Bubble Tea](https://github.com/charmbracelet/bubbletea#bubble-tea) evoking a [Ghost's exclamation of joy](https://ghostty.org).

## TypeScript API

The `BoobaTerminal` class wraps ghostty-web's Terminal and provides a high-level API for embedding BubbleTea programs:

```javascript
import { BoobaTerminal } from './booba/booba.js';

const booba = new BoobaTerminal('terminal-container', {
    cols: 80,
    rows: 24,
    fontSize: 14,
    scrollback: 1000,
    allowOSC52: true, // Enable OSC 52 clipboard access
    cursorBlink: true,
    cursorStyle: 'block',
    theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
    },
});

await booba.init();
booba.connectWebSocket('ws://localhost:8080/ws');
booba.focus();
```

### Terminal Options

All [ghostty-web ITerminalOptions](https://github.com/coder/ghostty-web) are supported: `fontSize`, `fontFamily`, `cols`, `rows`, `cursorBlink`, `cursorStyle`, `scrollback`, `allowOSC52`, `allowTransparency`, `convertEol`, `disableStdin`, `smoothScrollDuration`, and a full `theme` with 16-color palette and cursor/selection colors.

### Selection & Clipboard

```javascript
booba.getSelection()       // Get selected text
booba.hasSelection()       // Check if text is selected
booba.copySelection()      // Copy to clipboard
booba.selectAll()          // Select all text
booba.clearSelection()     // Clear selection
booba.select(col, row, len) // Select at position
booba.selectLines(start, end)
booba.getSelectionPosition() // Get selection range
```

### Scrollback & Viewport

```javascript
booba.scrollLines(amount)  // Scroll by lines
booba.scrollPages(amount)  // Scroll by pages
booba.scrollToTop()        // Scroll to top of history
booba.scrollToBottom()     // Scroll to current output
booba.scrollToLine(line)   // Scroll to specific line
booba.getViewportY()       // Get current scroll position
```

### Terminal Control

```javascript
booba.paste(data)          // Paste with bracketed paste support
booba.input(data)          // Input as if typed
booba.focus()              // Focus terminal
booba.blur()               // Remove focus
booba.clear()              // Clear screen
booba.reset()              // Reset terminal state
booba.write(data)          // Write to display
booba.writeln(data)        // Write with newline
```

### Terminal Mode Queries

```javascript
booba.hasMouseTracking()   // Is mouse tracking enabled?
booba.hasBracketedPaste()  // Is bracketed paste enabled?
booba.hasFocusEvents()     // Are focus events enabled?
booba.getMode(mode, isAnsi) // Query arbitrary terminal mode
```

### Events

```javascript
booba.onStatusChange = (state, message) => { /* connection state */ };
booba.onTitleChange = (title) => { /* program set window title */ };
booba.onBell = () => { /* bell/beep fired */ };
booba.onSelectionChange = () => { /* selection changed */ };
booba.onKey = (event) => { /* key pressed */ };
booba.onScroll = (viewportY) => { /* viewport scrolled */ };
booba.onRender = ({ start, end }) => { /* rows rendered */ };
booba.onCursorMove = () => { /* cursor moved */ };
```

### Link Detection

```javascript
const disposable = booba.registerLinkProvider({
    provideLinks(y, callback) {
        // Detect links on row y, call callback with results
        callback(links);
    }
});
disposable.dispose(); // Unregister when done
```

### Custom Event Handlers

```javascript
booba.attachCustomKeyEventHandler((event) => {
    // Return true to prevent default handling
    return false;
});

booba.attachCustomWheelEventHandler((event) => {
    // Return true to prevent default scroll handling
    return false;
});
```

### Lifecycle

```javascript
booba.dispose()            // Clean up all resources
booba.terminal             // Access underlying ghostty-web Terminal
booba.cols                 // Current column count
booba.rows                 // Current row count
```

### Types

All types are exported for TypeScript consumers:

```typescript
import type {
    BoobaTerminalOptions,
    BoobaTheme,
    BoobaBufferRange,
    BoobaKeyEvent,
    BoobaRenderEvent,
    BoobaLinkProvider,
    BoobaLink,
    BoobaAdapter,
    BoobaConnectionState,
} from './booba/booba.js';
```

For adapter usage (WebSocket, WASM, custom), see [ADAPTER_USAGE.md](./ADAPTER_USAGE.md).

## Embedding a BubbleTea Application in a Web Browser

We can take entire BubbleTea applications and embed them into a Web Browser. The primary limitation is that all of its dependencies can also be compiled to WebAssembly.

### Quickstart

The top-level `booba.Run` picks the right runtime for the build target, so a single `main.go` works for both the native terminal and the browser:

```go
package main

import (
    "log"

    booba "github.com/NimbleMarkets/go-booba"
)

func main() {
    if err := booba.Run(initialModel()); err != nil {
        log.Fatal(err)
    }
}
```

Build and run natively with `go run ./cmd/myapp`. Build for the browser with `go run github.com/NimbleMarkets/go-booba/cmd/booba-wasm-build -o web/app.wasm ./cmd/myapp/`.

For finer control, the [`wasm`](./wasm) subpackage exposes the browser bridge directly, and native code can construct a `tea.Program` the usual way.

TODO: link to live example

## Web Frontend for BubbleTea-based service

Otherwise, one might have a BubbleTea program running on a remote machine. While one might use `ssh` to access it, `booba` enables an HTTP-based interface to it. The top-level `serve` package is the single server implementation for that path, serving the embedded Ghostty frontend and bridging browser clients over WebSocket or WebTransport.

### Middleware

The `serve` package exposes three composable middleware layers that mirror and extend the Wish/sip shape:

| Layer | Type | Wraps | Install |
|---|---|---|---|
| 1. Handshake | `ConnectMiddleware` | `*http.Request` for both WS upgrade and WT CONNECT | `WithConnectMiddleware(...)` |
| 2. Session I/O | `SessionMiddleware` | `Session` (transport byte streams) | `WithSessionMiddleware(...)` |
| 3. Handler | `Middleware` | `Handler` (per-session `tea.Model` construction) | `WithMiddleware(...)` |

`serve.LiftHTTPMiddleware(mw)` adapts any `func(http.Handler) http.Handler` into a `ConnectMiddleware` that runs on both the WebSocket and WebTransport handshake paths — so the full chi/gorilla/tollbooth/otelhttp ecosystem is reusable at the handshake.

Built-in middleware subpackages:

- `serve/middleware/osc52gate` — allow/deny/audit OSC 52 clipboard-write escapes in the outbound stream.
- `serve/middleware/recover` — catch panics during handler construction.
- `serve/middleware/logging` — slog-based session start/end logging.
- `serve/sipmetrics` — Prometheus counters/gauges/histogram for session lifecycle and byte throughput (isolated behind a subpackage so the main module avoids a `prometheus/client_golang` dep).

```go
srv := serve.NewServer(cfg,
    serve.WithConnectMiddleware(serve.LiftHTTPMiddleware(myHTTPMiddleware)),
    serve.WithSessionMiddleware(osc52gate.New(osc52gate.ModeDeny)),
    serve.WithMiddleware(recover.New(), logging.New()),
)
```

Basic Auth, connection limits, and `cfg.IdleTimeout` are auto-installed by `NewServer` when the corresponding `Config` fields are set.

### Config knobs

Beyond the listener/TLS/auth fields, `serve.Config` exposes protocol-safety knobs with sensible defaults:

- `MaxPasteBytes` (default 1 MiB) — cap bracketed-paste payloads from clients.
- `ResizeThrottle` (default 16ms) — debounce inbound resize messages.
- `MaxWindowDims` (default 4096×4096) — reject adversarial resize values before the PTY `ioctl`.
- `InitialResizeTimeout` (default 10s) — deadline on the initial Resize message after the handshake.
- `IdleTimeout` — close sessions with no inbound bytes for the given duration (0 = disabled).

See `docs/DESIGN_MIDDLEWARE.md` for the design rationale.

## `booba` CLI Command Wrapper

The `booba` command wraps any local CLI program and serves it in the browser through the same embedded terminal stack.

Build and run it from the repository root:

```sh
task build-cmd-booba
./bin/booba --listen 127.0.0.1:8080 -- htop
```

Everything after `--` is treated as the wrapped command and its arguments:

```sh
./bin/booba --listen 127.0.0.1:8080 -- bash
./bin/booba --listen 127.0.0.1:8080 -- python3 -q
./bin/booba --listen 127.0.0.1:8080 -- vim README.md
```

Build and run the example server from the repository root:

```sh
task build-cmd-booba-view-example-native
./bin/booba-view-example --listen 127.0.0.1:8080
```

The browser page served from `http://127.0.0.1:8080/` will use WebTransport automatically when available and fall back to WebSocket otherwise. When you provide `--cert-file` and `--key-file`, the same public port is used for HTTPS/WSS over TCP and HTTP/3 WebTransport over UDP.

Useful flags:

```sh
./bin/booba-view-example --listen 127.0.0.1:8080 --http3-port=-1
./bin/booba-view-example --listen 127.0.0.1:8080 --origin=https://app.example.com,https://*.example.net
./bin/booba-view-example --listen 127.0.0.1:8080 --cert-file=server.crt --key-file=server.key
./bin/booba-view-example --listen 127.0.0.1:8080 --username=admin --password=secret
```

Notes:

 * `--http3-port=-1` disables WebTransport and uses WebSocket only.
 * the default bind address is loopback (`127.0.0.1`); non-loopback `--listen` addresses require `--cert-file` and `--key-file`.
 * browser origins are same-host by default; use `--origin` to allow additional cross-origin browser clients.
 * Basic Auth requires `--cert-file` and `--key-file`; the server refuses to start otherwise.
 * static frontend files are embedded with `go:embed`, so after frontend asset changes you must rebuild the Go binary you run.

## Open Collaboration

We welcome contributions and feedback.  Please adhere to our [Code of Conduct](./CODE_OF_CONDUCT.md) when engaging our community.

 * [GitHub Issues](https://github.com/NimbleMarkets/go-booba/issues)
 * [GitHub Pull Requests](https://github.com/NimbleMarkets/go-booba/pulls)

## Acknowledgements

Thanks to the [Ghostty developers](https://github.com/ghostty-org/ghostty), the [ghostty-web](https://github.com/coder/ghostty-web) developers, and to [Charm.sh](https://charm.sh) for making the command line glamorous with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Thanks to [@BigJK](https://github.com/BigJk/bubbletea-in-wasm) for the initial inspiration when I was exploring this before `libghostty`.

Thanks to [@Gaurav-Gosain](https://github.com/Gaurav-Gosain), who cotemporaneously invented `sip`.  [That `sip` tool](https://github.com/Gaurav-Gosain/sip) is similar to this library, but works with `xterm.js`.   We adopted and extended its protocol and it also inspired our CLI tool.

## License

Released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE.txt](./LICENSE.txt).

Copyright (c) 2026 [Neomantra Corp](https://www.neomantra.com).   

----
Made with :heart: and :fire: by the team behind [Nimble.Markets](https://nimble.markets).
