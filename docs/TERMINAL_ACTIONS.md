# Terminal Actions (experimental)

This is an opt-in experiment in Booba. APIs and the private wire convention may change without compatibility guarantees. The reference demo lives in `cmd/booba-actions-example`. The ghostty-web runtime embedded since Booba 0.8.0 supports OSC 7501.

Terminal Actions turns an application's help bindings into touch controls. The current implementation provides a transport-independent JSON model, Go publisher, Bubbles v2 adapter, browser controller, and optional action bar. The browser demo carries actions inside OSC 7501 terminal output using an opt-in private convention; activations return as ordinary keyboard input.

## Generate actions from help

`actions/bubblehelp` accepts `help.KeyMap`. Short help becomes the compact presentation; full help retains its groups. Both refer to the same action definitions.

```go
snapshot, diagnostics, err := bubblehelp.Snapshot(
    myKeyMap, "application-instance", "file-list", 1,
    bubblehelp.Options{},
)
```

Import `github.com/NimbleMarkets/go-booba/actions/bubblehelp`. Handle `err` before publishing, and inspect diagnostics for unsupported key strings. Disabled and unbound bindings are omitted by default. Set `IncludeDisabled` to retain disabled controls.

Each key in a help entry is independently selectable. An entry with up/down must expose both gestures. If keys are aliases, `Options.Customize` can set `Action.Preferred` to the desired key index, and optionally assign an explicit ID or label. Generated IDs are eight hexadecimal characters based on the complete original binding; exact copies share an ID across both help presentations. Changing a label or keys changes that generated ID. The adapter currently merges identical entries; represent intentionally distinct actions with distinct binding metadata.

`Snapshot` validates the 2048-byte decoded OSC 7501 message budget, including JSON, marker, and readable summary. Set `Options.Summary` to the summary you will publish; it defaults to context. `NewModel` validates its initial description at construction, using the actual status summary. Later publication errors still clear stale controls and call `OnError` when installed. ID collisions return an error; assign explicit IDs to resolve them.

`actions.NewPublisher(instance, sendJSON)` manages revisions and suppresses unchanged snapshots. Pass each updated snapshot to `Publish`; supplied version, instance, and revision are replaced by the publisher. Failed sends can be retried without losing a revision. `Refresh` resends the latest snapshot when the host requests resynchronization. Its callback must not reenter the publisher. Send errors mean delivery was not confirmed; receivers deduplicate retries by instance and revision.

The publisher has no implicit stdout side effects. Do not print JSON or escapes concurrently with Bubble Tea's renderer. The `bubblehelp.NewModel` wrapper publishes through Bubble Tea’s `tea.Raw`, queries OSC 7501 support, and refreshes the current help map when the model changes.

## Browser controls

The standalone `ActionController` accepts host callbacks for capability checks and complete key activation. `BoobaTerminal.actions` supplies a controller connected to booba's input and connection lifecycle. Install a key encoder explicitly: the current ghostty-web Terminal API does not expose enough keyboard state to assume a general-purpose encoder. Until an encoder is installed, buttons remain disabled.

```ts
booba.setActionKeyEncoder({
    canEncode: gesture => hostEncoder.canEncode(gesture),
    encode: gesture => hostEncoder.encode(gesture),
});

const removeControls = installActionBar(booba.actions, {
    parent: document.getElementById('actions')!,
    openKeyboard: () => booba.focus(),
    onLayout: () => booba.fitAddon?.fit(),
});

// Call after the connection is ready, using a fresh snapshot from the producer.
booba.actions.publish(receivedSnapshot);
```

For applications using legacy VT/xterm keyboard input, Booba supplies a shared encoder:

```ts
booba.setActionKeyEncoder({
    canEncode: gesture => booba.encodeKey(gesture) !== null,
    encode: gesture => booba.encodeKey(gesture),
});
```

`booba.encodeKey` reads current cursor, backarrow, and newline modes on every call. It supports navigation, F1–F12, printable characters, and common modifiers; unsupported combinations return `null`. Shift changes ASCII letter case; shifted punctuation must be supplied as its resulting character. The standalone `encodeKey(gesture, modes)` export accepts explicit `KeyEncodingModes`. Set `extendedKeyboard: true` to decline encoding when an extended protocol is active. These are **legacy encoders**: the pinned ghostty-web API cannot report Kitty keyboard flags or modifyOtherKeys state. Hosts using those protocols must provide a protocol-aware encoder. The legacy encoder is opt-in and is not installed automatically.

`hostEncoder` can instead be supplied by the embedding application. It must encode a complete gesture according to current terminal modes, including releases where required. Returning `null` declines activation. Do not implement it by concatenating help text or dispatching a synthetic browser event. The API deliberately has no default encoder that could silently send an incorrect key.

`installActionBar`, `ActionController`, `decodeActionSnapshot`, and their types are exported from `@nimblemarkets/booba`. For custom controls, read `controller.snapshot`, subscribe to changes with `subscribe`, and use `capture(actionID, keyIndex)` to bind activation. Captured activations become invalid after replacement or clearing. The snapshot getter returns a defensive copy. Call the returned unsubscribe or action-bar disposer during teardown.

The action bar wraps compact controls to available width, exposes full help through More/Less, retains short-only controls in the expanded view, and sends keys only on activation. Labels are plain text. Touch buttons do not focus the terminal textarea. The optional Keyboard button deliberately does.

### Styling

The bar ships structure and a few functional defaults; the host page owns its appearance. Every element carries a stable class, nothing is styled inline, and state lives in attributes the bar already sets:

| Element | Class | State |
| --- | --- | --- |
| Root (`role="group"`) | `booba-action-bar` | |
| Group of related buttons | `booba-action-group` | |
| Any button | `booba-action` | `:disabled` |
| More/Less toggle | `booba-action booba-action--toggle` | `[aria-expanded]` |
| Keyboard button | `booba-action booba-action--keyboard` | |

The defaults (wrapping flex layout, 44px touch targets, `touch-action: manipulation`) are wrapped in `@layer booba`. Unlayered host rules beat layered ones regardless of specificity, so a plain `.booba-action { min-height: 36px }` overrides them without `!important`. If your stylesheet uses layers, declare `@layer booba, app;` first so yours comes later.

`installActionBar` injects that stylesheet once per document as `<style data-booba-styles>`. Under a Content Security Policy that forbids inline styles, pass `styles: false` and serve the exported `ACTION_BAR_CSS` string from your own stylesheet.

After `booba.init()`, the terminal container carries the resolved theme as `--booba-background` and `--booba-foreground`. Custom properties inherit downward, so they apply to the container itself and its descendants, such as the padding gutter around the canvas. Chrome outside the container, like a bar mounted in a sibling element, does not see them. `data-action-focus` attributes are internal and may change.

Booba clears actions on adapter replacement, disconnect/reconnect, reset through `booba.reset()`, and disposal. Forward attached-process exit and replay boundaries to `booba.actions.clear()`. With `allowOSC7501Actions: true`, live output automatically clears controls at shell prompts (OSC 133 A), alternate-screen transitions, and terminal resets. Hosts remain responsible for replay and process boundaries not represented in output. Reconnect requires fresh publication; stored terminal output must not restore old buttons. For a standalone controller, call `setConnected` on each connection boundary and `setReadOnly` when permissions change. A custom booba adapter must forward read-only changes to `booba.actions.setReadOnly`.

Standard WebSocket/WebTransport options and `disableStdin` prevent activation through BoobaTerminal. Stale browser closures are rejected, but ordinary key input cannot guarantee that the remote application still has the advertised context. A future named-action reply would be needed for that guarantee.

## Program status integration

Bubble Tea v2.1 provides `View.ProgramStatus`, `RequestProgramStatusSupport`, and `ProgramStatusSupportMsg`. Booba's pinned WASM fork is based on v2.1.0 as of 0.8.0, so those APIs are available to WASM builds. The help adapter uses Bubbles v2.1.0.

The embedded ghostty-web runtime (`nm-kitty-built` `579fc63`, source `3f84d01`) exposes `onProgramStatus` backed by libghostty-vt parsing. Booba forwards it when the loaded runtime provides it:

```ts
booba.onProgramStatus = report => statusStore.apply(report);
// May be assigned before init. Set to null to unsubscribe.
console.log(booba.supportsProgramStatus); // Inspect after init.
```

Booba does not advertise status support merely because this callback exists on its wrapper: it subscribes to the underlying event only when a consumer is present. `supportsProgramStatus` reports whether the loaded runtime provides the event, which matters when a host supplies its own ghostty-web build.

Reports also land in `booba.programStatus`, a store with the protocol's lifetime rules, which `installStatusBar` renders; see "Program Status" in `docs/TYPESCRIPT_API.md`. An action record reaches the store with its readable summary as the message.

Enable `allowOSC7501Actions: true` in the terminal constructor to interpret the private convention. The standard decoded `msg` contains:

```text
Choose an item | booba-actions/v1 {"version":1,...}
```

The status record ID is `booba-actions/<instance>`; long instances are split into 32-byte path segments. It identifies the replaceable report, while each JSON action has its own ID. The namespace and version marker identify the convention, not the sender's authenticity. Recognized reports expose only the readable prefix to `onProgramStatus`.

The full decoded message must fit OSC 7501's 2048-byte limit. Oversized updates clear previous controls. Same-record replacement without actions, malformed replacement, and ancestor/root clear also remove controls. Unrelated records do not.

This is a private interpretation of a human-readable field, not a standardized action extension. Other status consumers may display the entire JSON suffix. The initial capability query establishes OSC 7501 support only; it does not establish support for action rendering. No new OSC number, Sip opcode, action endpoint, or action RPC is used.

## Try the browser demo

```sh
go run ./cmd/booba-actions-example -listen 127.0.0.1:8081
```

Open http://127.0.0.1:8081. Each tab gets a live Bubble Tea session. Tap More to reveal navigation, select an item, and tap Open. The controls change to Confirm and Back as the Go application changes context. Hide TUI help / Show TUI help (the `h` key) toggles the terminal help while browser controls remain available. Help is visible initially; `?` switches compact/full terminal help and reveals it if hidden. Keyboard opens normal terminal input; Restart starts a fresh session. The expandable snapshot panel shows the exact help-generated description.

To test a ghostty-web build before it is embedded, pass `-ghostty-assets <dir>` with a directory containing `ghostty-web.js`, its imported JavaScript chunks, and the matching `ghostty-vt.wasm`. A source checkout may put JavaScript in `dist/` and WASM at the repository root; assemble them in a temporary directory. The page reports an error if the loaded runtime lacks OSC 7501 support.

The demo uses Booba’s normal Sip WebSocket connection and PTY-backed Bubble Tea sessions. Action reports are ordinary terminal output; button presses are terminal input. It opts into Booba's shared legacy key encoder, including application cursor mode; it does not enable extended keyboard protocols. The demo binds to loopback only.

## Try the reference help map

```sh
# Run the TUI, using the same help map used for action generation.
go run ./cmd/booba-actions-example

# Export either context for a browser host or controller preview.
go run ./cmd/booba-actions-example -snapshot files
go run ./cmd/booba-actions-example -snapshot confirm
```

The native example and browser demo use the same model and help map. Both publish updated action snapshots through the same OSC 7501 wrapper after the terminal answers the capability query.

## Logical schema limits

Version 1 accepts up to 128 actions, 16 gestures per action, and 32 full-help groups. IDs, instance, and context contain 1–64 ASCII letters, digits, underscore, dot, plus, or hyphen. Labels are at most 256 UTF-8 bytes; hints at most 128. Both reject control characters. Revision is a nonnegative JavaScript-safe integer. JSON input is capped at 64 KiB; the OSC 7501 envelope imposes the smaller 2048-byte decoded-message limit. Unknown fields are ignored. Arrays and `enabled` are required; optional `preferred` must be a valid gesture index.

The named keys are space, enter, escape, tab, backspace, up, down, left, right, home, end, insert, delete, pageup, pagedown, and f1–f12. Printable single Unicode scalar values are also accepted. Modifiers are shift, alt, and ctrl without duplicates. Schema acceptance does not promise that a particular host can encode every combination. The Bubble Tea adapter additionally recognizes esc, pgup, pgdown, and a literal space.

Run `go test ./actions/...` and `npm test` to exercise the shared validation fixtures, adapter mapping, publisher retries, lifecycle handling, and DOM controls.
