package actions

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestStatusExactByteBoundary(t *testing.T) {
	for _, unicode := range []bool{false, true} {
		s := Snapshot{Version: 1, Instance: "app", Context: "map", Revision: 1, Actions: []Action{}, Short: []string{}, Full: [][]string{}}
		for i := 0; i < 6; i++ {
			s.Actions = append(s.Actions, Action{ID: string(rune('a' + i)), Label: strings.Repeat("x", 210), Enabled: true, Keys: []Gesture{{Key: "x", Modifiers: []string{}}}})
		}
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		remaining := MaxStatusMessageBytes - len(data) - len(MessageMarker)
		if remaining < 3 || remaining > 255 {
			t.Fatalf("fixture has %d summary bytes remaining", remaining)
		}
		summary := strings.Repeat("x", remaining)
		if unicode {
			summary = "界" + summary[3:]
		}
		for _, delta := range []int{-1, 0, 1} {
			text := summary
			if delta < 0 {
				text = text[:len(text)-1]
			} else if delta > 0 {
				text += "x"
			}
			wire, err := EncodeProgramStatus(s, ProgramStatus{Summary: text})
			if delta > 0 {
				if err == nil {
					t.Fatal("accepted 2049 bytes")
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			r, err := DecodeProgramStatus(wire)
			if err != nil || len(r.Message) != MaxStatusMessageBytes+delta || r.Status.Summary != text {
				t.Fatalf("boundary: %+v %v", r, err)
			}
		}
	}
	// Native receiver must reject oversize decoded messages, including multibyte text.
	for _, text := range []string{strings.Repeat("x", 2049), strings.Repeat("界", 683)} {
		wire := "\x1b]7501;state=idle:msg=" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
		if _, err := DecodeProgramStatus(wire); err == nil {
			t.Fatal("receiver accepted oversize message")
		}
	}
}
