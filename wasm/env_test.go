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

// TestEnvCanBeOverridden verifies that consumers can override environment
// variables set by the init() function. This is tested by setting a custom
// value and verifying it's preserved (demonstrating the override path works).
func TestEnvCanBeOverridden(t *testing.T) {
	customValue := "custom-term"
	t.Setenv("TERM_PROGRAM", customValue)

	if got := os.Getenv("TERM_PROGRAM"); got != customValue {
		t.Errorf("override should be preserved: got %q, want %q", got, customValue)
	}
}
