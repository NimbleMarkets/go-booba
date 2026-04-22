// Package booba provides a platform-polymorphic entry point for running
// BubbleTea programs that target both native terminals and web browsers.
//
// The same main.go works on both targets: when compiled for a native
// platform, [Run] runs the model with a standard tea.Program; when
// compiled with GOOS=js GOARCH=wasm, [Run] installs the JavaScript
// bridge that booba's web terminal uses.
//
//	package main
//
//	import (
//	    "log"
//
//	    booba "github.com/NimbleMarkets/go-booba"
//	)
//
//	func main() {
//	    if err := booba.Run(initialModel()); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// For finer control — or to match the original Bubble Tea API during
// porting — use [NewProgram]:
//
//	bp := booba.NewProgram(initialModel())
//	if _, err := bp.Run(); err != nil {
//	    log.Fatal(err)
//	}
//
// For more granular control, subpackages are available:
//   - [github.com/NimbleMarkets/go-booba/wasm] exposes the browser bridge directly.
//   - [github.com/NimbleMarkets/go-booba/serve] runs a BubbleTea program as an
//     HTTP/WebSocket/WebTransport backend for browser clients.
package booba
