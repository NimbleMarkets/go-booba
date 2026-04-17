# Design Note: Middleware Architecture

**Status:** Shipped in v0.3. The serve package now exposes ConnectMiddleware, SessionMiddleware, and Middleware (layer-3) with functional options on NewServer; LiftHTTPMiddleware adapts net/http middleware. See `docs/superpowers/specs/2026-04-16-v0.3-middleware-design.md` for the implementation spec and `docs/superpowers/plans/2026-04-16-v0.3-middleware-implementation.md` for the task-by-task plan that landed.

**Audience:** booba maintainers and contributors planning the middleware story.

## Context

booba's `serve` package already has a Wish-shaped handler API:

- `serve.NewServer(cfg, ...Option) *Server`
- `serve.Handler = func(Session) (tea.Model, []tea.ProgramOption)`
- `serve.MakeOptions(sess) []tea.ProgramOption`
- `serve.WithSessionFactory(factory)` (replaced `SetSessionFactory` in v0.3)

This mirrors `charmbracelet/wish`'s `bubbletea.Handler` + `wish.WithMiddleware(...)` shape, and matches `Gaurav-Gosain/sip`'s public API. What's missing compared to Wish is **composable middleware**. Wish ships `activeterm`, `logging`, `recover`, `ratelimiter`, `accesscontrol`, `elapsed`, `comment`, composed via `wish.WithMiddleware(...)`. sip has the Wish-shaped handler without the middleware layer. booba today is in the same state as sip.

This note argues that the right design is **three middleware layers, not one**, because HTTP, WebTransport, Sip, and VT each have different ideas of "what the middleware wraps" — and conflating them either excludes legitimate concerns or misleads users about what the net/http ecosystem can do for them.

## Why not "just reuse net/http middleware"?

It's tempting to point at `Server.HTTPHandler()` and say "wrap it in chi/gorilla/tollbooth". That works for the WebSocket handshake path but breaks down on several axes:

1. **booba runs two listeners.** WebSocket rides the main TCP HTTP mux. WebTransport runs its own H3/UDP listener (`wtServer.ListenAndServe()` in `serve/server.go`). Middleware wrapping `HTTPHandler()` covers only the WS path; WT bypasses it entirely.
2. **Request/response vs long-lived session.** Classic HTTP middleware assumes a response will be written. WS and WT connections stay open for the full session. `chi.Timeout`, `Compress`, response-rewriting loggers, body-buffering middleware break or deadlock on long-lived upgrades.
3. **Post-handshake, there is no `http.Handler` for WT.** The session is a `webtransport.Session` with streams and datagrams. `func(http.Handler) http.Handler` has nothing to wrap.
4. **Metric semantics are wrong.** `promhttp.InstrumentHandlerDuration` on a WT CONNECT reports sub-second — not the session duration.
5. **QUIC concepts have no HTTP vocabulary.** Per-stream flow control, datagrams, session-scoped limits have no middleware equivalent in the net/http world.

The honest claim is that the net/http ecosystem is reusable **at the handshake only**, and only if booba provides a transport-neutral hook that runs on both the WS upgrade and the WT CONNECT. Then a thin adapter lifts `func(http.Handler) http.Handler` into that hook for both paths.

## The three layers

Each layer has a different natural object to wrap and a different audience.

| Layer | Type | Wraps | Audience |
|---|---|---|---|
| 1. Handshake | `ConnectMiddleware` | `*http.Request` (both WS upgrade and WT CONNECT) | Standard HTTP concerns: auth, rate-limit-by-IP, origin, access logging, request ID, tracing |
| 2. Session I/O | `SessionMiddleware` | `Session` | **Booba/Sip/VT-specific concerns** — this is the distinctive layer |
| 3. Handler | `Middleware` | `Handler` | App/session-lifecycle: recover around `tea.Program`, session-duration logging, activity timeouts |

### Layer 1 — Handshake (`ConnectMiddleware`)

Proposed shape:

```go
// ConnectHandler runs at the handshake boundary for both WS and WT.
// Return a non-nil error to reject; the error is translated into the
// right status / QUIC error code for the transport in use.
type ConnectHandler func(ctx context.Context, r *http.Request) error

type ConnectMiddleware func(next ConnectHandler) ConnectHandler

// LiftHTTPMiddleware adapts a net/http middleware into a ConnectMiddleware
// that runs on both the WS and WT handshake paths. The lifted middleware
// must not assume it can write a response body — handshake rejection is
// expressed by returning an error, which the framework translates to the
// right transport-specific error.
func LiftHTTPMiddleware(mw func(http.Handler) http.Handler) ConnectMiddleware
```

Installed via a Config field or `serve.WithConnectMiddleware(...)`. Runs on both upgrade paths so the same auth/rate-limit/origin policy applies regardless of which transport the browser negotiates.

With `LiftHTTPMiddleware`, the full chi/gorilla/tollbooth/prometheus-handler/otelhttp ecosystem is available at the handshake — honestly, across both transports.

### Layer 2 — Session I/O (`SessionMiddleware`)

Proposed shape:

```go
// SessionMiddleware decorates a Session. Composes over SessionFactory.
type SessionMiddleware func(Session) Session
```

Installed alongside or replacing `SetSessionFactory`. This is the hook for behaviors that operate on the Sip byte stream or the VT semantics inside it — concerns that no other Go middleware ecosystem addresses, because no one else parses terminal protocols over WS/WT.

**This is booba's distinctive contribution.** It is the reason booba is more than "sip + WASM".

Candidates for built-in middleware at this layer:

- **`osc52gate`** — scan outbound VT stream for OSC 52 (clipboard write) escapes. Modes: `allow`, `deny`, `audit` (log + forward), `prompt` (rewrite into no-op, emit a booba-protocol event). Multi-tenant terminal-as-a-service will want this.
- **`activeterm`** — browser-analog of wish's `activeterm`. Drop sessions that haven't reported a valid initial resize within N seconds. Catches probe clients and broken handshakes.
- **`idletimeout`** — disconnect after N seconds of no inbound Sip bytes. Resource hygiene on public endpoints.
- **`sipmetrics`** — Prometheus counters per Sip message type (bytes in/out, count, error rate), session-duration histogram, concurrent-session gauge. Provides the observability that generic HTTP metrics cannot — HTTP middleware reports CONNECT duration, not session duration. Ship in a subpackage (`serve/metrics` or `serve/sipmetrics`) so the main module doesn't force a `prometheus/client_golang` dep.

Not middleware — make these **config knobs** on `serve.Config` with sensible defaults:

- **`MaxPasteBytes`** (~1 MiB) — cap Sip bracketed-paste payloads. Protects the tea.Program's input channel from clipboard-bombing.
- **`ResizeThrottle`** (~16ms) — debounce inbound resize messages. Drag-resize in browsers spams hundreds of events.
- **`MaxWindowDims`** (~4096×4096) — cap cols/rows to reject adversarial resize values before they reach the PTY `ioctl`.

These are universal enough that opt-in is noise.

### Layer 3 — Handler (`Middleware`)

Proposed shape (matches Wish/sip):

```go
type Middleware func(next Handler) Handler
```

Candidates for built-in middleware at this layer, ported from `charmbracelet/wish` (all MIT-licensed, tens of lines each):

- **`recover`** — catch panics in the `tea.Program` per session so one bad handler doesn't kill the server.
- **`logging`** — session-lifecycle logging: connect, disconnect, duration, client address, session ID. Note this is different from request logging at layer 1.

App-level auth/authz typically lives here (not at layer 1) because it needs `Session` context and app state.

## Sequencing

- **v0.2** (shipped): `booba.Run`, `Handler` aligned to `func(Session) (tea.Model, []tea.ProgramOption)` to match Wish/sip, `MakeTeaOptions` → `MakeOptions`, README quickstart. No middleware yet.
- **v0.3** (shipped): three-layer scaffolding. `ConnectMiddleware` + `LiftHTTPMiddleware`, `SessionMiddleware`, layer-3 `Middleware`. Functional options on `NewServer`: `WithConnectMiddleware`, `WithSessionMiddleware`, `WithMiddleware`, `WithSessionFactory`. Built-in basic-auth and connection-limit migrated to auto-installed `ConnectMiddleware`. `ProgramHandler` / `ServeWithProgram` removed. Config knobs `MaxPasteBytes` (1 MiB), `ResizeThrottle` (16ms), `MaxWindowDims` (4096×4096) with defaults on, plus `ConfigFromContext(ctx)` so middleware can read them. `Identity` interface + `WithIdentity` / `IdentityFromContext` for layer-1 → layers 2/3 propagation.
- **v0.3**: three-layer scaffolding. Ship `ConnectMiddleware` + `LiftHTTPMiddleware`, `SessionMiddleware`, layer-3 `Middleware`. Add config knobs (`MaxPasteBytes`, `ResizeThrottle`, `MaxWindowDims`) with defaults on.
- **v0.4**: built-in middleware — `osc52gate`, `activeterm`, `idletimeout` in main module; `sipmetrics` in a subpackage to isolate the Prometheus dep.

## The claim booba can honestly make once shipped

> booba is the only Go library that gives you:
> - **Wish-style handler ergonomics** for BubbleTea (layer 3)
> - **net/http middleware ecosystem reuse** via a transport-neutral handshake hook that works on both WebSocket and WebTransport (layer 1)
> - **VT/Sip-aware protocol middleware** for clipboard, paste, resize, and observability concerns that only exist when a terminal is on the other end of a browser connection (layer 2)

Layer 2 is the one no other ecosystem has. It is what justifies go-booba as more than "sip with WASM embedding".

## Open questions

- Should `ConnectMiddleware` have access to the `Config` (e.g., to see whether basic auth already ran)? Probably yes — pass via context.
- Do we want a way to expose the authenticated identity (from basic auth or a layer-1 middleware) to layer 2 and layer 3? Yes — likely a context value on the `Session.Context()` populated by layer 1.
- Should `sipmetrics` be a subpackage, a separate module, or vendored behind a build tag? Subpackage is simplest; separate module is cleanest but adds release coordination. Build tag is ugly. Lean subpackage.
- Do we want a "handshake response" shape richer than just `error`, e.g., for rejecting with a body or custom status? If so, a `ConnectError` type wrapping `code int, msg string` — map to HTTP status on WS, QUIC error code on WT.
