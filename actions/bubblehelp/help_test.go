package bubblehelp

import (
	"charm.land/bubbles/v2/key"
	"github.com/NimbleMarkets/go-booba/actions"
	"testing"
)

type keymap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (k keymap) ShortHelp() []key.Binding  { return k.short }
func (k keymap) FullHelp() [][]key.Binding { return k.full }
func TestHelpIdentityAndGroups(t *testing.T) {
	open := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "Open"))
	nav := key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "Navigate"))
	disabled := key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "Delete"), key.WithDisabled())
	bad := key.NewBinding(key.WithKeys("ctrl+x enter"), key.WithHelp("x enter", "Macro"))
	km := keymap{[]key.Binding{open, disabled}, [][]key.Binding{{nav, open}, {bad, disabled}}}
	s, diag, err := Snapshot(km, "app", "list", 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Actions) != 2 || len(s.Full) != 1 || s.Short[0] != s.Full[0][1] || len(s.Actions[1].Keys) != 2 || len(diag) != 1 {
		t.Fatalf("snapshot=%+v diagnostics=%+v", s, diag)
	}
	km.short = []key.Binding{nav, open}
	next, _, err := Snapshot(km, "app", "list", 2, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if next.Short[1] != s.Short[0] {
		t.Fatal("identity changed on reorder")
	}
}
func TestOverridesAndDisabled(t *testing.T) {
	binding := key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "\x1b[31mMove up\x1b[0m"), key.WithDisabled())
	km := keymap{short: []key.Binding{binding}}
	s, _, err := Snapshot(km, "app", "list", 1, Options{IncludeDisabled: true, Customize: func(_ key.Binding, a *actions.Action) error { a.ID = "up"; p := 0; a.Preferred = &p; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	a := s.Actions[0]
	if a.Enabled || a.Label != "Move up" || a.ID != "up" || a.Preferred == nil {
		t.Fatalf("%+v", a)
	}
}
