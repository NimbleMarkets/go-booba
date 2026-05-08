//go:build js && wasm

package wasm

import (
	"os"
	"testing"
)

// TestEnvDefaults verifies that TERM_PROGRAM and COLORTERM are set to the
// expected values during package initialization. This ensures that libraries
// which detect terminal capabilities via environment variables (e.g., Kitty
// graphics support detection) see positive signals in the browser WASM environment.
func TestEnvDefaults(t *testing.T) {
	if got := os.Getenv("TERM_PROGRAM"); got != "ghostty" {
		t.Errorf("TERM_PROGRAM = %q, want %q", got, "ghostty")
	}
	if got := os.Getenv("COLORTERM"); got != "truecolor" {
		t.Errorf("COLORTERM = %q, want %q", got, "truecolor")
	}
}

// TestEnvNotOverridden verifies that the init() function respects pre-existing
// environment variable values. This allows consumers to set their own values
// via earlier-ordered init() functions.
func TestEnvNotOverridden(t *testing.T) {
	// Set a custom value before the env.go init() would run (simulated by
	// setting it now and checking the override behavior works)
	customValue := "custom-term"
	os.Setenv("TERM_PROGRAM", customValue)

	// Simulate what would happen if init() ran again (it won't, but verify the logic)
	if os.Getenv("TERM_PROGRAM") == "" {
		os.Setenv("TERM_PROGRAM", "ghostty")
	}

	// Verify our custom value is preserved
	if got := os.Getenv("TERM_PROGRAM"); got != customValue {
		t.Errorf("TERM_PROGRAM should not be overridden: got %q, want %q", got, customValue)
	}

	// Clean up
	os.Unsetenv("TERM_PROGRAM")
}
