//go:build !js

package serve

import "net/http"

// basicAuthMiddleware returns a ConnectMiddleware that performs HTTP
// Basic Auth using the configured username and password. Returns
// *ConnectError{Status: 401, Headers: WWW-Authenticate, Body: "Unauthorized"}
// on failure.
func basicAuthMiddleware(username, password string) ConnectMiddleware {
	return func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			u, p, ok := r.BasicAuth()
			if !ok || u != username || p != password {
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
