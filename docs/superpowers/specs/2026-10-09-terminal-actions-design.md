# Terminal Actions

**Date:** 2026-10-09  
**Status:** Experimental, opt-in implementation; APIs and wire convention subject to change  
**Scope:** Framework-neutral action descriptions, a Bubble Tea help adapter, and a booba touch interface

Terminal applications should describe the interactions currently available to the user. A terminal host can use that description to render a compact touch bar or an expanded action menu. Applications keep their existing keyboard handling; mobile users can perform common actions without opening the software keyboard.

The reference integration is Bubble Tea's help interface. Its short help supplies the primary controls, and its full help supplies ordered groups for expanded controls. The same application-owned bindings drive both terminal help and browser controls.

## Implementation status

The first implementation provides Go and TypeScript validation, a change-aware JSON publisher, a Bubbles v2.1 adapter, a headless action controller, a wrapping action bar, BoobaTerminal lifecycle/input integration, and a reference help-map example. See [Terminal Actions usage](../../TERMINAL_ACTIONS.md) for concrete APIs and limits. Key encoding is an explicit host dependency; no general-purpose encoder has been assigned. The demo now uses the private OSC 7501 msg convention described in the usage document.

Bubble Tea v2.1 supplies declarative `View.ProgramStatus` and capability-query messages, and Booba's WASM fork is rebased onto that release (`c397c32`). The embedded ghostty-web runtime (`579fc63`) exposes standard status fields through `onProgramStatus`; the optional booba bridge forwards that event when the loaded runtime supports it. The action bridge interprets the marked JSON suffix in the event’s decoded message.

The live native browser demo is in `cmd/booba-actions-example`. It uses Booba’s normal Sip WebSocket transport: OSC 7501 reports travel as terminal output, and button activations return as keyboard input. It includes a bounded demo key encoder and a toggle for terminal help. General-purpose key encoding remains open work. Full-help controls currently wrap rather than using a separate overflow menu.

## Motivation and references

The [original discussion](https://news.ycombinator.com/item?id=50020169) proposes publishing a keybinding-to-command description instead of building a separate web interface for every TUI.

[OSC 7501](https://www.superlogical.com/rex/docs/build/program-status) reports program status and leaves presentation to the terminal. Its revision 0.3 permits extension through unknown keys, but defines only a fixed feature-detection reply. Reports replace records completely, and some states survive program exit. Actions require an additional contract for activation, capability detection, and lifetime. Support for 7501 alone must not imply support for actions.

The upstream Bubbles [help interface](https://github.com/charmbracelet/bubbles/blob/main/help/help.go) exposes ordered short help and grouped full help. Its [key bindings](https://github.com/charmbracelet/bubbles/blob/main/key/key.go) distinguish actual key strings, display hints, descriptions, and enabled state. Neither interface supplies an application command callback or a persistent action identifier.

## First version

The first version supports publishing a complete description of available key interactions and activating one advertised key gesture at a time. It includes compact and expanded presentations, dynamic updates, clearing, and explicit application overrides for ambiguous help entries.

Forms, arbitrary input strings, key macros, shell commands, URLs, executable markup, named application RPCs, and automatic activation are outside the first version. Named action replies remain a possible extension, especially where an application must reject stale input.

The description is independent of Go, Bubble Tea, booba, and the browser. Other terminal programs can emit it. Other terminal hosts can choose different presentations.

## Description model

The following JSON illustrates the proposed logical model. It is not an assigned OSC sequence or a finalized wire encoding.

```json
{
  "version": 1,
  "instance": "picker-7",
  "context": "files",
  "revision": 12,
  "actions": [
    {
      "id": "open",
      "label": "Open",
      "hint": "enter",
      "enabled": true,
      "keys": [{ "key": "enter", "modifiers": [] }]
    },
    {
      "id": "up",
      "label": "Move up",
      "hint": "↑/k",
      "enabled": true,
      "keys": [
        { "key": "up", "modifiers": [] },
        { "key": "k", "modifiers": [] }
      ]
    },
    {
      "id": "cancel",
      "label": "Cancel",
      "hint": "esc",
      "enabled": true,
      "keys": [{ "key": "escape", "modifiers": [] }]
    }
  ],
  "short": ["open", "cancel"],
  "full": [["up", "open"], ["cancel"]]
}
```

An action is a labeled help entry with one or more independently selectable key gestures. The protocol does not assume its keys are semantically equivalent. For example, an application may group both up and down under a navigation entry. A gesture describes one key with modifiers; it is never an arbitrary sequence of bytes.

| Field | Proposed meaning |
|---|---|
| `version` | Schema version; initially 1 |
| `instance` | Opaque identifier for one application invocation; not proof of process identity |
| `context` | Application-chosen interaction scope, such as a list or dialog |
| `revision` | Increasing safe integer within an instance, including across context changes |
| `actions` | Unique action definitions for this snapshot |
| `short` | Ordered references for compact presentation |
| `full` | Ordered groups of references for expanded presentation |
| `id` | Stable within an instance where the producer can supply identity |
| `label` | Plain human-readable description |
| `hint` | Optional display notation; never parsed as executable input |
| `enabled` | Whether activation is currently available |
| `keys` | Ordered, individually selectable gestures |

Short and full presentations may overlap, and neither must contain the other. A renderer's expanded view also keeps short-only actions reachable. Each reference must resolve to a definition. Duplicate definitions and duplicate references within one presentation are invalid; sharing between short and full is expected. Empty groups may be removed. An empty action set with empty presentations explicitly clears the controls.

Labels and hints are plain Unicode text without terminal controls or markup. IDs use a bounded printable ASCII vocabulary. Exact grammar and limits are part of the wire-format decision below.

## Bubble Tea adapter

The integration accepts `help.KeyMap`, rather than the rendered output of `help.Model`. It reads both help methods regardless of the TUI's current help expansion state. This lets the host expand controls without changing the application's on-screen help.

| Input | Mapping |
|---|---|
| `ShortHelp()` | Ordered short action references |
| `FullHelp()` | Ordered full groups |
| `Help().Desc` | Label |
| `Help().Key` | Display hint |
| `Keys()` | Parsed gestures, preserving order |
| `Enabled()` | Availability; disabled bindings omitted by default, matching help rendering |

Collect the union of both presentations before assigning IDs. Merge identical entries by their complete binding description: ordered keys, help text, and enabled state. Assign deterministic IDs from that canonical description, not slice positions. Equal entries in short and full therefore share an ID. A changed description may receive a new generated ID; explicit application IDs provide stronger continuity.

The adapter cannot infer semantic identity from copied `key.Binding` values. Two deliberately distinct but identical entries need an explicit override. Conflicting entries remain separate. Full canonical equality must be checked when resolving generated-ID collisions.

Multiple keys require particular care. A help binding may represent aliases such as up/k, or distinct gestures such as up/down. Default conversion preserves every key as a separately selectable control within the entry. It must not silently choose the first key and hide the rest. An optional application override may declare aliases and select a preferred gesture, or split an aggregate entry into individually labeled actions.

Suggested adapter options cover explicit IDs, label overrides, preferred gestures for declared aliases, and inclusion of disabled entries. These are proposed API capabilities, not existing exported functions.

Unsupported key notation produces a diagnostic and no activation for that gesture. Never reinterpret an unsupported multi-character key string as text to type. Missing descriptions use the display hint or normalized key name as a fallback label. Empty or unbound entries are omitted. Styling in help text is stripped by the adapter before publication; wire receivers still validate text independently.

The application publishes after changing its active help map or binding state. The publisher compares normalized snapshots and emits only changes, with monotonically increasing revisions. Emission must use Bubble Tea's serialized terminal-output path, rather than printing concurrently with the renderer or embedding side effects in `View`. The concrete output integration requires verification against booba's pinned Bubble Tea fork.

Keep the framework-neutral Go model separate from the Bubbles adapter package. Booba currently depends on Bubble Tea but does not directly require Bubbles; choosing and pinning the adapter's Bubbles version is an implementation step. Supporting Bubbles v1 would require a separate adapter because its binding types differ.

## Key activation

The initial vocabulary should include printable single Unicode scalars, space, enter, escape, tab, backspace, arrows, home, end, insert, delete, page up/down, and F1 through F12. Modifiers are an explicit set drawn from shift, alt, and ctrl. Exact names and supported combinations need a shared Go/TypeScript fixture table before freezing the schema. Unsupported combinations remain unavailable rather than producing approximate input.

A tap represents a complete key gesture. The terminal encodes it using the current terminal modes, including application cursor mode and negotiated keyboard protocols. Where the active protocol reports releases, the gesture includes the corresponding release. Automatic repeat is deferred.

Booba's existing `input(data)` accepts already-encoded input. It is not a symbolic-key encoder. The implementation needs a supported ghostty-web key-encoding entry point or a bounded, tested encoder with explicitly limited capabilities. Dispatching synthetic DOM keyboard events is not the public integration contract.

Activation checks the current connection generation, instance, revision, action ID, gesture, enabled state, and input permissions before encoding. Send the resulting input through the ordinary terminal input path and existing Sip or WASM adapter. Read-only and `disableStdin` restrictions apply to these controls too.

A browser-side revision check prevents tapping a control that the browser already knows is obsolete. It cannot eliminate the race where the program changes state before its output reaches the browser. The backend still receives an ordinary key, without action identity or revision. Applications requiring atomic context validation need a future named-action reply protocol.

## Snapshot lifetime

One active snapshot belongs to each terminal connection. Updates replace the whole snapshot atomically after validation. Within an instance, lower revisions are ignored; equal identical revisions are no-ops and equal conflicting revisions are rejected. A new instance replaces the old one. This identifier provides ordering context, not authentication or foreground ownership.

Clear controls on explicit empty publication, disconnect, adapter replacement, full terminal reset, observed attached-process exit, or a shell prompt boundary. Reconnection starts empty and needs a fresh snapshot. A transport without a resynchronization mechanism may leave controls absent until the producer republishes; the implementation must define a refresh mechanism before claiming reconnect support. Replaying historical output must not reactivate historical actions.

Alternate-screen transitions invalidate controls in the initial implementation. Applications publish after entering the screen on which they want controls. This deliberately differs from OSC 7501 status lifetime.

Process completion is not always observable inside an SSH session or nested shell. Explicit application cleanup and shell integration improve clearing but do not prove which process will receive the next key. Do not claim that status IDs, action IDs, or screen changes solve foreground ownership. Nested application restoration and multiplexer pane routing require further design; the initial model has one active publisher and no automatic restoration stack.

## Transport and OSC 7501

Implementation update: the working prototype uses ordinary OSC 7501 with a
`booba-actions/<instance>` record ID and a decoded message consisting of a
readable summary, ` | booba-actions/v1 `, and compact snapshot JSON. This is
explicitly opt-in and private; other terminals may display the entire suffix.
The message is bounded to 2048 decoded bytes. Buttons return ordinary keyboard
input over the existing transport. Capability detection establishes status
support only. The alternatives below remain future standardization options.


The logical model should be testable before choosing its terminal envelope. A local prototype can feed snapshots directly into the action controller, but that is not completion of the terminal protocol.

Two wire options remain open:

1. An explicitly proposed OSC 7501 extension carrying a bounded encoded description. This needs agreement on capability discovery and independence from status replacement/lifetime. Ordinary status reports must not unexpectedly remove or preserve actionable controls. The existing 4096-byte sequence cap also constrains payload size.
2. A separate action sequence with its own discovery, snapshot, and clear operations. This avoids tying every binding update to a required program status, but requires an identifier and a new parser integration.

Recommendation: define actions as a companion contract to program status and choose the envelope after reviewing the ghostty-web 7501 implementation in progress. Do not allocate an adjacent OSC number or advertise an experimental field as standardized.

Capability discovery must distinguish status support, action-description support, and supported key activation. Any new terminal replies need explicit input-parser handling in the application. No arbitrary payload is reflected into application input.

The eventual OSC parser must handle split sequences, multiple sequences in one write, BEL and ST termination, malformed data, and bounded recovery. Use the terminal parser's event interface when available; do not add a second unbounded scan of output. Sip's existing output framing can carry the sequence without a new Sip message type.

## Browser presentation

An optional action-bar installer mounts into a host-provided element. A headless controller exposes the same validated snapshot and activation API for custom hosts. Proposed API names are `onActionsChange`, `activateAction`, and `installActionBar`; signatures remain to be settled.

The compact view preserves short-help order. An expanded view preserves full-help group order, with short-only entries also reachable. Entries with multiple gestures expose a control for each gesture unless the producer explicitly marks aliases. Limited width moves controls into an accessible overflow view instead of silently discarding them.

Controls use native buttons with readable labels and keyboard access. Expansion is host-local and sends no key to the application. Touch activation avoids focusing the terminal textarea, keeping the software keyboard closed. A separate keyboard control remains available for free-form input. Layout changes refit the terminal through the existing FitAddon path.

Receiving a snapshot never activates a control. Labels are rendered as text, and application descriptions cannot supply HTML, JavaScript, links, CSS, or browser callbacks. The host retains control over whether action UI is enabled and where it appears.

## Implementation sequence and acceptance

1. Implement the framework-neutral model and validators in Go and TypeScript with shared fixtures. Exercise replacement, invalid references, duplicate identities, unsupported versions, limits, and ordering.
2. Implement the Bubbles adapter. Cover overlapping short/full help, grouped navigation, aliases with explicit overrides, disabled entries, unsupported keys, and dynamic context changes.
3. Implement the headless controller and optional browser action bar. A direct-snapshot demo validates interaction and accessibility before depending on a wire envelope.
4. Settle the terminal envelope, parser event, capability detection, publication path, and reconnect refresh. Integrate against the ghostty-web work rather than changing its submodule concurrently.
5. Ship an end-to-end Bubble Tea demo with list navigation and a confirmation dialog. The same bindings must drive TUI help and browser controls over both native/Sip and WASM paths.

Acceptance requires demonstrating compact and expanded views from one help map; an update when entering or leaving a dialog; correct input encoding in supported terminal modes; no software keyboard on touch activation; no input in read-only mode; clearing on observed lifecycle boundaries; rejection of stale browser activations; and bounded parser behavior for fragmented or malformed input. Unsupported terminals retain the ordinary keyboard experience.

## Decisions to settle before wire implementation

- Sequence envelope and capability negotiation, coordinated with the OSC 7501 work.
- Numeric size/count limits, exact identifier grammar, encoding, and unknown-field behavior. The model needs hard limits regardless of envelope; avoid chunking in the first version.
- Supported ghostty-web symbolic-key encoding API and keyboard-mode coverage.
- Serialized Bubble Tea publication API and reconnect snapshot request path.
- Whether the first public adapter exposes alias overrides directly or starts with per-key controls only.

The first implementation can proceed on the model, adapter, and headless controller while these transport decisions remain open.
