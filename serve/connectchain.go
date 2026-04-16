//go:build !js

package serve

import "net/http"

// runConnectChain builds the outermost-first chain over the given
// middleware list and a terminal handler that captures the final
// *http.Request. It returns the captured request (whose Context() may
// have been decorated by middleware) and any error returned by the
// chain.
//
// The captured request is meaningful only when the chain returns nil —
// on error the framework writes a rejection response and discards it.
func runConnectChain(r *http.Request, mws []ConnectMiddleware) (*http.Request, error) {
	var captured *http.Request
	terminal := func(r *http.Request) error {
		captured = r
		return nil
	}
	chain := ConnectHandler(terminal)
	for i := len(mws) - 1; i >= 0; i-- {
		chain = mws[i](chain)
	}
	if err := chain(r); err != nil {
		return nil, err
	}
	if captured == nil {
		// Should be unreachable: chain returned nil, but terminal was
		// not called. Fall back to the original request.
		captured = r
	}
	return captured, nil
}
