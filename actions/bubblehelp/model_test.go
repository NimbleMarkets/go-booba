package bubblehelp

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestProgramStatusSupportReplies(t *testing.T) {
	for _, ending := range []string{"\a", "\x1b\\"} {
		for _, payload := range []string{
			"?",
			"?:states=idle,working,done,blocked,error:kinds=permission,question,auth",
			"?:kinds=question:states=idle,working:future=value",
		} {
			reply := "\x1b]7501;" + payload + ending
			t.Run(reply, func(t *testing.T) {
				m, err := NewModel(testModel{}, func(tea.Model) Description {
					return Description{Context: "list", Keys: keymap{}, Status: actions.ProgramStatus{Summary: "Ready"}}
				}, ModelOptions{Instance: "test"})
				if err != nil {
					t.Fatal(err)
				}
				_, cmd := m.Update(uv.UnknownOscEvent(reply))
				if !m.supported || cmd == nil {
					t.Fatal("reply did not enable publication")
				}
				raw, ok := cmd().(tea.RawMsg)
				if !ok {
					t.Fatal("expected status publication")
				}
				report, err := actions.DecodeProgramStatus(raw.Msg.(string))
				if err != nil || report.Snapshot == nil {
					t.Fatalf("publication: %+v, %v", report, err)
				}
			})
		}
	}
	for _, reply := range []string{
		"\x1b]7501;?:states=idle", // Missing terminator.
		"\x1b]7501;?garbage\a",
		"\x1b]7501;state=idle\a",
		"\x1b]7502;?:states=idle\a",
	} {
		if IsProgramStatusSupport(uv.UnknownOscEvent(reply)) {
			t.Errorf("accepted unrelated or incomplete reply %q", reply)
		}
	}
	if IsProgramStatusSupport(nil) || IsProgramStatusSupport(tea.KeyPressMsg{Code: '?'}) {
		t.Fatal("accepted a non-OSC event")
	}
}

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
