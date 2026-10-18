// Package actiontest provides a realistic map viewer help map for regression tests.
package actiontest

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
	"github.com/NimbleMarkets/go-booba/actions/bubblehelp"
)

type Keys struct{ Primary, More []key.Binding }

func (k Keys) ShortHelp() []key.Binding  { return k.Primary }
func (k Keys) FullHelp() [][]key.Binding { return [][]key.Binding{k.Primary, k.More} }

// MapKeys models the individual controls in ntcharts-osm's mapview3d demo.
func MapKeys() Keys {
	var bindings []key.Binding
	for _, pair := range [][2]string{
		{"left", "Rotate left"}, {"right", "Rotate right"}, {"up", "Tilt up"}, {"down", "Tilt down"}, {"n", "Next place"},
		{"w", "Map north"}, {"s", "Map south"}, {"a", "Map west"}, {"d", "Map east"}, {"+", "Camera closer"}, {"-", "Camera farther"},
		{"[", "Map zoom out"}, {"]", "Map zoom in"}, {"p", "Previous place"}, {"0", "Reset camera"}, {"r", "Auto rotate"}, {"o", "Projection"}, {"g", "Graphics"},
	} {
		bindings = append(bindings, key.NewBinding(key.WithKeys(pair[0]), key.WithHelp(pair[0], pair[1])))
	}
	return Keys{bindings[:5], bindings[5:]}
}
func Describe(tea.Model) bubblehelp.Description {
	return bubblehelp.Description{Context: "mapview3d", Keys: MapKeys(), Status: actions.ProgramStatus{App: "ntcharts-osm", Summary: "Explore a textured 3D map"}}
}
