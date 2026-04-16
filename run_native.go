//go:build !js

package booba

import (
	tea "charm.land/bubbletea/v2"
)

// Run executes the given BubbleTea model with the appropriate runtime
// for the build target.
func Run(model tea.Model, opts ...tea.ProgramOption) error {
	_, err := tea.NewProgram(model, opts...).Run()
	return err
}
