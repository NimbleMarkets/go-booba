//go:build js && wasm

package wasm

import "os"

// In browser WASM there's no shell environment, so libraries that gate
// capability probes on env reads (e.g. TERM_PROGRAM, COLORTERM) see empty
// strings and silently disable themselves. ghostty-web — the rendering
// target booba ships in the browser — is fully capable, so we surface
// that as a positive signal during package init, before any consumer
// code runs. Consumers that want different detection can override with
// os.Setenv from their own init() or main().
//
// What we set, and why:
//   - TERM_PROGRAM=ghostty: most honest signal — ghostty-web really is
//     the terminal program rendering this output. Picked up by libraries
//     that detect Kitty graphics, hyperlinks, and similar features.
//   - COLORTERM=truecolor: ghostty-web supports 24-bit color. Many TUI
//     libraries gate truecolor output on this.
//   - CLICOLOR_FORCE=1: ensures color output even if the output stream
//     appears redirected. Prevents TUI libraries from disabling colors
//     in the browser context.
//
// What we deliberately don't set:
//   - TERM=xterm-ghostty: would require the matching terminfo entry on
//     the WASM filesystem, which doesn't exist; libraries that do
//     terminfo lookups would fail.
//   - KITTY_WINDOW_ID, GHOSTTY_RESOURCES_DIR: imply resource paths that
//     don't exist in WASM.
func init() {
	if os.Getenv("TERM_PROGRAM") == "" {
		_ = os.Setenv("TERM_PROGRAM", "ghostty")
	}
	if os.Getenv("COLORTERM") == "" {
		_ = os.Setenv("COLORTERM", "truecolor")
	}
	if os.Getenv("CLICOLOR_FORCE") == "" {
		_ = os.Setenv("CLICOLOR_FORCE", "1")
	}
}
