package bubblehelp

import (
	"charm.land/bubbles/v2/key"
	"encoding/json"
	"github.com/NimbleMarkets/go-booba/actions"
	"strings"
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

func TestCompactIDsAndStatusLimit(t *testing.T) {
	bindings := []key.Binding{}
	for _, k := range "abcdefghijk" {
		bindings = append(bindings, key.NewBinding(key.WithKeys(string(k)), key.WithHelp(string(k), "Action "+string(k))))
	}
	km := keymap{bindings, [][]key.Binding{bindings}}
	s, _, err := Snapshot(km, "app", "list", 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range s.Actions {
		if len(a.ID) != 8 {
			t.Fatalf("expected eight-character ID, got %q", a.ID)
		}
	}
	if _, err := actions.EncodeProgramStatus(s, actions.ProgramStatus{Summary: "list"}); err != nil {
		t.Fatal(err)
	}
	// Each label is valid on its own, but the combined status is too large.
	_, _, err = Snapshot(km, "app", "list", 1, Options{Customize: func(_ key.Binding, a *actions.Action) error {
		a.Label = strings.Repeat("x", 200)
		return nil
	}})
	if err == nil || !strings.Contains(err.Error(), "2048") {
		t.Fatalf("expected OSC limit error, got %v", err)
	}
}

func TestStatusSummaryByteBoundary(t *testing.T) {
	binding := key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Action"))
	km := keymap{short: []key.Binding{binding}}
	// Repeated references and JSON escaping must count toward the decoded limit.
	for n := 1; n < 20; n++ {
		km.short = append(km.short, key.NewBinding(key.WithKeys("b"), key.WithHelp("b", strings.Repeat("界", n))))
	}
	s, _, _ := Snapshot(km, "app", "list", 1, Options{})
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	remaining := actions.MaxStatusMessageBytes - len(actions.MessageMarker) - len(data)
	// Use a smaller map if the fixture already exceeds the wire limit.
	for remaining < 1 || remaining > 255 {
		km.short = km.short[:len(km.short)-1]
		s, _, _ = Snapshot(km, "app", "list", 1, Options{})
		data, _ = json.Marshal(s)
		remaining = actions.MaxStatusMessageBytes - len(actions.MessageMarker) - len(data)
	}
	if _, _, err := Snapshot(km, "app", "list", 1, Options{Summary: strings.Repeat("x", remaining)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Snapshot(km, "app", "list", 1, Options{Summary: strings.Repeat("x", remaining+1)}); err == nil {
		t.Fatal("accepted oversized summary plus snapshot")
	}
}
