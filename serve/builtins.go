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
