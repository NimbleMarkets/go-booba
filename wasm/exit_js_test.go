//go:build js && wasm

package wasm

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// quitModel renders one line and quits one frame after it learns the window
// size; on js the renderer needs that size before it paints anything.
type quitModel struct{}

type quitNow struct{}

func (quitModel) Init() tea.Cmd { return nil }
func (quitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.WindowSizeMsg:
		return quitModel{}, tea.Sequence(
			tea.Println("farewell from the model"),
			tea.Tick(20*time.Millisecond, func(time.Time) tea.Msg { return quitNow{} }),
		)
	case quitNow:
		return quitModel{}, tea.Quit
	}
	return quitModel{}, nil
}
func (quitModel) View() tea.View { return tea.NewView("farewell from the model") }

// TestRunPublishesExitToJavaScript pins the contract the browser adapter
// relies on: once the program has returned no Go function can be called, so
// Run must hand the unread output to JavaScript and flag the exit first.
func TestRunPublishesExitToJavaScript(t *testing.T) {
	global := js.Global()
	global.Set("bubbletea_exited", true) // stale value from an earlier program
	global.Set("bubbletea_final", "stale")

	p := NewProgram(quitModel{})
	if global.Get("bubbletea_exited").Truthy() {
		t.Fatal("NewProgram must reset bubbletea_exited")
	}
	if global.Get("bubbletea_final").Truthy() {
		t.Fatal("NewProgram must clear bubbletea_final")
	}

	// Send blocks until the event loop starts, so deliver the size from a goroutine.
	go p.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	if _, err := p.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	exited := global.Get("bubbletea_exited")
	if exited.Type() != js.TypeBoolean || !exited.Bool() {
		t.Fatalf("bubbletea_exited = %v, want true", exited)
	}
	final := global.Get("bubbletea_final")
	if final.Type() != js.TypeString {
		t.Fatalf("bubbletea_final has type %v, want string", final.Type())
	}
	if !strings.Contains(final.String(), "farewell from the model") {
		t.Fatalf("bubbletea_final = %q, want the final frame", final.String())
	}
}

// TestSupersededProgramDoesNotPublishExit covers a page that starts a second
// program while the first still runs (a Restart button). The second NewProgram
// takes over the bridge, so the first program's later exit must not flip
// bubbletea_exited or replace bubbletea_final under the program that is live.
func TestSupersededProgramDoesNotPublishExit(t *testing.T) {
	global := js.Global()
	first := NewProgram(quitModel{})
	NewProgram(quitModel{}) // supersedes first on the bridge

	go first.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	if _, err := first.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if global.Get("bubbletea_exited").Truthy() {
		t.Fatal("a superseded program must not set bubbletea_exited")
	}
	if final := global.Get("bubbletea_final"); final.Truthy() {
		t.Fatalf("a superseded program must not publish bubbletea_final, got %q", final.String())
	}
}
