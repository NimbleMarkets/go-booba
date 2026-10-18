// Package termhost hosts Booba's Terminal Actions inside the program when the
// terminal cannot. Booba's browser terminal answers the OSC 7501 capability
// query and turns each published help snapshot into buttons. A plain terminal
// never answers, so nothing is published and the bindings stay keyboard-only.
//
// Model wraps the application, usually already wrapped by bubblehelp. It waits
// for the terminal's reply; if none arrives within Options.Timeout, it answers
// the query itself, filters the OSC 7501 reports out of the program's output
// stream, and draws them as a row of buttons under the application's view.
// Clicking a button sends that action's key through the ordinary message loop,
// which is exactly what the browser bar does. When the terminal does answer,
// the wrapper steps aside and every byte reaches the terminal unchanged.
//
//	wrapped, _ := bubblehelp.NewModel(app, describe, bubblehelp.ModelOptions{})
//	host, out := termhost.New(wrapped, os.Stdout, termhost.Options{})
//	p := tea.NewProgram(host, tea.WithOutput(out))
package termhost

import (
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
)

// Mode is the host's relationship to the terminal.
type Mode int8

const (
	Probing     Mode = iota // waiting for the terminal's capability reply
	Passthrough             // the terminal supports OSC 7501; nothing is intercepted
	Hosting                 // the program answers its own queries and draws the bar
)

// DefaultTimeout bounds the wait for the terminal's capability reply.
const DefaultTimeout = 250 * time.Millisecond

// Options configures a Model.
type Options struct {
	Timeout time.Duration // zero means DefaultTimeout
	Styles  *Styles       // nil means DefaultStyles
	// OnError receives malformed reports. The stale controls are removed
	// either way, matching the browser's behavior.
	OnError func(error)
}

// Model wraps an application model and owns the host's probe, snapshot, and
// bar. Use it as the program's model with the writer New returns as output.
type Model struct {
	Inner    tea.Model
	writer   *Writer
	opts     Options
	mode     Mode
	snapshot *actions.Snapshot
	id       string
	summary  string
	expanded bool
	width    int
	bar      barLayout
}

type probeTimeoutMsg struct{}
type drainMsg struct{}

// New wraps inner and output. The returned writer must be the program's
// output, or reports reach the terminal instead of the host.
func New(inner tea.Model, output io.Writer, opts Options) (*Model, io.Writer) {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Styles == nil {
		defaults := DefaultStyles()
		opts.Styles = &defaults
	}
	w, out := newWriter(output)
	return &Model{Inner: inner, writer: w, opts: opts}, out
}

// Mode reports whether the terminal or the program is hosting the actions.
func (m *Model) Mode() Mode { return m.mode }

// Snapshot is the hosted action set, or nil when nothing is shown.
func (m *Model) Snapshot() *actions.Snapshot { return m.snapshot }

// Summary is the readable status that accompanied the hosted snapshot.
func (m *Model) Summary() string { return m.summary }

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.Inner.Init(), m.listen(),
		tea.Tick(m.opts.Timeout, func(time.Time) tea.Msg { return probeTimeoutMsg{} }))
}

func (m *Model) listen() tea.Cmd {
	w := m.writer
	return func() tea.Msg {
		<-w.wake
		return drainMsg{}
	}
}

func (m *Model) setMode(mode Mode) {
	m.mode = mode
	m.writer.setMode(mode)
}

func (m *Model) forward(msg tea.Msg) tea.Cmd {
	next, cmd := m.Inner.Update(msg)
	m.Inner = next
	return cmd
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.ProgramStatusSupportMsg:
		if m.mode == Probing {
			m.setMode(Passthrough)
		}
	case probeTimeoutMsg:
		if m.mode != Probing {
			return m, nil
		}
		m.setMode(Hosting)
		return m, m.forward(tea.ProgramStatusSupportMsg{})
	case drainMsg:
		var cmds []tea.Cmd
		for _, e := range m.writer.drain() {
			cmds = append(cmds, m.handle(e))
		}
		return m, tea.Batch(append(cmds, m.listen())...)
	case tea.WindowSizeMsg:
		m.width = v.Width
	case tea.MouseClickMsg:
		if m.mode == Hosting && m.snapshot != nil && v.Button == tea.MouseLeft {
			if s, ok := m.bar.hit(v.X, v.Y); ok {
				return m, m.activate(s)
			}
		}
	}
	return m, m.forward(msg)
}

// handle applies one intercepted sequence with the browser's lifetime rules:
// a new query resets the controls, a clear of the record or an ancestor
// removes them, and a malformed or action-less replacement removes them too.
func (m *Model) handle(e event) tea.Cmd {
	switch {
	case e.query:
		m.clear()
		return m.forward(tea.ProgramStatusSupportMsg{})
	case e.err != nil:
		m.clear()
		if m.opts.OnError != nil {
			m.opts.OnError(e.err)
		}
	case e.report.Clear:
		if e.report.ID == "" || e.report.ID == m.id || strings.HasPrefix(m.id, e.report.ID+"/") {
			m.clear()
		}
	case e.report.Snapshot != nil:
		m.snapshot, m.id, m.summary = e.report.Snapshot, e.report.ID, e.report.Status.Summary
	case e.report.ID == m.id:
		m.clear()
	}
	return nil
}

func (m *Model) clear() {
	m.snapshot, m.id, m.summary = nil, "", ""
	m.expanded = false
}

func (m *Model) activate(s span) tea.Cmd {
	if s.toggle {
		m.expanded = !m.expanded
		return nil
	}
	a := m.snapshot.Actions[s.action]
	if !a.Enabled || len(a.Keys) == 0 {
		return nil
	}
	gesture := a.Keys[0]
	if a.Preferred != nil && *a.Preferred >= 0 && *a.Preferred < len(a.Keys) {
		gesture = a.Keys[*a.Preferred]
	}
	key, ok := KeyPress(gesture)
	if !ok {
		return nil
	}
	// Through the program loop, so every wrapper sees it as typed input.
	return func() tea.Msg { return key }
}

// View appends the bar under the application's content while hosting and
// enables mouse reporting so buttons can be clicked. The bar needs free rows
// below the content; the host does not scroll or shrink the application.
func (m *Model) View() tea.View {
	v := m.Inner.View()
	m.bar = barLayout{}
	if m.mode != Hosting || m.snapshot == nil {
		return v
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	bar, layout := renderBar(m.snapshot, m.expanded, width, *m.opts.Styles)
	if v.Content != "" {
		layout.row = strings.Count(v.Content, "\n") + 1
		v.Content += "\n" + bar
	} else {
		v.Content = bar
	}
	m.bar = layout
	if v.MouseMode == tea.MouseModeNone {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}
