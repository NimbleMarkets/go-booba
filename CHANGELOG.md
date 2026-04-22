# `booba` CHANGELOG

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