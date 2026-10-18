package termhost

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/go-booba/actions"
)

// Styles colors the bar. Zero values take DefaultStyles.
type Styles struct {
	Button   lipgloss.Style // enabled action
	Disabled lipgloss.Style // disabled action, still shown
	Hint     lipgloss.Style // key hint inside a button
	Toggle   lipgloss.Style // More / Less
	Gap      string         // between buttons
}

// DefaultStyles renders buttons in reverse video, disabled ones faint, and
// hints dim. It needs no particular color depth.
func DefaultStyles() Styles {
	return Styles{
		Button:   lipgloss.NewStyle().Reverse(true),
		Disabled: lipgloss.NewStyle().Faint(true),
		Hint:     lipgloss.NewStyle().Faint(true),
		Toggle:   lipgloss.NewStyle().Bold(true),
		Gap:      " ",
	}
}

// span is one clickable cell range on a bar row. action indexes
// Snapshot.Actions; toggle marks the More / Less control.
type span struct {
	row, x0, x1 int
	action      int
	toggle      bool
}

type barLayout struct {
	row   int // screen row of the first bar line
	rows  int
	spans []span
}

func (l barLayout) hit(x, y int) (span, bool) {
	for _, s := range l.spans {
		if y == l.row+s.row && x >= s.x0 && x < s.x1 {
			return s, true
		}
	}
	return span{}, false
}

// items lists the actions to show. Compact shows the short help; expanded
// shows the full groups and keeps short-only controls, as the browser bar does.
func items(s *actions.Snapshot, expanded bool) (shown []int, more bool) {
	index := map[string]int{}
	for i, a := range s.Actions {
		index[a.ID] = i
	}
	seen := map[int]bool{}
	add := func(ids []string) {
		for _, id := range ids {
			if i, ok := index[id]; ok && !seen[i] {
				seen[i] = true
				shown = append(shown, i)
			}
		}
	}
	add(s.Short)
	short := len(shown)
	for _, group := range s.Full {
		add(group)
	}
	more = len(shown) > short
	if !expanded {
		shown = shown[:short]
	}
	return shown, more
}

func renderBar(s *actions.Snapshot, expanded bool, width int, st Styles) (string, barLayout) {
	shown, more := items(s, expanded)
	type button struct {
		text   string
		action int
		toggle bool
	}
	var buttons []button
	for _, i := range shown {
		a := s.Actions[i]
		label := " " + a.Label
		if a.Hint != "" {
			label += " " + st.Hint.Inline(true).Render(a.Hint)
		}
		label += " "
		style := st.Button
		if !a.Enabled {
			style = st.Disabled
		}
		buttons = append(buttons, button{style.Inline(true).Render(label), i, false})
	}
	if more {
		text := " More "
		if expanded {
			text = " Less "
		}
		buttons = append(buttons, button{st.Toggle.Inline(true).Render(text), -1, true})
	}
	var lines []string
	var layout barLayout
	var line strings.Builder
	x, row := 0, 0
	for _, b := range buttons {
		w := lipgloss.Width(b.text)
		if x > 0 && x+len(st.Gap)+w > width {
			lines = append(lines, line.String())
			line.Reset()
			x, row = 0, row+1
		}
		if x > 0 {
			line.WriteString(st.Gap)
			x += lipgloss.Width(st.Gap)
		}
		layout.spans = append(layout.spans, span{row: row, x0: x, x1: x + w, action: b.action, toggle: b.toggle})
		line.WriteString(b.text)
		x += w
	}
	lines = append(lines, line.String())
	layout.rows = len(lines)
	return strings.Join(lines, "\n"), layout
}
