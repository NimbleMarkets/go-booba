//go:build !js

package serve

import (
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// validateBasicAuth reports whether r carries credentials that match
// the configured username and password. If both are empty, auth is
// skipped and the result is true. Comparisons use
// crypto/subtle.ConstantTimeCompare so response timing does not leak
// the configured secret.
func validateBasicAuth(r *http.Request, username, password string) bool {
	if username == "" && password == "" {
		return true
	}
	u, p, ok := r.BasicAuth()
	if !ok {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(u), []byte(username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(p), []byte(password)) == 1
	return userOK && passOK
}

// basicAuthMiddleware returns a ConnectMiddleware that performs HTTP
// Basic Auth using the configured username and password. Returns
// *ConnectError{Status: 401, Headers: WWW-Authenticate, Body: "Unauthorized"}
// on failure.
func basicAuthMiddleware(username, password string) ConnectMiddleware {
	return func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			if !validateBasicAuth(r, username, password) {
				headers := make(http.Header)
				headers.Add("WWW-Authenticate", `Basic realm="booba"`)
				return &ConnectError{
					Status:  http.StatusUnauthorized,
					Headers: headers,
					Body:    "Unauthorized",
				}
			}
			return next(r)
		}
	}
}

// connLimitMiddleware returns a ConnectMiddleware that gates connections
// against srv.config.MaxConnections. Acquires on success (tracking the
// connection in srv.connCount even when MaxConnections <= 0 so the
// handler's deferred srv.releaseConnection() pairs up unconditionally).
// The caller is responsible for invoking srv.releaseConnection() when
// the connection is closed.
func connLimitMiddleware(srv *Server) ConnectMiddleware {
	return func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			if !srv.tryAcquireConnection() {
				return &ConnectError{
					Status: http.StatusServiceUnavailable,
					Body:   "max connections reached",
				}
			}
			if err := next(r); err != nil {
				srv.releaseConnection()
				return err
			}
			return nil
		}
	}
}

// idleTimeoutMiddleware returns a SessionMiddleware that closes the
// wrapped Session if no inbound bytes are received for d. Inbound
// means client-to-session writes on InputWriter; outbound activity
// does NOT reset the timer.
//
// A d <= 0 makes the middleware a no-op (returns the session
// unwrapped) so callers can install it unconditionally.
func idleTimeoutMiddleware(d time.Duration) SessionMiddleware {
	return func(base Session) Session {
		if d <= 0 {
			return base
		}
		w := &idleSession{Session: base, timer: time.NewTimer(d), duration: d}
		go w.watch()
		return w
	}
}

type idleSession struct {
	Session
	timer    *time.Timer
	duration time.Duration
}

// InputWriter wraps the underlying writer to reset the idle timer on
// every inbound write.
func (s *idleSession) InputWriter() io.Writer {
	return &idleResetWriter{inner: s.Session.InputWriter(), sess: s}
}

func (s *idleSession) watch() {
	defer s.timer.Stop()
	for {
		select {
		case <-s.Done():
			return
		case <-s.timer.C:
			slog.Default().Info("idle timeout", slog.Duration("after", s.duration))
			_ = s.Close()
			return
		}
	}
}

type idleResetWriter struct {
	inner io.Writer
	sess  *idleSession
}

func (w *idleResetWriter) Write(p []byte) (int, error) {
	// Go 1.23+ guarantees that calling Reset after Stop (even when
	// Stop returned false) is safe and deterministic — the next value
	// on the channel will correspond to the new duration, and the
	// watcher goroutine is the sole consumer so there is no stale
	// value to drain. If the session is already closed, the inner
	// writer will return an error on its own.
	w.sess.timer.Stop()
	w.sess.timer.Reset(w.sess.duration)
	return w.inner.Write(p)
}
