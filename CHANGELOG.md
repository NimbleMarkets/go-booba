# `booba` CHANGELOG

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