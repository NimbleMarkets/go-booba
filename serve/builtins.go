//go:build !js

package serve

import (
	"crypto/subtle"
	"net/http"
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
