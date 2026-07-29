// tests/rtl-text/rtl_text.go
//
// Isolated reproduction for RTL text rendering in ghostty-web: BiDi ordering
// and Arabic cursive joining, across Hebrew, Arabic, and Persian.
//
//	https://github.com/coder/ghostty-web/issues/83
//	https://github.com/ghostty-org/ghostty/issues/1442
//
// Every sample is written to the PTY in logical (Unicode) order. The screen is
// static — no timers, no animation — so anything wrong on it comes from the
// renderer, not from this program. Run it natively for a baseline, then with
// --listen to compare the same byte stream as painted by ghostty-web.

package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/pflag"
)

// Note that an editor applying the BiDi algorithm displays the literals below
// right-to-left, so the source reads mirrored from the byte order — rows 7 and
// 15 of the view spell out what actually goes on the wire.
//
// Hebrew — BiDi class R (strong right-to-left), non-cursive.
const (
	shalom = "שלום" // shin lamed vav mem-final
	olam   = "עולם" // ayin vav lamed mem-final

	phrase = shalom + " " + olam
	mixed  = "hello " + shalom + " world"
	digits = shalom + " 123 " + olam
	punct  = "!" + shalom + ", " + olam + "?"
)

// Arabic and Persian — BiDi class AL (Arabic Letter), which resolves
// differently from R in several UBA rules that Hebrew alone never reaches.
// Both scripts are also cursive: letters take initial/medial/final/isolated
// forms depending on their neighbours, which is a shaping problem distinct
// from ordering. A renderer that rasterizes one grapheme per cell produces
// isolated forms regardless of how correct its BiDi is.
const (
	arMarhaba = "مرحبا"   // Arabic "hello"
	arAlam    = "بالعالم" // Arabic "world"
	arWord    = "عربي"    // "Arabic" — 4 letters, all of which should join
	faSalam   = "سلام"    // Persian "hello"
	faDonya   = "دنیا"    // Persian "world"

	// Digits differ by BiDi class, not just by glyph: Arabic-Indic
	// U+0660..U+0669 are class AN, while Persian (Extended Arabic-Indic)
	// U+06F0..U+06F9 are class EN, same as ASCII. AN and EN resolve
	// differently, so both are worth exercising.
	//
	// Both use the value 456 deliberately. The two encodings are visually
	// identical at 1/2/3 but clearly distinct at 4/5/6, so 456 lets a reader
	// confirm the intended codepoints actually reached the renderer rather
	// than one range being substituted for the other by font fallback.
	arDigits = "٤٥٦" // U+0664 U+0665 U+0666 — class AN
	faDigits = "۴۵۶" // U+06F4 U+06F5 U+06F6 — class EN

	arPhrase = arMarhaba + " " + arAlam
	arMixed  = "hello " + arMarhaba + " world"
	arNum    = arMarhaba + " " + arDigits + " " + arAlam
	faPhrase = faSalam + " " + faDonya
	faNum    = faSalam + " " + faDigits + " " + faDonya
)

const labelWidth = 16

var (
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("211")).Bold(true)
	sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))
	subtleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(labelWidth)
	numStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))
	redStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	blueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	boxStyle     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("241")).
			Padding(0, 1)
	mainStyle = lipgloss.NewStyle().Margin(1, 2)
)

func main() {
	pflag.Parse()

	if startWebServerIfRequested() {
		return
	}

	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Println("could not start program:", err)
	}
}

type model struct {
	quitting bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("\n  See you later!\n\n")
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("ghostty-web RTL/BiDi repro — coder/ghostty-web#83"))
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render(
		"Every sample is written in logical (Unicode) order. A conforming renderer\n" +
			"applies the Unicode BiDi Algorithm, so each RTL run paints right-to-left,\n" +
			"and shapes Arabic cursively so its letters join."))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render("Hebrew — ordering (BiDi class R)"))
	b.WriteString("\n")
	b.WriteString(row(1, "pure RTL", phrase))
	b.WriteString(row(2, "mixed LTR/RTL", mixed))
	b.WriteString(row(3, "RTL + digits", digits))
	b.WriteString(row(4, "RTL + punct", punct))
	b.WriteString(row(5, "styled runs", redStyle.Render(shalom)+" "+blueStyle.Render(olam)))
	b.WriteString(rowBlock(6, "boxed", boxStyle.Render(phrase)))
	b.WriteString(rowBlock(7, "logical order", subtleStyle.Render(codepoints(phrase))))
	b.WriteString("\n")

	b.WriteString(sectionStyle.Render("Arabic / Persian — ordering (class AL) and cursive joining"))
	b.WriteString("\n")
	b.WriteString(row(8, "arabic", arPhrase))
	b.WriteString(row(9, "arabic mixed", arMixed))
	b.WriteString(row(10, "arabic styled", redStyle.Render(arMarhaba)+" "+blueStyle.Render(arAlam)))
	b.WriteString(row(11, "joining", arWord))
	b.WriteString(row(12, "digits (AN)", arNum))
	b.WriteString(row(13, "persian", faPhrase))
	b.WriteString(row(14, "digits (EN)", faNum))
	b.WriteString(rowBlock(15, "logical order", subtleStyle.Render(codepoints(arWord))))

	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("q, esc, ctrl+c: quit"))

	return tea.NewView(mainStyle.Render(b.String()))
}

// row renders a single-line sample beside its LTR label. The label column is
// fixed-width, so it doubles as an alignment reference for the sample.
func row(n int, label, sample string) string {
	return gutter(n, label) + sample + "\n"
}

// rowBlock is row for a sample spanning multiple lines, keeping the sample's
// own lines aligned to each other rather than wrapping under the label.
func rowBlock(n int, label, sample string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, gutter(n, label), sample) + "\n"
}

// gutter is the fixed-width "NN label" prefix. The row number is right-aligned
// so two-digit rows don't shift the label column, which is what the samples are
// visually aligned against.
func gutter(n int, label string) string {
	return numStyle.Render(fmt.Sprintf("%2d", n)) + " " + labelStyle.Render(label)
}

// codepoints spells out what this program actually wrote, which is the point of
// the exercise: it demonstrates the PTY received logical order, so any reversal
// on screen happened downstream in the renderer.
func codepoints(s string) string {
	var b strings.Builder
	for _, r := range s {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "U+%04X", r)
	}
	return b.String()
}
