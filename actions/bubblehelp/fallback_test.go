package bubblehelp_test

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
	"github.com/NimbleMarkets/go-booba/actions/bubblehelp"
	"github.com/NimbleMarkets/go-booba/internal/actiontest"
	uv "github.com/charmbracelet/ultraviolet"
)

type model struct{ updates int }

func (*model) Init() tea.Cmd                         { return nil }
func (m *model) Update(tea.Msg) (tea.Model, tea.Cmd) { m.updates++; return m, nil }
func (*model) View() tea.View                        { return tea.NewView("map viewer") }

func TestOversizedMapFallbackAndRecovery(t *testing.T) {
	inner := &model{}
	d := actiontest.Describe(inner)
	describe := func(tea.Model) bubblehelp.Description { return d }
	if _, err := bubblehelp.NewModel(inner, describe, bubblehelp.ModelOptions{}); err == nil || !strings.Contains(err.Error(), "2048") {
		t.Fatalf("strict constructor: %v", err)
	}
	var diagnostics []error
	m, err := bubblehelp.NewModelWithFallback(inner, describe, bubblehelp.ModelOptions{Instance: "map", OnError: func(err error) { diagnostics = append(diagnostics, err) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Error(), "keyboard controls remain available") {
		t.Fatalf("diagnostics: %v", diagnostics)
	}
	// A capability reply clears controls, and repeated failures do not spam diagnostics.
	_, cmd := m.Update(statusSupport())
	assertClear(t, cmd)
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if inner.updates != 1 || m.View().Content != "map viewer" || len(diagnostics) != 1 {
		t.Fatal("fallback changed model behavior or repeated diagnostic")
	}
	// A smaller current description recovers publication without restarting.
	keys := actiontest.MapKeys()
	keys.More = nil
	d.Keys = keys
	_, cmd = m.Update(statusSupport())
	if cmd == nil {
		t.Fatal("no recovered publication")
	}
	raw, ok := cmd().(tea.RawMsg)
	if !ok {
		t.Fatalf("expected raw publication: %T", cmd())
	}
	report, err := actions.DecodeProgramStatus(raw.Msg.(string))
	if err != nil || report.Snapshot == nil || len(report.Snapshot.Actions) != 5 {
		t.Fatalf("recovery: %+v %v", report, err)
	}
	for i, a := range report.Snapshot.Actions {
		if a.Keys[0].Key != keys.Primary[i].Keys()[0] {
			t.Fatal("binding changed")
		}
	}
	d = actiontest.Describe(inner)
	_, cmd = m.Update(statusSupport())
	assertClear(t, cmd)
	if len(diagnostics) != 2 {
		t.Fatal("new failure after recovery not reported")
	}
}

func assertClear(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("missing clear")
	}
	raw, ok := cmd().(tea.RawMsg)
	if !ok {
		t.Fatal("clear is not raw output")
	}
	r, err := actions.DecodeProgramStatus(raw.Msg.(string))
	if err != nil || !r.Clear {
		t.Fatalf("clear: %+v %v", r, err)
	}
}

func TestFallbackConfigurationAndMetadataErrors(t *testing.T) {
	for _, opts := range []bubblehelp.ModelOptions{{Instance: "invalid instance"}} {
		if _, err := bubblehelp.NewModelWithFallback(&model{}, actiontest.Describe, opts); err == nil {
			t.Fatal("invalid instance accepted")
		}
	}
	if _, err := bubblehelp.NewModelWithFallback(nil, actiontest.Describe, bubblehelp.ModelOptions{}); err == nil {
		t.Fatal("nil model accepted")
	}
	if _, err := bubblehelp.NewModelWithFallback(&model{}, nil, bubblehelp.ModelOptions{}); err == nil {
		t.Fatal("nil callback accepted")
	}
	sentinel := errors.New("bad metadata")
	var got error
	opts := bubblehelp.ModelOptions{OnError: func(err error) { got = err }, Adapter: bubblehelp.Options{Customize: func(_ key.Binding, _ *actions.Action) error { return sentinel }}}
	if _, err := bubblehelp.NewModelWithFallback(&model{}, actiontest.Describe, opts); err != nil || !errors.Is(got, sentinel) {
		t.Fatalf("fallback: %v, diagnostic %v", err, got)
	}
	if _, err := bubblehelp.NewModel(&model{}, actiontest.Describe, opts); !errors.Is(err, sentinel) {
		t.Fatalf("strict: %v", err)
	}
}

func statusSupport() tea.Msg { return uv.UnknownOscEvent(actions.QueryProgramStatus) }

func TestFallbackDefaultDiagnostic(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	if _, err := bubblehelp.NewModelWithFallback(&model{}, actiontest.Describe, bubblehelp.ModelOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "2048") || !strings.Contains(output.String(), "keyboard controls remain available") {
		t.Fatalf("missing default diagnostic: %s", output.String())
	}
}

func TestFallbackRejectsInvalidStatusWithoutPublication(t *testing.T) {
	keys := actiontest.MapKeys()
	keys.More = nil
	d := bubblehelp.Description{Context: "map", Keys: keys, Status: actions.ProgramStatus{State: "bogus", Summary: "Ready"}}
	describe := func(tea.Model) bubblehelp.Description { return d }
	if _, err := bubblehelp.NewModel(&model{}, describe, bubblehelp.ModelOptions{}); err == nil {
		t.Fatal("strict constructor accepted invalid status")
	}
	var diagnostic error
	m, err := bubblehelp.NewModelWithFallback(&model{}, describe, bubblehelp.ModelOptions{OnError: func(err error) { diagnostic = err }})
	if err != nil || diagnostic == nil {
		t.Fatalf("constructor=%v diagnostic=%v", err, diagnostic)
	}
	_, cmd := m.Update(statusSupport())
	assertClear(t, cmd)
}
