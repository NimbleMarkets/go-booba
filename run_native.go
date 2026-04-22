//go:build !js

package booba

import (
	tea "charm.land/bubbletea/v2"
)

// Program wraps a BubbleTea program for the native terminal.
type Program struct {
	prog *tea.Program
}

// NewProgram creates a new BubbleTea program for the native terminal.
func NewProgram(model tea.Model, opts ...tea.ProgramOption) *Program {
	return &Program{
		prog: tea.NewProgram(model, opts...),
	}
}

// Run starts the program and blocks until it exits.
func (p *Program) Run() (tea.Model, error) {
	return p.prog.Run()
}

// Send sends a message to the program.
func (p *Program) Send(msg tea.Msg) {
	p.prog.Send(msg)
}

// Quit sends a quit message to the program.
func (p *Program) Quit() {
	p.prog.Quit()
}

// Kill immediately exits the program.
func (p *Program) Kill() {
	p.prog.Kill()
}

// Wait waits for the program to finish.
func (p *Program) Wait() {
	p.prog.Wait()
}

// ReleaseTerminal releases the terminal from the BubbleTea session.
func (p *Program) ReleaseTerminal() error {
	return p.prog.ReleaseTerminal()
}

// RestoreTerminal restores the terminal after a ReleaseTerminal call.
func (p *Program) RestoreTerminal() error {
	return p.prog.RestoreTerminal()
}

// Println prints a line above the program output.
func (p *Program) Println(args ...any) {
	p.prog.Println(args...)
}

// Printf prints formatted text above the program output.
func (p *Program) Printf(template string, args ...any) {
	p.prog.Printf(template, args...)
}

// Run executes the given BubbleTea model with the appropriate runtime
// for the build target.
func Run(model tea.Model, opts ...tea.ProgramOption) error {
	_, err := NewProgram(model, opts...).Run()
	return err
}
