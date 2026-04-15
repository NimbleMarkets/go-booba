# Booba Deck: External Command Surface for BubbleTea Apps

## Goal

Add a protocol-level mechanism for BubbleTea apps to declare their available actions (the "Deck") so that frontends — mobile web, assistive devices, desktop sidebars, kiosks — can render contextual controls without requiring a full keyboard. The server declares what actions are available; the frontend owns all presentation.

## Design Principles

- **Server declares, frontend renders**: The BubbleTea model is the source of truth for what actions exist. The frontend decides how they appear — buttons, grids, sidebars, or hardware controls.
- **BubbleTea-native**: The Deck is a composable `tea.Model` concept. It receives the same message stream, reacts to state changes, and requires no special signaling.
- **Keystroke injection**: Tapping a Deck button injects the raw keystroke through the existing `MsgInput ('0')` channel. The BubbleTea program cannot distinguish a button tap from a physical keypress. No new input path.
- **Minimal data model**: Groups of items, each with a key and a label. No styling, no icons, no enabled/disabled state. The frontend infers presentation.
- **Opt-in**: Models that don't implement `Decker` are completely unaffected. Zero overhead, no messages sent.

## Protocol

### Message Type

| Type | Name | Direction | Payload | Description |
|------|------|-----------|---------|-------------|
| `'9'` | Deck | server → client | JSON | Current command surface |

This extends the Protocol v2 spec. `'0'`–`'8'` are unchanged. Non-booba clients silently ignore `'9'` (forward-compatible).

### Wire Format

Same framing as all other Protocol v2 messages:

- **WebSocket**: `['9'][JSON payload]`
- **WebTransport**: `[4-byte big-endian length]['9'][JSON payload]`

### Payload

```json
{
  "groups": [
    {
      "name": "Navigation",
      "items": [
        { "key": "k", "label": "Up" },
        { "key": "j", "label": "Down" },
        { "key": "enter", "label": "Select" }
      ]
    },
    {
      "name": "Actions",
      "items": [
        { "key": "d", "label": "Delete" },
        { "key": "/", "label": "Search" },
        { "key": "q", "label": "Quit" }
      ]
    }
  ]
}
```

- `groups` array of `DeckGroup`, each with a `name` and `items` array
- Each `DeckItem` has `key` (the keystroke to inject) and `label` (human-readable description)
- `{"groups": []}` clears the deck entirely
- Max payload size: within the existing 1MB protocol limit

### Input Path

Button taps on the frontend inject the raw keystroke through the existing `MsgInput ('0')` channel:

```
User taps "Delete" button → frontend calls adapter.write("d") → '0' MsgInput → BubbleTea Update
```

No new message type for input. The Deck is purely a server → client rendering hint.

## Go API

### Types

```go
// serve/deck.go

// DeckItem is a single action in the Deck.
type DeckItem struct {
    Key   string `json:"key"`   // Keystroke to inject, e.g. "d", "ctrl+c", "enter"
    Label string `json:"label"` // Human-readable, e.g. "Delete item"
}

// DeckGroup is a named group of related actions.
type DeckGroup struct {
    Name  string     `json:"name"`  // e.g. "Navigation", "Actions"
    Items []DeckItem `json:"items"`
}

// DeckMessage is the wire format for MsgDeck ('9').
type DeckMessage struct {
    Groups []DeckGroup `json:"groups"`
}
```

### Decker Interface

```go
// Decker is implemented by a tea.Model that exposes
// an external command surface (the "Deck").
// Booba checks for this interface on the root model
// after each Update cycle.
type Decker interface {
    // Deck returns the current command surface.
    // Return nil to clear the deck on the client.
    Deck() []DeckGroup
}
```

### DeckFromKeyMap Helper

```go
// DeckFromKeyMap converts a help.KeyMap into DeckGroups.
// Existing BubbleTea apps can expose their help bindings
// as a deck with a single line.
func DeckFromKeyMap(km help.KeyMap) []DeckGroup
```

This iterates `km.FullHelp()`, skips disabled bindings, takes the first key from each binding, and uses the help description as the label. Since `FullHelp()` returns `[][]key.Binding` (columns without names), each column becomes a `DeckGroup` with an empty `Name`. Consumers can set group names after conversion, or frontends can render unnamed groups without headers.

### Usage Patterns

**Pattern 1: Wrap existing help.KeyMap (zero effort)**

```go
func (m model) Deck() []DeckGroup {
    return serve.DeckFromKeyMap(m.keys)
}
```

**Pattern 2: Custom deck (full control)**

```go
func (m model) Deck() []DeckGroup {
    nav := serve.DeckGroup{
        Name: "Navigation",
        Items: []serve.DeckItem{
            {Key: "j", Label: "Down"},
            {Key: "k", Label: "Up"},
        },
    }
    actions := serve.DeckGroup{
        Name: "Actions",
        Items: []serve.DeckItem{
            {Key: "enter", Label: "Select"},
        },
    }
    if m.selected != nil {
        actions.Items = append(actions.Items,
            serve.DeckItem{Key: "d", Label: "Delete"},
        )
    }
    return []serve.DeckGroup{nav, actions}
}
```

**Pattern 3: View-dependent (switches per state)**

```go
func (m model) Deck() []DeckGroup {
    switch m.state {
    case stateList:
        return serve.DeckFromKeyMap(m.listKeys)
    case stateDetail:
        return serve.DeckFromKeyMap(m.detailKeys)
    case stateConfirm:
        return []serve.DeckGroup{{
            Name: "Confirm",
            Items: []serve.DeckItem{
                {Key: "y", Label: "Yes"},
                {Key: "n", Label: "No"},
            },
        }}
    }
    return nil
}
```

## Server Integration

### Update Loop

After each BubbleTea `Update()` cycle, the session handler:

1. Calls `model.View()` → writes to PTY as `'1'` Output (existing behavior)
2. Checks if the root model implements `Decker`
3. If yes, calls `model.Deck()` to get the current surface
4. Compares with the previously sent deck (`reflect.DeepEqual` or JSON hash)
5. If changed → serializes as JSON and sends `'9'` MsgDeck to client

### Diff Strategy

- **Full replace**: Each `'9'` message carries the complete deck. No incremental add/remove/patch. Deck payloads are small (typically < 1KB) so full replacement is efficient and simple for both sides.
- **Send on change only**: The server maintains `lastDeck` per session. Only sends `'9'` when the output of `Deck()` differs from `lastDeck`.
- **Nil = clear**: `Deck()` returning `nil` sends `{"groups":[]}`. The client hides the deck.

### Connection Lifecycle

```
Client                              Server
  |  [connect]                        |
  |--- '2' Resize {cols,rows} ------->|
  |                                   |  create PTY, start BubbleTea
  |<-- '6' Options {readOnly} --------|
  |<-- '1' Output (initial view) -----|
  |<-- '9' Deck (initial commands) ---|  ← sent if model implements Decker
  |                                   |
  |--- '0' Input (user types 'j') --->|  model.Update(KeyMsg)
  |<-- '1' Output (list scrolls) -----|  View() changed
  |                                   |  Deck() unchanged → no '9'
  |                                   |
  |--- '0' Input (user types 'd') --->|  model.Update → m.deleting = true
  |<-- '1' Output (confirm dialog) ---|  View() changed
  |<-- '9' Deck (confirm buttons) ----|  Deck() changed → send
  |                                   |
  |--- '0' Input ('y' via button) --->|  model.Update → delete, back to list
  |<-- '1' Output (updated list) -----|
  |<-- '9' Deck (list commands) ------|  Deck() changed → send
```

## TypeScript API

### Types

```typescript
// ts/types.ts additions

interface DeckItem {
  key: string;    // keystroke to inject, e.g. "d", "ctrl+c"
  label: string;  // display text, e.g. "Delete"
}

interface DeckGroup {
  name: string;       // group label, e.g. "Navigation"
  items: DeckItem[];
}

interface DeckMessage {
  groups: DeckGroup[];
}
```

### BoobaTerminal API

```typescript
interface BoobaTerminalOptions {
  // ... existing options ...

  // Called when the server sends an updated Deck.
  // Receives null when the deck is cleared.
  onDeckChange?: (deck: DeckGroup[] | null) => void;
}

class BoobaTerminal {
  // Returns the current deck, or null if no deck / not connected.
  getDeck(): DeckGroup[] | null;
}
```

### Adapter Integration

The protocol adapter (`BoobaProtocolAdapter`) handles `'9'` messages:

1. Decodes JSON payload as `DeckMessage`
2. Stores as current deck state
3. Dispatches to `onDeckChange` callback
4. Empty groups array → dispatches `null`

### Rendering

Booba's TypeScript library does **not** ship a default deck renderer. It delivers the data via `onDeckChange` and `getDeck()`. The consumer builds whatever UI makes sense for its context:

- **Mobile web**: Touch-friendly buttons below the terminal
- **Assistive device**: Hardware button grid or accessibility overlay
- **Desktop**: Clickable sidebar or floating palette
- **Kiosk**: Large touch targets, labels optional

## Protocol v2 Spec Update

Add `MsgDeck` to the message type table in `docs/superpowers/specs/2026-04-14-ghostty-protocol-design.md`:

| Type | Name | Direction | Payload | Description |
|------|------|-----------|---------|-------------|
| `'9'` | Deck | server → client | `{"groups":[...]}` | Command surface declaration. Sent when the available actions change. |

Add to the Ghostty Extensions section alongside `'8'` (KittyKbd). Forward-compatibility note: non-booba clients silently ignore `'9'`.

## File Changes

| File | Change |
|------|--------|
| `serve/protocol.go` | Add `MsgDeck` constant, `DeckItem`, `DeckGroup`, `DeckMessage` types |
| `serve/deck.go` | `Decker` interface, `DeckFromKeyMap` helper |
| `serve/session.go` | Deck diffing in session update loop (when session.go is built) |
| `ts/types.ts` | `DeckItem`, `DeckGroup`, `DeckMessage` types |
| `ts/booba.ts` | `onDeckChange` callback, `getDeck()` method |
| `ts/adapter.ts` | Handle `'9'` message in protocol adapter |

## Out of Scope

- **Default deck renderer**: Booba delivers data, not UI. Consumers build their own.
- **Styling hints**: No icons, colors, categories, or enabled/disabled state in the protocol. Frontends decide presentation.
- **Semantic command IDs**: Button taps inject raw keystrokes. No command identifiers beyond the key itself.
- **Nested/hierarchical menus**: Flat groups only. If needed later, groups could contain subgroups without breaking the wire format.
- **Bidirectional deck negotiation**: The server does not ask the client what it supports. It sends the deck; the client uses it or ignores it.
