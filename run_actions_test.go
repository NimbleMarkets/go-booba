package booba_test

import (
	"io"
	"testing"

	tea "charm.land/bubbletea/v2"
	booba "github.com/NimbleMarkets/go-booba"
	"github.com/NimbleMarkets/go-booba/actions/bubblehelp"
	"github.com/NimbleMarkets/go-booba/internal/actiontest"
)

type startupModel struct {
	t                *testing.T
	started, handled bool
}

func (m *startupModel) Init() tea.Cmd {
	m.started = true
	checkActionBridge(m.t)
	return func() tea.Msg { return tea.KeyPressMsg{Code: 'w', Text: "w"} }
}
func (m *startupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "w" {
		m.handled = true
		return m, tea.Quit
	}
	return m, nil
}
func (*startupModel) View() tea.View { return tea.NewView("map viewer") }

func TestRunWithOversizedOptionalActions(t *testing.T) {
	inner := &startupModel{t: t}
	diagnostics := 0
	wrapped, err := bubblehelp.NewModelWithFallback(inner, actiontest.Describe, bubblehelp.ModelOptions{OnError: func(error) { diagnostics++ }})
	if err != nil {
		t.Fatal(err)
	}
	if err := booba.Run(wrapped, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler()); err != nil {
		t.Fatal(err)
	}
	if !inner.started || !inner.handled || diagnostics != 1 {
		t.Fatalf("started=%v handled=%v diagnostics=%d", inner.started, inner.handled, diagnostics)
	}
}
