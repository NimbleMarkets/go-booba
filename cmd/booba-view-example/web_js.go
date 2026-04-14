//go:build js

package main

// startWebServerIfRequested is a no-op on js/wasm — the serve package
// requires Unix PTYs which are not available in the browser.
func startWebServerIfRequested() bool {
	return false
}
