# `booba` CHANGELOG

## Unreleased

### Fixed

- Prevent WebGL and WebGPU from clipping the tops of emoji, and Canvas2D from
  painting them into the row above. The embedded ghostty-web now measures cell
  height and baseline using descenders, the primary font line box, and emoji
  fallback ink. Column width stays the same; rows can become taller to fit the
  full glyph. Joined emoji remain a single two-column grapheme.

### Added

- Add a lightweight touch and mouse tracking demo to the demo site, with a local
  input inspector, decoded modifiers, gesture counters, and explicit keyboard
  and tracking controls. It needs no Go application or network after startup.
- Translate browser terminal touch gestures into SGR mouse input when the program
  enables mouse tracking: tap to click, one finger to drag, pinch to zoom around
  the midpoint, and two fingers to scroll. Support all adapters, prevent page
  scrolling during tracked gestures, and suppress duplicate mouse clicks.

## [0.10.3] - 2026-10-19

### Fixed

- Color emoji keep their colors with the WebGPU and WebGL renderers. Update
  the embedded ghostty-web to the fork build from 70a5e6f, whose GPU
  renderers drew every glyph as a mask in the text color, so emoji came out
  as silhouettes; Canvas2D already drew them.

## [0.10.2] - 2026-10-19

### Fixed

- OSC 52 clipboard writes now work for in-browser (wasm) programs. The wasm
  bridge delivers output as strings, which the scanner skipped; it now scans
  bytes and strings alike, and decodes the payload as UTF-8 so non-ASCII text
  copies intact.
- Update the embedded ghostty-web to the fork build from 919b01d, which
  reports wheel events as SGR mouse buttons when the program tracks the
  mouse, forwards Ctrl+V and Cmd+V as 0x16 before the paste event, and
  follows DECSCUSR cursor shapes at runtime (coder/ghostty-web#145).

## [0.10.1] - 2026-10-18

### Terminal Actions (experimental)

- Add `bubblehelp.NewModelWithFallback` for applications where action metadata
  is optional. Invalid or oversized descriptions produce a diagnostic without
  blocking native startup or WASM bridge registration. Failed publication clears
  stale controls and retries on later updates; keyboard bindings remain intact.
  Existing `NewModel` callers retain strict validation and must opt in.
- Keep OSC 7501's 2,048-byte decoded message limit, including the UTF-8 summary,
  marker, and serialized JSON. Never truncate payloads or partially advertise
  oversized descriptions. The Go receiver now rejects oversized messages,
  matching browser enforcement.
- Document action-bar sizing and migration, and add regression coverage for
  exact byte boundaries, Unicode, oversized help maps, recovery, native startup,
  and WASM bridge registration.

### Development

- Check SIP client shutdown errors in tests to satisfy repository lint checks.

## [0.10.0] - 2026-10-17

### Terminal Actions (experimental)

- Add `actions/termhost`, which hosts Terminal Actions inside the program when
  the terminal does not answer the OSC 7501 capability query. It answers the
  query itself after a timeout, filters the published reports out of the
  program's output, draws them as a row of clickable buttons under the view,
  and sends activations through the ordinary message loop as key presses.
  A terminal that does answer is left alone.
- Add `actions.DecodeProgramStatus`, `actions.Report`, and
  `actions.IsProgramStatusQuery` for Go hosts that read OSC 7501 reports.

## [0.9.2] - 2026-10-12

- Refresh ghostty-web to fix retransmitted-image caches, honor synchronized
  output, suppress missing-image placeholder boxes, and reduce Canvas2D redraw
  costs and repeated clears at fractional display scaling.

## [0.9.1] - 2026-10-11

### Terminal Actions (experimental)

- Shorten generated action IDs from 34 characters to eight hexadecimal
  characters, reducing repeated IDs in action definitions and help lists.
  Collisions return an error and can be resolved with explicit IDs.
- Make `bubblehelp.Snapshot` return an error when the snapshot, marker, and
  readable summary exceed OSC 7501's 2048-byte decoded message limit.
  `Options.Summary` supplies the summary for validation and defaults to context.
  `bubblehelp.NewModel` also validates its initial description at construction;
  later publication errors still clear stale controls and call `OnError` when set.
- Export `encodeKey(gesture, modes)` and `booba.encodeKey(gesture)` for opt-in
  legacy VT/xterm keyboard encoding. The terminal method reads current cursor,
  backarrow, and newline modes on each call. Unsupported gestures return `null`.
  Kitty keyboard and modifyOtherKeys protocols still require a host encoder
  because ghostty-web does not expose their state. Update the actions demo to
  use the shared legacy encoder.

### Packaging

- Add `booba-assets --no-index` to skip the starter page and the existing-page
  notice when a host supplies its own HTML. It takes precedence over `--force`.
- Refresh embedded browser assets to include the shared key encoder.

## [0.9.0] - 2026-10-11

### Server shutdown

- Let served Bubble Tea programs stop their input readers before closing the PTY
  on disconnect. Drain final renderer output and ignore late resizes during shutdown.

### Program status

- Keep Program Status Protocol (OSC 7501) records in `booba.programStatus`, a store
  with the protocol's lifetime rules: reports replace records, `clear` removes a
  subtree, working and blocked records drop at a shell prompt or process exit, and
  done and error persist until the user's next input.
- Add `installStatusBar`, a host-styled bar over the store with the same class and
  cascade-layer contract as the action bar, and `STATUS_BAR_CSS` for CSP hosts.
  Installing it again on the same parent replaces the earlier bar instead of
  stacking a second one.
- `booba-view-example` reports idle, working with a percentage, and done; the demo
  site shows the bar on every demo page.

### Packaging and development

- Add a GitHub Pages demo site generated by `cmd/wasm-build` from `web/demos.yaml`:
  the Bubble Tea views sample and the Terminal Actions demo, compiled to
  WebAssembly. `task site` builds it locally and `task site-serve` serves it.
  Each demo page has a Restart button that runs the program again on a fresh Go
  runtime after it exits, and frames the program status bar as a panel with a
  state dot. `task site-serve` fails when port 3000 is taken instead of moving
  to another port.
- Run `booba-view-example` through `booba.Run`, so its browser build registers the
  WASM bridge instead of failing to open a TTY.

### Go and WASM integration

- Announce program exit across the WASM bridge. When a browser-hosted program
  returns, the `wasm` package stores its unread output in `window.bubbletea_final`
  and sets `window.bubbletea_exited` before the Go runtime goes away; the WASM
  adapter delivers that final frame, stops polling, and reports the session as
  disconnected. The action bar clears and the console no longer fills with
  "Go program has already exited" errors. `task go-test-wasm` runs the `wasm`
  package's tests under Node and is part of `task test`.
- A program superseded on the bridge by a later `NewProgram` no longer publishes
  its exit, so a page can start a fresh program while the previous one is still
  returning.

## [0.8.0] - 2026-10-10

Booba 0.8.0 adds an experimental Terminal Actions system that turns an
application's help bindings into browser touch controls, updates the embedded
ghostty-web runtime to a libghostty-vt build with OSC 7501 program status, and
moves the BubbleTea WASM fork onto v2.1.0.

### Upgrading from 0.7.x

The pinned BubbleTea fork changed. Update the `replace` directive in your
module's `go.mod`:

```sh
go mod edit -replace charm.land/bubbletea/v2=github.com/neomantra/bubbletea/v2@v2.0.0-20261009201241-c397c32a5074
go mod tidy
```

The fork is now based on BubbleTea v2.1.0 and carries only the `js/wasm` and
`wasip1` build-tag stubs. Register the asset tool with
`go get -tool github.com/NimbleMarkets/go-booba/cmd/booba-assets@v0.8.0`.
Browser builds pick up the new ghostty-web runtime by rerunning `booba-assets`.

### Terminal Actions (experimental)

- Add a transport-independent JSON description of an application's available
  key interactions, with Go validation and a change-aware publisher.
- Add a Bubbles v2.1 help adapter: short help supplies the compact controls,
  full help supplies grouped expanded controls, and the same bindings drive
  terminal help and browser buttons.
- Carry snapshots inside OSC 7501 program status reports under the private,
  opt-in `booba-actions/v1` convention. Enable it with `allowOSC7501Actions`.
- Add `ActionController`, `installActionBar`, and `setActionKeyEncoder` to the
  browser API. Buttons send only host-encoded keys and never implicitly focus
  the terminal. Controls clear at shell prompts, alternate-screen transitions,
  resets, reconnects, and disposal.
- Style the action bar from the host page: every element carries a stable
  class, defaults live in `@layer booba` so plain host rules win without
  `!important`, `ACTION_BAR_CSS` and `styles: false` serve strict CSP hosts,
  and `init()` mirrors the theme as `--booba-background` and
  `--booba-foreground` on the container.
- Add the `booba-actions-example` native demo and
  [docs/TERMINAL_ACTIONS.md](docs/TERMINAL_ACTIONS.md). APIs and the wire
  convention may change.

### Terminal rendering

- Update ghostty-web to `579fc63` (source `3f84d01`), synced to libghostty-vt
  `a4aacd9`. The runtime exposes OSC 7501 reports through `onProgramStatus`;
  Booba forwards them via `booba.onProgramStatus` and reports availability in
  `supportsProgramStatus`.
- Restore line-based scrollback. The libghostty-vt update introduced a 10,000
  byte scrollback budget that capped history at roughly 940 rows; Booba's
  `scrollback` option again means lines.
- Keep grapheme clustering (mode 2027) across a full terminal reset, so flag
  and ZWJ emoji stay single cells after a program sends `ESC c`.
- The WASM runtime grows from 626 KB to 940 KiB with the upstream update.
  Rebuilding it requires Zig 0.16.0.

### Go and WASM integration

- Add `Server.HTTPHandlerFor` to build an embeddable handler for a given
  session handler with the server's middleware applied once.
- Move the BubbleTea WASM fork to `c397c32`, rebased onto v2.1.0, which
  provides `View.ProgramStatus` and the capability-query messages natively.
- Hold a finished WebTransport session open briefly, up to two seconds, until the
  client closes it. Closing immediately could make the client discard the final
  output and close frames, which surfaced as a flaky end-to-end test on CI.

### Packaging and development

- Point the Command Reference badge at the generated command docs in the
  repository; the previous GitHub Pages target was never deployed.
- Document the host styling contract in
  `docs/superpowers/specs/2026-10-09-action-bar-styling-design.md`.

## [0.7.0] - 2026-09-28

Booba 0.7.0 improves browser terminal rendering with GPU backends, bidirectional
text support, and faster input echo. 

Browser WASM programs now build with standard Go commands, and `booba-assets` embeds its browser assets. Embedded assets are smaller, and development dependencies have been updated.

### Upgrading from 0.6.x

Go 1.26 or newer is now required.

`booba-wasm-build` has been removed. Add the pinned BubbleTea fork to your
application's `go.mod`, then build with Go directly:

```sh
go mod edit -replace charm.land/bubbletea/v2=github.com/neomantra/bubbletea/v2@v2.0.0-20260928192001-1b36865b418a
go mod tidy
GOOS=js GOARCH=wasm go build -o web/app.wasm ./cmd/myapp/
```

Go does not propagate dependency `replace` directives, so each consumer building
for the browser needs this entry. Use `booba-assets web/` to generate the supporting
browser files.

### Terminal rendering

- Update ghostty-web to `c51504e` (source `10a023d`), with WebGPU, WebGL, and
  Canvas2D renderers and automatic fallback.
- Add bidirectional text ordering, with matching selection, copy, link detection,
  and mouse-coordinate handling. Arabic/Persian cursive shaping remains unsupported.
- Support browser Kitty shared-memory images, reduce input echo latency, and fix
  stale cells appearing after scrolling.
- Expose renderer selection and HUD utilities through the TypeScript API. The HUD
  shows the active backend and FPS; click the badge or press Alt+Shift+R to switch.
- Add `--renderer auto|webgpu|canvas2d` to set the server default. The browser's
  `?renderer=` option takes precedence and also supports `webgl`.
- Validate the CLI renderer option and safely encode it into the served page.
- Wait for asynchronous terminal initialization before connecting a backend.

### Go and WASM integration

- Use the patched BubbleTea dependency instead of a separate WASM build tool.
  Refresh the `nm-wasm` fork to `1b36865`, based on BubbleTea v2.0.10.
- Expose BubbleTea methods directly and add `TeaProgram()` to access the underlying
  program when an API requires `*tea.Program`.
- Set browser terminal capability defaults for `TERM_PROGRAM`, `COLORTERM`, and
  `CLICOLOR_FORCE`, preserving values already supplied by the host.
- Embed the scaffolding files in `booba-assets`, so the built binary can run
  outside a Go module without downloading assets. A matching Go toolchain is
  still needed for `wasm_exec.js`. Version-suffixed `go run` and `go install`
  are blocked by the module's BubbleTea `replace` directive; use a release
  binary or register the command with `go get -tool` in a consumer module.

### Packaging and development

- Consume prebuilt ghostty-web artifacts; building Booba no longer requires Zig,
  Bun, or Nix.
- Remove about 1.2 MB of unused CJS and declaration files from the embedded assets.
  Copy only browser JavaScript and WASM, prune obsolete chunks, and exclude leftover
  development files from Go binaries.
- Keep declarations and source maps in the npm package, without emitting them into
  the embedded browser files.
- Update Happy DOM to 20.14.5, Vitest to 4.1.11, and affected transitive dependencies.
  Install the ghostty-web package without its development toolchain.
- Run TypeScript tests and check committed assets in CI. Add JavaScript/WASM
  compatibility tests and a manual RTL rendering harness.
- Preserve intentional submodule updates during builds, track embedded assets when
  rebuilding `booba-assets`, and use the project's Go toolchain for commit checks
  and linting.

This release is dedicated to all the mothers, past and present. Thank you for your
love and nurturing.

## `v0.6.0` (2026-04-29)

End-to-end kitty graphics support — `kitten icat`, [ntcharts image demos](https://nimblemarkets.github.io/ntcharts), and other libraries that emit Unicode placeholder cells (`a=T,U=1,…` + `U+10EEEE`) now render correctly in the browser.

   * `feat(serve)`: server-side kitty graphics PNG → RGBA transcoder. The wasm has no PNG decoder (wuffs needs libc, `wasm32-freestanding` doesn't have it). The transcoder sits in `kittyGfxTranscoder` between the PTY and the WS/WT writer, intercepting `f=100` APC sequences and re-emitting them as chunked `f=32`
  raw NRGBA.
   * `feat(sip)`: pixel dimensions plumbed through resize. `widthPx`/`heightPx` extend the Sip resize message; `WindowSize` / `xpty.UnixPty.SetWinsize` populate `ws_xpixel`/`ws_ypixel` so kittens see non-zero `TIOCGWINSZ` pixel fields.
   * `fix(serve)`: stop dropping U=1 kitty graphics transmissions. Earlier defensive guard against an older ghostty-web that couldn't render virtual placements; with the new renderer, the drop was silently breaking ntcharts-style demos.
   * `fix(task)`: `serve/static` assets tracked in build sources so embedded JS/WASM regenerate when source changes.
   * Docs: improved README and added project mascot.

  **Note:** kitty graphics rendering requires a `ghostty-web` build with virtual-placement support. If pinning to a specific `ghostty-web` version, ensure it includes the `Substitute U+10EEEE cells with kitty graphics image slices` change.

To achieve this, we are maintaining a [NimbleMarkets fork of ghostty-web](https://github.com/NimbleMarkets/ghostty-web/tree/nm-kitty-meow) in the `nm-kitty-meow` branch.

## `v0.5.3` (2026-04-23)

 * `booba-sip-client`: add WebTransport support
 * Two server-side WebTransport bug fixes surfaced during end-to-end development

## `v0.5.2` (2026-04-22)

 * fix(wasm): force color output in browser terminal emulator using `CLICOLOR_FORCE=1`

## `v0.5.1` (2026-04-22)

 * Add `booba.NewProgram(model)` and `wasm.NewProgram(model)` as more idiomatic entry points

## `v0.5.0` (2026-04-22)

  * New companion CLI `booba-sip-client` for connecting to a running booba server from a terminal
  * Shared `sip/` package carrying the wire protocol.
  * Bug fixes

## `v0.4.1` (2026-04-21)

Follow-up patch addressing findings from repository review. No breaking API changes.

 * Security — `/static/` now runs through `checkAuth` so assets don't leak fingerprints to unauthenticated clients (SEC-2)
 * Security — `--password-file` flag and `$BOOBA_PASSWORD` env fallback; help text steers operators off the argv-leaking `--password` form. Precedence: flag > file > env (SEC-1)
 * Security — `index.html` endpoints resolve against `document.baseURI` via the new exported `resolveBoobaURLs()` helper, letting booba host behind a path-prefix reverse proxy that strips the prefix (SEC-15)
 * Fix — `serve.MakeOptions` no longer miswires non-`*ptySession` I/O; the non-PTY path now returns env only and custom sessions supply their own `tea.WithInput`/`tea.WithOutput` via handler extras (SE-2)
 * Fix — `booba-assets` `copyFile` normalizes destination permissions to `0o644` regardless of source mode (SE-10)
 * DX — `Debug`-gated log line for unknown WS/WT message types (SE-9)
 * DX — `ts/booba.ts` `term: any` replaced with `Terminal | null`; narrowed locals threaded through `init()`/`_setupAdapter()`/`_watchDevicePixelRatio()` (SE-6)
 * Docs — `OriginPatterns` godoc, `--origin` flag help, and README spell out that patterns are `path.Match` shell globs, not regex
 * Testing — Vitest added; 41 TypeScript tests across protocol encode/decode, WebTransport length-prefix framing, OSC 52 scanner edge cases, WebSocket reconnection backoff, and reverse-proxy URL resolution. `tryDecodeWTFrame` extracted into `ts/protocol.ts` so the framing logic is unit-testable
 * Build — `go.mod` floor lowered from `go 1.26.2` to `go 1.25` (actual dep minimum)

## `v0.4.0` (2026-04-21)

Large rollup spanning the unreleased v0.2 / v0.3 tags into a single cut.

 * `booba.Run` polymorphic entry point dispatching on `js && wasm` build tags
 * Three-layer middleware architecture: Connect → Session → Handler, with `WithConnectMiddleware`, `WithSessionMiddleware`, `WithMiddleware`, and `LiftHTTPMiddleware` adapter
 * `NewServer` variadic options pattern (`WithSessionFactory`, etc.)
 * Built-in middleware: basic auth, connection limit, panic recovery (`serve/middleware/recover`), session-lifecycle logging (`serve/middleware/logging`), idle timeout, OSC 52 clipboard-write gate (`serve/middleware/osc52gate`)
 * `serve/sipmetrics` subpackage — Prometheus-backed session metrics
 * Config knobs: `MaxPasteBytes`, `ResizeThrottle`, `MaxWindowDims`, `InitialResizeTimeout`
 * `Identity` API and `ConfigFromContext` / `RemoteAddr` context helpers for middleware
 * `ConnectError` with WebTransport status-code mapping; `writeConnectError` for WS rejection rendering
 * Windows ConPTY support for the command wrapper
 * GoReleaser-based release pipeline
 * WASM: release `js.FuncOf` callbacks to prevent leaks on hot reload
 * WebTransport: amortized-grow read buffer (replaces O(n²) per-chunk copy)
 * Documentation generation commands

## `v0.1.4` (2026-04-16)

 * Initial release
