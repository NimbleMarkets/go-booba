//go:build !js

package serve

import (
	"errors"
	"net/http"
)

// errChainBroken is returned by runConnectChain when a middleware
// approved the connection (returned nil) without invoking next — the
// contract on ConnectHandler requires that a middleware either call
// next or return a non-nil error.
var errChainBroken = errors.New("serve: connect chain broken: middleware approved without calling next")

// runConnectChain builds the outermost-first chain over the given
// middleware list and a terminal handler that captures the final
// *http.Request. It returns the captured request (whose Context() may
// have been decorated by middleware) and any error returned by the
// chain.
//
// The captured request is meaningful only when the chain returns nil —
// on error the framework writes a rejection response and discards it.
// If a middleware approves the connection without invoking next, the
// chain is broken (inner middleware's context decorations would be
// silently lost); runConnectChain returns errChainBroken in that case.
//
// Note: used by unit tests only. Production handlers use runLiftedChain
// (see lift.go) so that LiftHTTPMiddleware can write responses directly.
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
		return nil, errChainBroken
	}
	return captured, nil
}
