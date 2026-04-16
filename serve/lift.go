//go:build !js

package serve

import (
	"context"
	"errors"
	"net/http"
)

// errResponseWritten is the sentinel returned by LiftHTTPMiddleware-
// adapted middleware when the lifted middleware writes a response
// directly instead of calling next. The framework treats it as
// "rejected, response already sent" and stops without writing again.
var errResponseWritten = errors.New("serve: response already written by lifted middleware")

type liftCtxKey struct{}

type liftBridge struct {
	w http.ResponseWriter
}

// statusCapturingWriter wraps http.ResponseWriter to record whether
// WriteHeader or Write was called.
type statusCapturingWriter struct {
	http.ResponseWriter
	wrote bool
}

func (s *statusCapturingWriter) WriteHeader(code int) {
	s.wrote = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusCapturingWriter) Write(p []byte) (int, error) {
	s.wrote = true
	return s.ResponseWriter.Write(p)
}

// LiftHTTPMiddleware adapts a standard func(http.Handler) http.Handler
// into a ConnectMiddleware so existing net/http middleware (chi,
// gorilla, otelhttp, prometheus, tollbooth, ...) can run on the booba
// handshake boundary.
//
// Outcomes:
//   - The lifted middleware calls next: the adapter invokes the booba
//     ConnectHandler chain and returns its result.
//   - The lifted middleware writes a response and does not call next:
//     the adapter returns errResponseWritten so the framework stops
//     without writing again.
//
// Lifted middleware MUST NOT inspect the response after next returns —
// there is no response after upgrade.
func LiftHTTPMiddleware(mw func(http.Handler) http.Handler) ConnectMiddleware {
	return func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			b, ok := r.Context().Value(liftCtxKey{}).(*liftBridge)
			if !ok {
				// Lift was invoked outside runLiftedChain — fall back to
				// just running next.
				return next(r)
			}
			scw := &statusCapturingWriter{ResponseWriter: b.w}
			var called bool
			var nextErr error
			handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r2 *http.Request) {
				called = true
				nextErr = next(r2)
			}))
			handler.ServeHTTP(scw, r)
			if called {
				return nextErr
			}
			if scw.wrote {
				return errResponseWritten
			}
			// Middleware neither called next nor wrote a response — treat
			// as a programmer error.
			return &ConnectError{Status: http.StatusInternalServerError}
		}
	}
}

// runLiftedChain runs the connect chain with the per-request lift bridge
// installed in context. Used by handleWS / handleWT to enable
// LiftHTTPMiddleware to write responses directly.
func runLiftedChain(w http.ResponseWriter, r *http.Request, mws []ConnectMiddleware, terminal ConnectHandler) error {
	bridge := &liftBridge{w: w}
	r = r.WithContext(context.WithValue(r.Context(), liftCtxKey{}, bridge))
	chain := terminal
	for i := len(mws) - 1; i >= 0; i-- {
		chain = mws[i](chain)
	}
	return chain(r)
}
