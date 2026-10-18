//go:build js && wasm

package booba_test

import (
	"syscall/js"
	"testing"
)

func checkActionBridge(t *testing.T) {
	t.Helper()
	for _, name := range []string{"bubbletea_read", "bubbletea_write", "bubbletea_resize"} {
		if js.Global().Get(name).Type() != js.TypeFunction {
			t.Fatalf("%s was not registered", name)
		}
	}
	if js.Global().Get("bubbletea_exited").Truthy() {
		t.Fatal("bridge already exited")
	}
}
