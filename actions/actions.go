// Package actions describes terminal interactions independently of their wire transport.
package actions

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	Version     = 1
	MaxBytes    = 65536 // Logical JSON limit; an eventual OSC envelope may impose a lower limit.
	MaxActions  = 128
	MaxKeys     = 16
	MaxGroups   = 32
	MaxRevision = 9007199254740991 // JavaScript's largest exact integer.
)

// Gesture is one key press with modifiers, never a macro or arbitrary input bytes.
type Gesture struct {
	Key       string   `json:"key"`
	Modifiers []string `json:"modifiers"`
}

type Action struct {
	ID      string    `json:"id"`
	Label   string    `json:"label"`
	Hint    string    `json:"hint,omitempty"`
	Enabled bool      `json:"enabled"`
	Keys    []Gesture `json:"keys"`
	// Preferred selects a single gesture only when the producer declares aliases.
	Preferred *int `json:"preferred,omitempty"`
}

type Snapshot struct {
	Version  int        `json:"version"`
	Instance string     `json:"instance"`
	Context  string     `json:"context"`
	Revision uint64     `json:"revision"`
	Actions  []Action   `json:"actions"`
	Short    []string   `json:"short"`
	Full     [][]string `json:"full"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9_.+-]{1,64}$`)
var namedKey = regexp.MustCompile(`^(space|enter|escape|tab|backspace|up|down|left|right|home|end|insert|delete|pageup|pagedown|f([1-9]|1[0-2]))$`)

func validText(s string, max int) bool {
	if !utf8.ValidString(s) || len(s) > max {
		return false
	}
	for _, r := range s {
		if r < 32 || (r >= 127 && r <= 159) {
			return false
		}
	}
	return true
}

func (g Gesture) Validate() error {
	if !namedKey.MatchString(g.Key) && (utf8.RuneCountInString(g.Key) != 1 || g.Key == " " || !validText(g.Key, 4)) {
		return fmt.Errorf("unsupported key %q", g.Key)
	}
	if g.Modifiers == nil || len(g.Modifiers) > 3 {
		return fmt.Errorf("modifiers must be an array of at most three values")
	}
	seen := map[string]bool{}
	for _, m := range g.Modifiers {
		if (m != "shift" && m != "alt" && m != "ctrl") || seen[m] {
			return fmt.Errorf("invalid modifier %q", m)
		}
		seen[m] = true
	}
	return nil
}

// Validate checks the logical schema. Unknown JSON fields are ignored for forward compatibility.
func (s Snapshot) Validate() error {
	if s.Version != Version || !identifier.MatchString(s.Instance) || !identifier.MatchString(s.Context) || s.Revision > MaxRevision {
		return fmt.Errorf("invalid snapshot header")
	}
	if s.Actions == nil || s.Short == nil || s.Full == nil || len(s.Actions) > MaxActions || len(s.Full) > MaxGroups {
		return fmt.Errorf("invalid snapshot arrays")
	}
	ids := map[string]bool{}
	for _, a := range s.Actions {
		if !identifier.MatchString(a.ID) || ids[a.ID] || a.Label == "" || !validText(a.Label, 256) || !validText(a.Hint, 128) || len(a.Keys) == 0 || len(a.Keys) > MaxKeys {
			return fmt.Errorf("invalid action %q", a.ID)
		}
		ids[a.ID] = true
		for _, g := range a.Keys {
			if err := g.Validate(); err != nil {
				return fmt.Errorf("action %s: %w", a.ID, err)
			}
		}
		if a.Preferred != nil && (*a.Preferred < 0 || *a.Preferred >= len(a.Keys)) {
			return fmt.Errorf("invalid preferred gesture")
		}
	}
	check := func(refs []string, seen map[string]bool) error {
		if refs == nil || len(refs) > MaxActions {
			return fmt.Errorf("invalid references")
		}
		for _, id := range refs {
			if !ids[id] || seen[id] {
				return fmt.Errorf("invalid or repeated reference %q", id)
			}
			seen[id] = true
		}
		return nil
	}
	if err := check(s.Short, map[string]bool{}); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, group := range s.Full {
		if err := check(group, seen); err != nil {
			return err
		}
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if len(data) > MaxBytes {
		return fmt.Errorf("snapshot exceeds %d bytes", MaxBytes)
	}
	return nil
}

// Decode accepts one bounded JSON snapshot and validates it before returning it.
func Decode(data []byte) (Snapshot, error) {
	var s Snapshot
	if len(data) > MaxBytes || !utf8.Valid(data) || !validSurrogates(data) {
		return s, fmt.Errorf("invalid snapshot encoding or size")
	}
	// Check required fields before typed decoding (otherwise missing booleans become false).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return s, err
	}
	for _, k := range []string{"version", "instance", "context", "revision", "actions", "short", "full"} {
		if len(raw[k]) == 0 || string(raw[k]) == "null" {
			return s, fmt.Errorf("missing %s", k)
		}
	}
	var aa []map[string]json.RawMessage
	if err := json.Unmarshal(raw["actions"], &aa); err != nil {
		return s, err
	}
	for _, a := range aa {
		if v, ok := a["hint"]; ok && string(v) == "null" {
			return s, fmt.Errorf("null hint")
		}
		for _, k := range []string{"id", "label", "enabled", "keys"} {
			if len(a[k]) == 0 || string(a[k]) == "null" {
				return s, fmt.Errorf("missing action %s", k)
			}
		}
		if v, ok := a["preferred"]; ok && string(v) == "null" {
			return s, fmt.Errorf("null preferred gesture")
		}
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, s.Validate()
}

// ParseKey converts a Bubble Tea-style key string into a gesture. Unsupported
// multi-key notation is an error; it is never treated as text to type.
func ParseKey(s string) (Gesture, error) {
	g := Gesture{Modifiers: []string{}}
	for {
		found := false
		for _, m := range []string{"shift", "alt", "ctrl"} {
			if strings.HasPrefix(s, m+"+") {
				g.Modifiers = append(g.Modifiers, m)
				s = strings.TrimPrefix(s, m+"+")
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	switch s {
	case "esc":
		s = "escape"
	case " ":
		s = "space"
	case "pgup":
		s = "pageup"
	case "pgdown":
		s = "pagedown"
	}
	g.Key = s
	return g, g.Validate()
}

// encoding/json replaces unpaired UTF-16 escapes with U+FFFD. Reject them so
// Go and JavaScript agree on the Unicode scalar values permitted by the schema.
func validSurrogates(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) || data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return false
		}
		n, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n >= 0xDC00 && n <= 0xDFFF {
			return false
		}
		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return true
}
