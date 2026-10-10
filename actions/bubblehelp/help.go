// Package bubblehelp adapts Bubbles v2 help maps to terminal action snapshots.
package bubblehelp

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"github.com/NimbleMarkets/go-booba/actions"
	"github.com/charmbracelet/x/ansi"
)

// Options allows application identity and presentation to supplement help data.
type Options struct {
	IncludeDisabled bool
	// Customize is called once per distinct binding. It may set ID, labels and
	// Preferred (only for aliases); errors abort the snapshot. It must be deterministic.
	Customize func(key.Binding, *actions.Action) error
}

type Diagnostic struct {
	Hint   string
	Key    string
	Reason string
}

// Snapshot preserves help ordering and groups. Unsupported keys are diagnosed
// and omitted. Distinct gestures remain separate unless Customize sets Preferred.
func Snapshot(km help.KeyMap, instance, context string, revision uint64, opts Options) (actions.Snapshot, []Diagnostic, error) {
	s := actions.Snapshot{Version: actions.Version, Instance: instance, Context: context, Revision: revision, Actions: []actions.Action{}, Short: []string{}, Full: [][]string{}}
	diagnostics := []Diagnostic{}
	identities := map[string]string{}
	used := map[string]string{}
	convert := func(b key.Binding) (string, error) {
		if !b.Enabled() && !opts.IncludeDisabled {
			return "", nil
		}
		canonical, _ := json.Marshal(struct {
			Keys        []string
			Hint, Label string
			Enabled     bool
		}{b.Keys(), b.Help().Key, b.Help().Desc, b.Enabled()})
		identity := string(canonical)
		if id, ok := identities[identity]; ok {
			return id, nil
		}
		hash := sha256.Sum256(canonical)
		a := actions.Action{ID: fmt.Sprintf("a-%x", hash[:16]), Label: ansi.Strip(b.Help().Desc), Hint: ansi.Strip(b.Help().Key), Enabled: b.Enabled(), Keys: []actions.Gesture{}}
		seen := map[string]bool{}
		for _, k := range b.Keys() {
			g, err := actions.ParseKey(k)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{a.Hint, k, err.Error()})
				continue
			}
			encoded, _ := json.Marshal(g)
			if !seen[string(encoded)] {
				a.Keys = append(a.Keys, g)
				seen[string(encoded)] = true
			}
		}
		if len(a.Keys) == 0 {
			identities[identity] = ""
			return "", nil
		}
		if a.Label == "" {
			a.Label = a.Hint
		}
		if a.Label == "" {
			a.Label = a.Keys[0].Key
		}
		if opts.Customize != nil {
			if err := opts.Customize(b, &a); err != nil {
				return "", err
			}
		}
		if previous, exists := used[a.ID]; exists && previous != identity {
			return "", fmt.Errorf("action ID collision %q; assign distinct explicit IDs", a.ID)
		}
		used[a.ID] = identity
		identities[identity] = a.ID
		s.Actions = append(s.Actions, a)
		return a.ID, nil
	}
	collect := func(bindings []key.Binding, seen map[string]bool) ([]string, error) {
		ids := []string{}
		for _, b := range bindings {
			id, err := convert(b)
			if err != nil {
				return nil, err
			}
			if id != "" && !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
		return ids, nil
	}
	var err error
	s.Short, err = collect(km.ShortHelp(), map[string]bool{})
	if err != nil {
		return s, diagnostics, err
	}
	seen := map[string]bool{}
	for _, group := range km.FullHelp() {
		ids, e := collect(group, seen)
		if e != nil {
			return s, diagnostics, e
		}
		if len(ids) > 0 {
			s.Full = append(s.Full, ids)
		}
	}
	return s, diagnostics, s.Validate()
}
