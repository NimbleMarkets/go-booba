//go:build !js

package serve

import (
	"crypto/subtle"
	"net/http"
)

// basicAuthMiddleware returns a ConnectMiddleware that performs HTTP
// Basic Auth using the configured username and password. Returns
// *ConnectError{Status: 401, Headers: WWW-Authenticate, Body: "Unauthorized"}
// on failure.
//
// Credential comparison uses crypto/subtle.ConstantTimeCompare to avoid
// leaking the configured secret via response timing.
func basicAuthMiddleware(username, password string) ConnectMiddleware {
	expectedUser := []byte(username)
	expectedPass := []byte(password)
	return func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			u, p, ok := r.BasicAuth()
			userOK := subtle.ConstantTimeCompare([]byte(u), expectedUser) == 1
			passOK := subtle.ConstantTimeCompare([]byte(p), expectedPass) == 1
			if !ok || !userOK || !passOK {
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
