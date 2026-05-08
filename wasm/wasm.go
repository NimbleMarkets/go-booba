//go:build js && wasm

// Package wasm provides a bridge for running BubbleTea programs in the browser.
//
// It registers JavaScript functions on window (bubbletea_read, bubbletea_write,
// bubbletea_resize) that booba's BoobaWasmAdapter polls to shuttle data between
// the ghostty-web terminal emulator and the Go program.
//
// Environment synthesis:
//
// During package initialization (before any consumer code runs), this package
// sets TERM_PROGRAM=ghostty, COLORTERM=truecolor, and CLICOLOR_FORCE=1 in the
// Go environment, unless they're already set. This allows libraries that detect
// terminal capabilities via environment variables to discover that the rendering
// target (ghostty-web) supports advanced features like Kitty graphics protocol
// and truecolor output, and ensures color output is not disabled. Consumers can
// override these values with os.Setenv before calling Run if needed.
//
// Usage:
//
//	//go:build js && wasm
//	package main
//
//	import "github.com/NimbleMarkets/go-booba/wasm"
//
//	func main() {
//	    wasm.Run(initialModel())
//	}
package wasm

import (
	"bytes"
	"sync"
	"syscall/js"

	tea "charm.land/bubbletea/v2"
)

// Program wraps a BubbleTea program for the browser.
type Program struct {
	tea *tea.Program
	*tea.Program
	fromJS     *syncBuffer
	toJS       *syncBuffer
	writeFunc  js.Func
	readFunc   js.Func
	resizeFunc js.Func
}

// NewProgram creates a new BubbleTea program for the browser.
// It sets up the JavaScript bridge functions but does not start the program.
func NewProgram(model tea.Model, opts ...tea.ProgramOption) *Program {
	fromJS := newSyncBuffer()
	toJS := newSyncBuffer()

	baseOpts := []tea.ProgramOption{
		tea.WithInput(fromJS),
		tea.WithOutput(toJS),
	}

	teaProg := tea.NewProgram(model, append(baseOpts, opts...)...)

	p := &Program{
		tea:     teaProg,
		Program: teaProg,
		fromJS:  fromJS,
		toJS:    toJS,
	}

	p.writeFunc = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			p.fromJS.Write([]byte(args[0].String()))
		}
		return nil
	})
	js.Global().Set("bubbletea_write", p.writeFunc)

	p.readFunc = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		data := p.toJS.ReadAndReset()
		if len(data) == 0 {
			return ""
		}
		return string(data)
	})
	js.Global().Set("bubbletea_read", p.readFunc)

	p.resizeFunc = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) >= 2 {
			p.Send(tea.WindowSizeMsg{
				Width:  args[0].Int(),
				Height: args[1].Int(),
			})
		}
		return nil
	})
	js.Global().Set("bubbletea_resize", p.resizeFunc)

	return p
}

// Run starts the program and blocks until it exits, cleaning up JS functions.
func (p *Program) Run() (tea.Model, error) {
	defer p.writeFunc.Release()
	defer p.readFunc.Release()
	defer p.resizeFunc.Release()

	return p.Program.Run()
}

// TeaProgram returns the underlying *tea.Program for use with functions
// expecting the tea.Program type.
func (p *Program) TeaProgram() *tea.Program {
	return p.tea
}

// ReleaseTerminal is a no-op in the browser.
func (p *Program) ReleaseTerminal() error {
	return nil
}

// RestoreTerminal is a no-op in the browser.
func (p *Program) RestoreTerminal() error {
	return nil
}

// Run creates a BubbleTea program from the given model, registers the
// JavaScript bridge functions, and blocks until the program exits.
//
// Additional tea.ProgramOption values can be passed to configure the
// program (e.g., tea.WithMouseCellMotion(), tea.WithAltScreen()).
func Run(model tea.Model, opts ...tea.ProgramOption) error {
	_, err := NewProgram(model, opts...).Run()
	return err
}

// syncBuffer is a goroutine-safe buffer for bridging Go I/O with
// JavaScript's single-threaded polling.
type syncBuffer struct {
	mu   sync.Mutex
	cond *sync.Cond
	buf  bytes.Buffer
}

func newSyncBuffer() *syncBuffer {
	b := &syncBuffer{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *syncBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for b.buf.Len() == 0 {
		b.cond.Wait()
	}
	return b.buf.Read(p)
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.buf.Write(p)
	b.cond.Signal()
	return n, err
}

// ReadAndReset returns all buffered data and resets the buffer.
// Returns nil if empty.
func (b *syncBuffer) ReadAndReset() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() == 0 {
		return nil
	}
	data := make([]byte, b.buf.Len())
	copy(data, b.buf.Bytes())
	b.buf.Reset()
	return data
}
