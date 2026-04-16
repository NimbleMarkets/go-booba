//go:build js && wasm

// Package wasm provides a bridge for running BubbleTea programs in the browser.
//
// It registers JavaScript functions on window (bubbletea_read, bubbletea_write,
// bubbletea_resize) that booba's BoobaWasmAdapter polls to shuttle data between
// the ghostty-web terminal emulator and the Go program.
//
// Usage:
//
//	//go:build js && wasm
//	package main
//
//	import "github.com/NimbleMarkets/booba/wasm"
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

// Run creates a BubbleTea program from the given model, registers the
// JavaScript bridge functions, and blocks until the program exits.
//
// Additional tea.ProgramOption values can be passed to configure the
// program (e.g., tea.WithMouseCellMotion(), tea.WithAltScreen()).
func Run(model tea.Model, opts ...tea.ProgramOption) error {
	fromJS := &syncBuffer{}
	toJS := &syncBuffer{}

	baseOpts := []tea.ProgramOption{
		tea.WithInput(fromJS),
		tea.WithOutput(toJS),
	}

	prog := tea.NewProgram(model, append(baseOpts, opts...)...)

	js.Global().Set("bubbletea_write", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			fromJS.Write([]byte(args[0].String()))
		}
		return nil
	}))

	js.Global().Set("bubbletea_read", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		data := toJS.ReadAndReset()
		if len(data) == 0 {
			return ""
		}
		return string(data)
	}))

	js.Global().Set("bubbletea_resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) >= 2 {
			prog.Send(tea.WindowSizeMsg{
				Width:  args[0].Int(),
				Height: args[1].Int(),
			})
		}
		return nil
	}))

	_, err := prog.Run()
	return err
}

// syncBuffer is a goroutine-safe buffer for bridging Go I/O with
// JavaScript's single-threaded polling.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Read(p)
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
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
