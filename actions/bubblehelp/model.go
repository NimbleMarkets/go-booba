package bubblehelp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"

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
}

func NewModel(inner tea.Model, describe Describe, opts ModelOptions) (*Model, error) {
	if inner == nil || describe == nil {
		return nil, fmt.Errorf("model and description callback are required")
	}
	if opts.Instance == "" {
		opts.Instance = rand.Text()
	}
	m := &Model{Inner: inner, describe: describe, options: opts, alternate: inner.View().AltScreen}
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
	return m, nil
}
func (m *Model) Init() tea.Cmd  { return tea.Batch(m.Inner.Init(), tea.Raw(actions.QueryProgramStatus)) }
func (m *Model) View() tea.View { return m.Inner.View() }

// IsProgramStatusSupport recognizes both the current fork's untyped OSC event
// and Bubble Tea v2.1's typed reply. The reflection compatibility check can go
// away once booba's WASM fork is rebased and the typed API can be imported.
func IsProgramStatusSupport(msg tea.Msg) bool {
	if osc, ok := msg.(uv.UnknownOscEvent); ok {
		return string(osc) == actions.QueryProgramStatus || string(osc) == "\x1b]7501;?\a"
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
	if description.Keys == nil {
		return m.fail(fmt.Errorf("action help map is nil"))
	}
	snapshot, _, err := Snapshot(description.Keys, m.options.Instance, description.Context, 0, m.options.Adapter)
	if err != nil {
		return m.fail(err)
	}
	m.status = description.Status
	if m.status.Summary == "" {
		m.status.Summary = description.Context
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
	m.lastStatus = m.status
	if m.sequence == "" {
		return nil
	}
	return tea.Raw(m.sequence)
}
func (m *Model) fail(err error) tea.Cmd {
	m.retry = true
	if m.options.OnError != nil {
		m.options.OnError(err)
	}
	// Invalid/oversized updates must not leave previous controls actionable.
	clear, _ := actions.ClearProgramStatus(m.options.Instance)
	return tea.Raw(clear)
}
