//go:build js && wasm

package booba

import (
	tea "charm.land/bubbletea/v2"

	"github.com/NimbleMarkets/go-booba/wasm"
)

// Run executes the given BubbleTea model with the appropriate runtime
// for the build target. On js/wasm it delegates to [wasm.Run].
func Run(model tea.Model, opts ...tea.ProgramOption) error {
	return wasm.Run(model, opts...)
}
