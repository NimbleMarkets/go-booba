package termhost

import (
	"testing"

	"github.com/NimbleMarkets/go-booba/actions"
)

func TestKeyPress(t *testing.T) {
	cases := map[string]actions.Gesture{
		"enter":       {Key: "enter"},
		"esc":         {Key: "escape"},
		"right":       {Key: "right"},
		"space":       {Key: "space"},
		"q":           {Key: "q"},
		"ctrl+c":      {Key: "c", Modifiers: []string{"ctrl"}},
		"alt+x":       {Key: "x", Modifiers: []string{"alt"}},
		"f5":          {Key: "f5"},
		"pgdown":      {Key: "pagedown"},
		"A":           {Key: "a", Modifiers: []string{"shift"}},
		"shift+enter": {Key: "enter", Modifiers: []string{"shift"}},
	}
	for want, g := range cases {
		key, ok := KeyPress(g)
		if !ok {
			t.Errorf("%s: not encodable", want)
			continue
		}
		if got := key.String(); got != want {
			t.Errorf("%s: got %q", want, got)
		}
	}
	if k, ok := KeyPress(actions.Gesture{Key: "q"}); !ok || k.Text != "q" {
		t.Fatalf("printable text %+v", k)
	}
	if k, ok := KeyPress(actions.Gesture{Key: "c", Modifiers: []string{"ctrl"}}); !ok || k.Text != "" {
		t.Fatalf("ctrl text %+v", k)
	}
	for _, g := range []actions.Gesture{{Key: "qq"}, {Key: "q", Modifiers: []string{"meta"}}, {Key: ""}} {
		if _, ok := KeyPress(g); ok {
			t.Errorf("%+v encodable", g)
		}
	}
}
