# `booba-sip-client` — Design

**Date:** 2026-04-21
**Status:** Draft, pending implementation plan

## Motivation

Today `booba` is server-only: it wraps a local command under a PTY and serves it
over WebSocket/WebTransport using a Sip-compatible `[type][payload]` protocol.
The only client is the TypeScript `BoobaTerminal` frontend running in a browser.

A companion CLI client covers two use cases the browser frontend cannot:

1. **Real end-user access from a terminal** — connect to a running booba server
   over `ws://` or `wss://` and get an interactive session without a browser.
   Useful over SSH tunnels, in headless environments, and anywhere a terminal
   is preferable to a browser.
2. **Server testing and debugging** — a scriptable client that exercises the
   live wire protocol end-to-end in CI and during development, giving us
   regression coverage for drift between the server and any client.

## Goals

- One binary, `booba-sip-client`, that serves both use cases well.
- Minimal surface area; no features that can be deferred to a later revision
  without hurting the two goals above.
- Clean separation from the server: the client should not import
  `github.com/NimbleMarkets/go-booba/serve`.

## Non-goals

- WebTransport support (WebSocket only for v1).
- Parity with every feature of the browser frontend (e.g., OSC 52 clipboard,
  theming, font rendering — the local terminal handles those).
- A scenario DSL for scripted protocol tests. `--dump-frames` plus the
  existing test harnesses in `serve/` are sufficient.
- Windows-specific polish beyond what `golang.org/x/term` gives us for free.

## Packaging

- **New binary:** `cmd/booba-sip-client/main.go` — tiny entrypoint, build tag
  `//go:build !js`, calls into `internal/sipclient.Execute(ctx)`.
- **New CLI package:** `internal/sipclient/` (sibling of `internal/cli/`).
  - `root.go` — cobra root, flag wiring, `Execute`.
  - `client.go` — interactive client (connect, pumps, raw mode, escape, resize).
  - `dump.go` — `--dump-frames` mode.
  - `escape.go` — `^]` telnet-style command prompt.
  - Test files alongside each.
- **New shared protocol package:** `sip/` at the repo root (sibling of `serve/`).
  - Move `serve/protocol.go` and `serve/protocol_test.go` to `sip/` (package
    `sip`); update every reference in `serve/` and its tests from
    `serve.MsgXxx`/`serve.DecodeWSMessage`/etc. to `sip.MsgXxx`/
    `sip.DecodeWSMessage`/etc.
  - `sip/` has only stdlib deps, keeping the client's import graph small.

## CLI surface

```
booba-sip-client [flags] <url>                 # interactive
booba-sip-client --dump-frames [flags] <url>   # non-interactive frame dumper
```

`<url>` is a positional argument of the form `ws://host:port[/path]` or
`wss://host:port[/path]`. If the path is omitted it defaults to `/ws` (the
server's default endpoint). The scheme is required.

### Flags

| Flag | Default | Purpose |
|---|---|---|
| `--origin <url>` | derived from target URL | Value sent as `Origin` header |
| `--header <k:v>` | — | Repeatable extra request headers |
| `--insecure-skip-verify` | false | Accept self-signed TLS certs |
| `--ca-file <path>` | — | Additional trust anchors for `wss://` |
| `--escape-char <char>` | `^]` | Local escape; accepts `^X` notation or `none` |
| `--read-only` | false | Client-side: ignore local input, still render output |
| `--kitty` / `--no-kitty` | auto | Enable Kitty keyboard passthrough |
| `--debug` | false | Log decoded frames to stderr during interactive use |
| `--dump-frames` | false | Non-interactive frame dumper |
| `--dump-input <path>` | — | With `--dump-frames`: file sent as `MsgInput` after connect |
| `--dump-timeout <dur>` | `0` | With `--dump-frames`: exit after this long |
| `--connect-timeout <dur>` | `10s` | Dial/upgrade timeout |

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Clean disconnect (`MsgClose` received, or user escaped out) |
| `1` | Connect or TLS handshake failure |
| `2` | Protocol error (malformed frame, unknown type without `--debug`) |
| `3` | Unexpected transport close after connect |

### Escape prompt

On `--escape-char` at start-of-line, the client shows `booba-sip-client> ` on
the local tty and reads a single line. Commands:

- `quit`, `exit`, `q` — clean disconnect, exit 0.
- `continue`, empty line — resume the session.
- `status` — print the target URL, connection duration, last frame time.
- `help` — print available commands.
- Anything else — print help, stay in prompt.

## Data flow

### Connection setup

1. Parse `<url>`; build `http.Header` (`Origin`, repeated `--header` entries).
2. If `wss://`, build a `tls.Config` honoring `--insecure-skip-verify` and
   `--ca-file`.
3. Dial via `github.com/coder/websocket`'s `Dial` with a context bounded by
   `--connect-timeout`.
4. On successful upgrade, create a `context.WithCancelCause` that both pumps
   share — first error wins and tears down the session.

### Pumps

**Server → client pump.** Read frames off the WebSocket, decode via
`sip.DecodeWSMessage`, and route:

- `MsgOutput` → write payload to stdout.
- `MsgTitle` → emit `OSC 2 ; <title> BEL` to the local tty.
- `MsgOptions` → parse JSON, apply (e.g., set `readOnly`).
- `MsgPing` → reply with `MsgPong`.
- `MsgClose` → cancel context with a clean-close cause, return.
- `MsgKittyKbd` → push the reported flags to the local terminal.
- Unknown → protocol error (with `--debug`, log and continue).

**Client → server pump.** Feed stdin through
`github.com/charmbracelet/x/input`'s reader to get key events with access to
their source bytes.

- If the event matches `--escape-char` and `startOfLine == true`, detach from
  the pump, show the escape prompt, reattach on `continue`.
- Otherwise, if `--read-only` is off, write the event's raw bytes as
  `MsgInput` to the server.
- Track `startOfLine`: set to `true` after emitting `\r` or `\n`; cleared by
  any other byte. This mirrors telnet's rule and avoids false positives
  mid-command.

### Resize

- Unix: install a `SIGWINCH` handler. On signal, read current `(cols, rows)`
  via `x/term.GetSize(fd)` and send `MsgResize` with `{cols, rows}` JSON.
- Windows: poll terminal size on a ticker (x/term abstracts the source).
- Debounce with a 50ms trailing timer to coalesce resize bursts.
- Server-side throttling in `serve/resize_throttle.go` is authoritative; light
  client-side coalescing is enough.

### Startup sequence

After dial succeeds, before starting pumps:

1. Put the local tty into raw mode (`x/term.MakeRaw`); defer restore.
2. If Kitty is enabled, query the terminal for supported flags via
   `CSI ? u`, read the response with a ~100ms timeout. If the terminal
   answers, push our supported flags via `CSI > <flags> u` and send an
   initial `MsgKittyKbd` to the server. If it doesn't answer, proceed
   without Kitty and log a debug line.
3. Send an initial `MsgResize` with the current terminal size.
4. Start both pumps.

### Shutdown sequence

Guaranteed in reverse via `defer`s:

1. Cancel the shared context; both pumps exit.
2. Close the WebSocket with an appropriate status code (`1000` normal,
   `1002` protocol error, `1011` internal).
3. Pop Kitty flags (`CSI < u`) if pushed.
4. Restore tty state (raw mode off, cursor visible).
5. Print a final status line on stderr: `Connection closed: <reason>`.

### `--dump-frames` mode

Same server-read pump structure, but every decoded frame is encoded as a JSON
line on stdout:

```json
{"type":"output","data":"<base64>"}
{"type":"title","title":"vim - README.md"}
{"type":"options","readOnly":false}
{"type":"close"}
```

No raw-mode tty, no escape handling, no resize. The client-send pump is
skipped unless `--dump-input` is set, in which case the file's contents are
sent as a single `MsgInput` and then the client just listens until
`MsgClose` or `--dump-timeout` elapses.

## Error handling

- Connect or TLS errors → exit 1, write `Error: <cause>` to stderr.
- Malformed frames or unknown types in interactive mode without `--debug` →
  exit 2 promptly; silently dropping frames masks real bugs.
- Transport errors after connect → exit 3; `--debug` emits the underlying
  error.
- **Stdout discipline.** In interactive mode, stdout is the terminal — the
  ONLY thing written there is `MsgOutput` payload bytes. Status, debug, and
  error output all go to stderr. `--dump-frames` reverses this for its
  non-interactive use: stdout is JSON frames, stderr is status.

## Testing

### Unit tests (`internal/sipclient/`)

- `escape_test.go` — escape-char parsing (`^]` → 0x1d, `^X` notation, `none`),
  start-of-line state machine across synthetic byte streams, prompt command
  dispatch.
- `client_test.go` — pump logic with a fake `io.ReadWriter` standing in for
  the WebSocket; verifies every `MsgType` is routed correctly, `MsgPing`
  triggers `MsgPong`, unknown types produce a protocol error (and are logged
  under `--debug`), `MsgClose` causes a clean cancel.
- `dump_test.go` — every message type encodes to the expected JSON shape,
  `--dump-input` sends file contents verbatim, `--dump-timeout` exits with
  the right code.
- `root_test.go` — flag parsing: URL validation, `--origin` defaulting,
  `--header` repetition, `--escape-char` notation, flag-combination behavior.

### Protocol package tests (`sip/`)

- `sip/protocol_test.go` — moved from `serve/protocol_test.go` unchanged
  beyond package rename.

### End-to-end tests

1. **In-process e2e in `cmd/booba-sip-client/`.** Spin up `serve.Server` in a
   goroutine against `httptest.NewServer` wrapping a deterministic command
   (`sh -c 'echo hello; sleep 0.1; exit 0'`), invoke the client in
   `--dump-frames` mode against that URL, assert the expected JSON sequence
   (options → output containing `hello` → close). Fast and flake-resistant.
2. **Cross-binary smoke in `serve/e2e_test.go`.** Extend the existing e2e
   with a subtest that builds the client binary and runs it with
   `--dump-frames` against the live server. Guards against server/client
   wire-format drift.

### Deliberately out of scope

- Real terminal emulation (raw mode, OSC 2 title, Kitty push/pop). Tty
  behavior is awkward to stub; interactive flows are exercised by developers
  running the client manually, plus the e2e coverage above catches wire-level
  bugs.
- Windows-specific ConPTY behavior beyond what `x/term` gives us.

## Release

- `Taskfile.yml` gains `build:client` / `test:client` targets mirroring the
  server's equivalents.
- If the release workflow builds and publishes `booba`, it learns to build
  and publish `booba-sip-client` alongside it. To be confirmed during plan
  authoring against the actual workflow files.

## Open questions / follow-ups

- None blocking v1. Revisit after landing:
  - WebTransport support.
  - A `--script` / scenario-runner mode if `--dump-frames` proves too
    low-level for the tests we actually want.
  - `docs` subcommand for man-page generation (mirror of `booba docs`).
