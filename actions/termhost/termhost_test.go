package termhost

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
)

// recorder is the wrapped application: it records messages and shows two lines.
type recorder struct {
	msgs []tea.Msg
}

func (r *recorder) Init() tea.Cmd { return nil }
func (r *recorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	r.msgs = append(r.msgs, msg)
	return r, nil
}
func (r *recorder) View() tea.View { return tea.NewView("line one\nline two") }

func (r *recorder) supported() int {
	n := 0
	for _, m := range r.msgs {
		if _, ok := m.(tea.ProgramStatusSupportMsg); ok {
			n++
		}
	}
	return n
}

func run(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	return cmd()
}

func host(t *testing.T) (*Model, *recorder, *bytes.Buffer) {
	t.Helper()
	app := &recorder{}
	var out bytes.Buffer
	m, w := New(app, &out, Options{})
	if w != any(m.writer) {
		t.Fatal("buffer output should be the plain writer")
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	return m, app, &out
}

func TestTerminalReplyMeansPassthrough(t *testing.T) {
	m, app, _ := host(t)
	m.Update(tea.ProgramStatusSupportMsg{})
	if m.Mode() != Passthrough || app.supported() != 1 {
		t.Fatalf("mode %v supported %d", m.Mode(), app.supported())
	}
	m.Update(probeTimeoutMsg{})
	if m.Mode() != Passthrough || app.supported() != 1 {
		t.Fatal("timeout after reply must change nothing")
	}
	if v := m.View(); v.Content != "line one\nline two" || v.MouseMode != tea.MouseModeNone {
		t.Fatalf("passthrough view %+v", v)
	}
}

func TestTimeoutHostsAndRendersBar(t *testing.T) {
	m, app, out := host(t)
	m.Update(probeTimeoutMsg{})
	if m.Mode() != Hosting || app.supported() != 1 {
		t.Fatalf("mode %v supported %d", m.Mode(), app.supported())
	}
	// The application now publishes through the output stream.
	wire, _ := actions.EncodeProgramStatus(testSnapshot(), actions.ProgramStatus{Summary: "Hello"})
	write(t, m.writer, "frame"+wire)
	if out.String() != "frame" {
		t.Fatalf("report leaked: %q", out.String())
	}
	if msg := run(t, m.listen()); msg != (drainMsg{}) {
		t.Fatalf("listen returned %T", msg)
	}
	m.Update(drainMsg{})
	if m.Snapshot() == nil || m.Summary() != "Hello" {
		t.Fatalf("snapshot %+v summary %q", m.Snapshot(), m.Summary())
	}
	v := m.View()
	lines := strings.Split(v.Content, "\n")
	if len(lines) != 3 || !strings.Contains(lines[2], "Pop") || !strings.Contains(lines[2], "Color") || !strings.Contains(lines[2], "More") || strings.Contains(lines[2], "Quit") {
		t.Fatalf("bar %q", v.Content)
	}
	if v.MouseMode != tea.MouseModeCellMotion || m.bar.row != 2 || len(m.bar.spans) != 3 {
		t.Fatalf("mouse %v layout %+v", v.MouseMode, m.bar)
	}
	// Click Pop: the key goes through the program loop as a command.
	_, cmd := m.Update(tea.MouseClickMsg{X: m.bar.spans[0].x0, Y: 2, Button: tea.MouseLeft})
	key, ok := run(t, cmd).(tea.KeyPressMsg)
	if !ok || key.String() != "enter" {
		t.Fatalf("click produced %T %v", key, key)
	}
	// Disabled controls and misses do nothing but reach the application.
	before := len(app.msgs)
	if _, cmd := m.Update(tea.MouseClickMsg{X: m.bar.spans[1].x0, Y: 2, Button: tea.MouseLeft}); cmd != nil {
		t.Fatal("disabled action activated")
	}
	if _, cmd := m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft}); cmd != nil || len(app.msgs) != before+1 {
		t.Fatal("miss not forwarded")
	}
	// More expands to the full groups, including Quit.
	m.Update(tea.MouseClickMsg{X: m.bar.spans[2].x0, Y: 2, Button: tea.MouseLeft})
	if v := m.View(); !strings.Contains(v.Content, "Quit") || !strings.Contains(v.Content, "Less") {
		t.Fatalf("expanded %q", v.Content)
	}
}

func TestLifetimes(t *testing.T) {
	m, app, _ := host(t)
	m.Update(probeTimeoutMsg{})
	wire, _ := actions.EncodeProgramStatus(testSnapshot(), actions.ProgramStatus{Summary: "Hello"})
	show := func() {
		write(t, m.writer, wire)
		m.Update(drainMsg{})
		if m.Snapshot() == nil {
			t.Fatal("snapshot not applied")
		}
	}
	// A clear of the record removes the controls; an unrelated clear does not.
	show()
	write(t, m.writer, "\x1b]7501;state=clear:id=other\a")
	m.Update(drainMsg{})
	if m.Snapshot() == nil {
		t.Fatal("unrelated clear removed controls")
	}
	write(t, m.writer, "\x1b]7501;state=clear:id=booba-actions\a")
	m.Update(drainMsg{})
	if m.Snapshot() != nil {
		t.Fatal("ancestor clear kept controls")
	}
	// A re-query resets the controls and is answered again.
	show()
	write(t, m.writer, actions.QueryProgramStatus)
	m.Update(drainMsg{})
	if m.Snapshot() != nil || app.supported() != 2 {
		t.Fatalf("query: snapshot %v supported %d", m.Snapshot(), app.supported())
	}
	// A malformed replacement removes the controls and reports the error.
	var reported error
	m.opts.OnError = func(err error) { reported = err }
	show()
	write(t, m.writer, "\x1b]7501;state=idle:id=booba-actions/demo:msg=!!!\a")
	m.Update(drainMsg{})
	if m.Snapshot() != nil || reported == nil {
		t.Fatal("malformed replacement kept controls")
	}
	// An action-less replacement of the same record removes the controls.
	show()
	write(t, m.writer, "\x1b]7501;state=idle:id=booba-actions/demo:msg=SGVsbG8=\a")
	m.Update(drainMsg{})
	if m.Snapshot() != nil {
		t.Fatal("plain replacement kept controls")
	}
	if v := m.View(); v.Content != "line one\nline two" || v.MouseMode != tea.MouseModeNone {
		t.Fatalf("cleared view %+v", v)
	}
}
