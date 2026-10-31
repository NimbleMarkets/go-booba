// booba-actions-example uses one help map for terminal help and JSON actions.
// -snapshot files or -snapshot confirm prints a description for host previews.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	booba "github.com/NimbleMarkets/go-booba"
	"github.com/NimbleMarkets/go-booba/actions"
	"github.com/NimbleMarkets/go-booba/actions/bubblehelp"
)

type keymap struct {
	up, down, open, back, quit, more, visibility key.Binding
	confirm                                      bool
}

func binding(k, label string) key.Binding {
	return key.NewBinding(key.WithKeys(k), key.WithHelp(k, label))
}
func keys(confirm, hideHelp bool) keymap {
	label := "Hide TUI help"
	if hideHelp {
		label = "Show TUI help"
	}
	return keymap{
		up: binding("up", "Move up"), down: binding("down", "Move down"), open: binding("enter", "Open"),
		back: binding("esc", "Back"), quit: binding("ctrl+c", "Quit"), more: binding("?", "Expand help"), visibility: binding("h", label), confirm: confirm,
	}
}
func (k keymap) ShortHelp() []key.Binding {
	if k.confirm {
		return []key.Binding{binding("enter", "Confirm"), k.back, k.visibility}
	}
	return []key.Binding{k.open, k.visibility}
}
func (k keymap) FullHelp() [][]key.Binding {
	if k.confirm {
		return [][]key.Binding{k.ShortHelp(), {k.quit}}
	}
	return [][]key.Binding{{k.up, k.down, k.open}, {k.more, k.visibility, k.quit}}
}

type model struct {
	help     help.Model
	selected int
	confirm  bool
	message  string
	hideHelp bool
}

func (m model) Init() tea.Cmd { return nil }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.help.SetWidth(msg.Width)
	case tea.KeyPressMsg:
		k := keys(m.confirm, m.hideHelp)
		switch {
		case key.Matches(msg, k.quit):
			return m, tea.Quit
		case key.Matches(msg, k.visibility):
			m.hideHelp = !m.hideHelp
		case key.Matches(msg, k.more):
			m.hideHelp = false
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, k.back):
			m.confirm = false
		case key.Matches(msg, k.open):
			if m.confirm {
				m.message = fmt.Sprintf("Opened item %d", m.selected+1)
				m.confirm = false
			} else {
				m.confirm = true
			}
		case !m.confirm && key.Matches(msg, k.up):
			m.selected = (m.selected + 2) % 3
		case !m.confirm && key.Matches(msg, k.down):
			m.selected = (m.selected + 1) % 3
		}
	}
	return m, nil
}
func (m model) View() tea.View {
	var b strings.Builder
	if m.confirm {
		fmt.Fprintf(&b, "Open item %d?\n\n", m.selected+1)
	} else {
		b.WriteString("Choose an item\n\n")
		for i := 0; i < 3; i++ {
			cursor := "  "
			if i == m.selected {
				cursor = "> "
			}
			fmt.Fprintf(&b, "%sItem %d\n", cursor, i+1)
		}
		fmt.Fprintf(&b, "\n%s\n\n", m.message)
	}
	if !m.hideHelp {
		b.WriteString(m.help.View(keys(m.confirm, m.hideHelp)))
	}
	view := tea.NewView(b.String())
	view.AltScreen = true
	return view
}
func newActionModel() tea.Model {
	wrapped, err := bubblehelp.NewModel(model{help: help.New()}, func(inner tea.Model) bubblehelp.Description {
		m := inner.(model)
		context, status := "files", actions.ProgramStatus{App: "booba-demo", Summary: "Choose an item"}
		if m.confirm {
			context = "confirm"
			status.State = "blocked"
			status.Kind = "question"
			status.Summary = fmt.Sprintf("Open item %d?", m.selected+1)
		}
		return bubblehelp.Description{Context: context, Keys: keys(m.confirm, m.hideHelp), Status: status}
	}, bubblehelp.ModelOptions{OnError: func(err error) { fmt.Fprintln(os.Stderr, err) }})
	if err != nil {
		panic(err)
	}
	return wrapped
}
func main() {
	context := flag.String("snapshot", "", "print a files or confirm JSON snapshot instead of running the TUI")
	listen := flag.String("listen", "", "serve the browser demo, e.g. 127.0.0.1:8081")
	ghosttyAssets := flag.String("ghostty-assets", "", "serve a local ntwebterm dist with OSC 7501 support")
	flag.Parse()
	if *listen != "" {
		if err := serveBrowser(*listen, *ghosttyAssets); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *context != "" {
		if *context != "files" && *context != "confirm" {
			fmt.Fprintln(os.Stderr, "snapshot must be files or confirm")
			os.Exit(2)
		}
		s, _, err := bubblehelp.Snapshot(keys(*context == "confirm", false), "demo", *context, 1, bubblehelp.Options{})
		if err == nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			err = enc.Encode(s)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := booba.Run(newActionModel()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
