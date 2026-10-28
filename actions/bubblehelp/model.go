package bubblehelp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/actions"
	uv "github.com/charmbracelet/ultraviolet"
)

// Description connects a model's current help map to human-readable status.
type Description struct {
	Context string
	Keys    help.KeyMap
	Status  actions.ProgramStatus
}
type Describe func(tea.Model) Description

type ModelOptions struct {
	Instance string // Empty generates a fresh identifier per invocation.
	Adapter  Options
	OnError  func(error)
}

// Model wraps a Bubble Tea model and publishes its current help through the
// private OSC 7501 msg convention. Replies advertise status support only; other
// OSC 7501 hosts may display the readable summary followed by the JSON payload.
type Model struct {
	Inner      tea.Model
	describe   Describe
	options    ModelOptions
	publisher  *actions.Publisher
	supported  bool
	alternate  bool
	status     actions.ProgramStatus
	lastStatus actions.ProgramStatus
	sequence   string
	retry      bool
	fallback   bool
	lastError  string
}

// NewModel strictly validates the initial action description before startup.
// Use NewModelWithFallback when action metadata is optional.
func NewModel(inner tea.Model, describe Describe, opts ModelOptions) (*Model, error) {
	return newModel(inner, describe, opts, false)
}

// NewModelWithFallback keeps the application usable when its action description
// is invalid or oversized. It publishes no actions for a failing description,
// clears stale controls, and retries on subsequent updates. Bindings and help in
// Inner are unchanged. OnError reports each distinct consecutive failure; when
// unset, diagnostics go to the standard logger. Configuration errors (nil model,
// nil callback, invalid instance) still return an error.
func NewModelWithFallback(inner tea.Model, describe Describe, opts ModelOptions) (*Model, error) {
	if opts.OnError == nil {
		opts.OnError = func(err error) { log.Print(err) }
	}
	return newModel(inner, describe, opts, true)
}

func newModel(inner tea.Model, describe Describe, opts ModelOptions, fallback bool) (*Model, error) {
	if inner == nil || describe == nil {
		return nil, fmt.Errorf("model and description callback are required")
	}
	if opts.Instance == "" {
		opts.Instance = rand.Text()
	}
	m := &Model{Inner: inner, describe: describe, options: opts, alternate: inner.View().AltScreen, fallback: fallback}
	publisher, err := actions.NewPublisher(opts.Instance, func(data []byte) error {
		var s actions.Snapshot
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		wire, err := actions.EncodeProgramStatus(s, m.status)
		if err != nil {
			return err
		}
		m.sequence = wire
		return nil
	})
	if err != nil {
		return nil, err
	}
	m.publisher = publisher
	// Validate the initial description before any terminal output is emitted.
	description := m.describe(m.Inner)
	snapshot, err := m.snapshot(description, 1)
	if err == nil {
		_, err = actions.EncodeProgramStatus(snapshot, m.status)
	}
	if err != nil {
		if !fallback {
			return nil, err
		}
		m.reportError(err)
	}
	return m, nil
}
func (m *Model) Init() tea.Cmd  { return tea.Batch(m.Inner.Init(), tea.Raw(actions.QueryProgramStatus)) }
func (m *Model) View() tea.View { return m.Inner.View() }

// IsProgramStatusSupport recognizes both the current fork's untyped OSC event
// and Bubble Tea v2.1's typed reply. The reflection compatibility check can go
// away once booba's WASM fork is rebased and the typed API can be imported.
func IsProgramStatusSupport(msg tea.Msg) bool {
	if osc, ok := msg.(uv.UnknownOscEvent); ok {
		payload, ok := strings.CutPrefix(string(osc), "\x1b]7501;")
		if !ok {
			return false
		}
		if body, ok := strings.CutSuffix(payload, "\x1b\\"); ok {
			payload = body
		} else if body, ok := strings.CutSuffix(payload, "\a"); ok {
			payload = body
		} else {
			return false
		}
		// Revision 0.4 appends capability lists. Keep accepting the legacy
		// bare reply and allow future capability fields without requiring
		// a particular list or ordering.
		return payload == "?" || strings.HasPrefix(payload, "?:")
	}
	t := reflect.TypeOf(msg)
	return t != nil && t.PkgPath() == "charm.land/bubbletea/v2" && t.Name() == "ProgramStatusSupportMsg"
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if IsProgramStatusSupport(msg) {
		m.supported = true
		return m, m.publish(true)
	}
	next, cmd := m.Inner.Update(msg)
	m.Inner = next
	alternate := next.View().AltScreen
	if alternate != m.alternate {
		// A capability round trip puts the next snapshot after the screen switch
		// in the renderer output, rather than restoring controls before it clears.
		m.alternate = alternate
		m.supported = false
		return m, tea.Batch(cmd, tea.Raw(actions.QueryProgramStatus))
	}
	if !m.supported {
		return m, cmd
	}
	return m, tea.Batch(cmd, m.publish(false))
}
func (m *Model) publish(force bool) tea.Cmd {
	description := m.describe(m.Inner)
	snapshot, err := m.snapshot(description, 0)
	if err != nil {
		return m.fail(err)
	}
	m.sequence = ""
	changed, err := m.publisher.Publish(snapshot)
	if err == nil && !changed && (force || m.retry || m.lastStatus != m.status) {
		err = m.publisher.Refresh()
	}
	if err != nil {
		return m.fail(err)
	}
	m.retry = false
	m.lastError = ""
	m.lastStatus = m.status
	if m.sequence == "" {
		return nil
	}
	return tea.Raw(m.sequence)
}
func (m *Model) snapshot(description Description, revision uint64) (actions.Snapshot, error) {
	if description.Keys == nil {
		return actions.Snapshot{}, fmt.Errorf("action help map is nil")
	}
	m.status = description.Status
	if m.status.Summary == "" {
		m.status.Summary = description.Context
	}
	opts := m.options.Adapter
	opts.Summary = m.status.Summary
	snapshot, _, err := Snapshot(description.Keys, m.options.Instance, description.Context, revision, opts)
	return snapshot, err
}

func (m *Model) fail(err error) tea.Cmd {
	m.retry = true
	m.reportError(err)
	// Invalid/oversized updates must not leave previous controls actionable.
	clear, _ := actions.ClearProgramStatus(m.options.Instance)
	return tea.Raw(clear)
}

func (m *Model) reportError(err error) {
	if m.fallback {
		if m.lastError == err.Error() {
			return
		}
		m.lastError = err.Error()
		err = fmt.Errorf("booba: action publication disabled for current description; keyboard controls remain available: %w", err)
	}
	if m.options.OnError != nil {
		m.options.OnError(err)
	}
}
