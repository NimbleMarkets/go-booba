package bubblehelp

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
)

type testModel struct{}

func (testModel) Init() tea.Cmd                         { return nil }
func (m testModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (testModel) View() tea.View                        { return tea.NewView("") }

func TestNewModelValidatesInitialDescription(t *testing.T) {
	bindings := []key.Binding{}
	for _, k := range "abcdefghijk" {
		bindings = append(bindings, key.NewBinding(key.WithKeys(string(k)), key.WithHelp(string(k), strings.Repeat("x", 200))))
	}
	describe := func(tea.Model) Description {
		return Description{Context: "list", Keys: keymap{short: bindings}, Status: actions.ProgramStatus{Summary: "Ready"}}
	}
	if _, err := NewModel(testModel{}, describe, ModelOptions{}); err == nil || !strings.Contains(err.Error(), "2048") {
		t.Fatalf("expected construction size error, got %v", err)
	}
	bindings = bindings[:1]
	if _, err := NewModel(testModel{}, describe, ModelOptions{}); err != nil {
		t.Fatal(err)
	}
}
