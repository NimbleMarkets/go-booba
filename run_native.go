//go:build !js

package booba

import (
	tea "charm.land/bubbletea/v2"
)

// Program wraps a BubbleTea program for the native terminal.
type Program struct {
	tea *tea.Program
	*tea.Program
}

// NewProgram creates a new BubbleTea program for the native terminal.
func NewProgram(model tea.Model, opts ...tea.ProgramOption) *Program {
	p := tea.NewProgram(model, opts...)
	return &Program{
		tea:     p,
		Program: p,
	}
}

// TeaProgram returns the underlying *tea.Program for use with functions
// expecting the tea.Program type.
func (p *Program) TeaProgram() *tea.Program {
	return p.tea
}

// Run executes the given BubbleTea model with the appropriate runtime
// for the build target.
func Run(model tea.Model, opts ...tea.ProgramOption) error {
	_, err := NewProgram(model, opts...).Run()
	return err
}
