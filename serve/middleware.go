//go:build !js

package serve

import (
	"context"
	"net/http"
)

// ConnectHandler is invoked at the handshake boundary for both
// WebSocket upgrades and WebTransport CONNECTs. It runs on the raw
// *http.Request before the upgrade is performed. Returning a non-nil
// error rejects the connection; returning *ConnectError gives full
// control over the rejection response.
//
// Middleware that wants to attach request-scoped values (e.g. an
// authenticated identity) does so by replacing r before calling next:
//
//	r = r.WithContext(serve.WithIdentity(r.Context(), id))
//	return next(r)
//
// The framework's terminal handler captures the *http.Request as last
// seen, so context updates propagate to layer 2 and layer 3.
type ConnectHandler func(r *http.Request) error

// ConnectMiddleware decorates a ConnectHandler.
type ConnectMiddleware func(next ConnectHandler) ConnectHandler

// SessionMiddleware decorates a Session. Compose using Go's
// interface-embedding idiom:
//
//	type myMW struct{ serve.Session }
//	func (m *myMW) OutputReader() io.Reader { ... wrap m.Session.OutputReader() ... }
//
// SessionMiddleware values are applied in install order; the first
// middleware installed is the outermost wrapper.
type SessionMiddleware func(Session) Session

// Middleware decorates a Handler. Matches the shape of
// charmbracelet/wish bubbletea.Middleware.
type Middleware func(next Handler) Handler

// silence unused import on stripped builds
var _ = context.Background
