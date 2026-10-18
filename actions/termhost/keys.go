package termhost

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
)

var namedKeys = map[string]rune{
	"space": tea.KeySpace, "enter": tea.KeyEnter, "escape": tea.KeyEscape, "tab": tea.KeyTab,
	"backspace": tea.KeyBackspace, "up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft,
	"right": tea.KeyRight, "home": tea.KeyHome, "end": tea.KeyEnd, "insert": tea.KeyInsert,
	"delete": tea.KeyDelete, "pageup": tea.KeyPgUp, "pagedown": tea.KeyPgDown,
	"f1": tea.KeyF1, "f2": tea.KeyF2, "f3": tea.KeyF3, "f4": tea.KeyF4, "f5": tea.KeyF5, "f6": tea.KeyF6,
	"f7": tea.KeyF7, "f8": tea.KeyF8, "f9": tea.KeyF9, "f10": tea.KeyF10, "f11": tea.KeyF11, "f12": tea.KeyF12,
}

// KeyPress is the in-process key encoder: it turns an action gesture into the
// key press the application would receive from the terminal. It reports false
// for gestures it cannot represent, and the host then declines activation.
func KeyPress(g actions.Gesture) (tea.KeyPressMsg, bool) {
	var k tea.Key
	for _, mod := range g.Modifiers {
		switch mod {
		case "shift":
			k.Mod |= tea.ModShift
		case "alt":
			k.Mod |= tea.ModAlt
		case "ctrl":
			k.Mod |= tea.ModCtrl
		default:
			return tea.KeyPressMsg{}, false
		}
	}
	if code, ok := namedKeys[g.Key]; ok {
		k.Code = code
		if code == tea.KeySpace && k.Mod == 0 {
			k.Text = " "
		}
		return tea.KeyPressMsg(k), true
	}
	r, size := utf8.DecodeRuneInString(g.Key)
	if r == utf8.RuneError || size != len(g.Key) {
		return tea.KeyPressMsg{}, false
	}
	k.Code = r
	switch {
	case k.Mod == tea.ModShift && r >= 'a' && r <= 'z':
		// Shift changes ASCII letter case, as Booba's legacy encoder does.
		k.Text = string(r - 'a' + 'A')
	case k.Mod&^tea.ModShift == 0:
		k.Text = string(r)
	}
	return tea.KeyPressMsg(k), true
}
