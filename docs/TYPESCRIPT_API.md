# `BoobaTerminal` TypeScript API

The `BoobaTerminal` class wraps ghostty-web's Terminal and provides a high-level API for embedding BubbleTea programs in a web page.

## Quick Start

Install the package:

```sh
npm install @nimblemarkets/booba
```

```javascript
import { BoobaTerminal } from '@nimblemarkets/booba';
// or, if using the files directly from the server's embedded assets:
// import { BoobaTerminal } from './booba/booba.js';

const booba = new BoobaTerminal('terminal-container', {
    cols: 80,
    rows: 24,
    fontSize: 14,
    scrollback: 1000,
    allowOSC52: true, // Enable OSC 52 clipboard access
    cursorBlink: true,
    cursorStyle: 'block',
    theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
    },
});

await booba.init();
booba.connectWebSocket('ws://localhost:8080/ws');
booba.focus();
```

## Terminal Options

All [ghostty-web ITerminalOptions](https://github.com/coder/ghostty-web) are supported: `fontSize`, `fontFamily`, `cols`, `rows`, `cursorBlink`, `cursorStyle`, `scrollback`, `allowOSC52`, `allowTransparency`, `convertEol`, `disableStdin`, `smoothScrollDuration`, and a full `theme` with 16-color palette and cursor/selection colors.

After `init()`, the container element carries the resolved `theme.background` and `theme.foreground` as the CSS custom properties `--booba-background` and `--booba-foreground`. They inherit to the container and its descendants only, so the padding around the canvas can use `var(--booba-background)` without repeating the hex value.

## Selection & Clipboard

```javascript
booba.getSelection()        // Get selected text
booba.hasSelection()        // Check if text is selected
booba.copySelection()       // Copy to clipboard
booba.selectAll()           // Select all text
booba.clearSelection()      // Clear selection
booba.select(col, row, len) // Select at position
booba.selectLines(start, end)
booba.getSelectionPosition() // Get selection range
```

## Scrollback & Viewport

```javascript
booba.scrollLines(amount)  // Scroll by lines
booba.scrollPages(amount)  // Scroll by pages
booba.scrollToTop()        // Scroll to top of history
booba.scrollToBottom()     // Scroll to current output
booba.scrollToLine(line)   // Scroll to specific line
booba.getViewportY()       // Get current scroll position
```

## Terminal Control

```javascript
booba.paste(data)          // Paste with bracketed paste support
booba.input(data)          // Input as if typed
booba.focus()              // Focus terminal
booba.blur()               // Remove focus
booba.clear()              // Clear screen
booba.reset()              // Reset terminal state
booba.write(data)          // Write to display
booba.writeln(data)        // Write with newline
```

## Terminal Mode Queries

```javascript
booba.hasMouseTracking()    // Is mouse tracking enabled?
booba.hasBracketedPaste()   // Is bracketed paste enabled?
booba.hasFocusEvents()      // Are focus events enabled?
booba.getMode(mode, isAnsi) // Query arbitrary terminal mode
```

## Events

```javascript
booba.onStatusChange = (state, message) => { /* connection state */ };
booba.onTitleChange = (title) => { /* program set window title */ };
booba.onBell = () => { /* bell/beep fired */ };
booba.onSelectionChange = () => { /* selection changed */ };
booba.onKey = (event) => { /* key pressed */ };
booba.onScroll = (viewportY) => { /* viewport scrolled */ };
booba.onRender = ({ start, end }) => { /* rows rendered */ };
booba.onCursorMove = () => { /* cursor moved */ };
```

## Program Status

Programs can describe themselves with the Program Status Protocol (OSC 7501): idle, working with a percentage, done, blocked on a permission, question, or login, or failed. Bubble Tea v2.1 sets it from `View.ProgramStatus`. booba keeps the records in a store that follows the protocol's lifetime rules and can render them outside the canvas.

```javascript
import { installStatusBar } from './booba/booba.js';

booba.programStatus.records          // sorted by id path, root first
booba.programStatus.subscribe(() => render(booba.programStatus.records));

const dispose = installStatusBar(booba.programStatus, {
    parent: document.getElementById('status'),
    onLayout: () => booba.fitAddon?.fit(),
});
```

The store fills only while something consumes status: a `programStatus.subscribe` listener (which `installStatusBar` adds), an `onProgramStatus` callback, or the `allowOSC7501Actions` option. Polling `records` without one of those always sees an empty store.

Lifetimes: each report replaces its record; `state=clear` removes a record and everything beneath it. Working and blocked records drop at a shell prompt and when the process exits. Done and error persist until the user's next input to the program. A full terminal reset clears everything; alternate-screen switches change nothing. The store holds at most 256 records.

The bar is host-styled like the action bar: every element has a stable class (`booba-status-bar`, `booba-status`, `booba-status--<state>`, `booba-status__app`, `booba-status__title`, `booba-status__message`, `booba-status__progress`), state and kind are also `data-state` and `data-kind` attributes, defaults live in `@layer booba` so plain host rules win, and `styles: false` with `STATUS_BAR_CSS` serves hosts whose CSP forbids inline styles. `booba.onProgramStatus` still delivers every report for hosts that keep their own store; `supportsProgramStatus` says whether the loaded runtime emits the event at all. The bar sets the `hidden` attribute when it has nothing to show; a host that restyles the root's `display` should scope that rule with `:not([hidden])`, since an unlayered rule would otherwise reveal the empty bar.

## Link Detection

```javascript
const disposable = booba.registerLinkProvider({
    provideLinks(y, callback) {
        // Detect links on row y, call callback with results
        callback(links);
    }
});
disposable.dispose(); // Unregister when done
```

## Custom Event Handlers

```javascript
booba.attachCustomKeyEventHandler((event) => {
    // Return true to prevent default handling
    return false;
});

booba.attachCustomWheelEventHandler((event) => {
    // Return true to prevent default scroll handling
    return false;
});
```

## Lifecycle

```javascript
booba.dispose()   // Clean up all resources
booba.terminal    // Access underlying ghostty-web Terminal
booba.cols        // Current column count
booba.rows        // Current row count
```

## Renderer HUD (Optional Dev Feature)

Display active renderer backend and live FPS in a corner badge. Booba re-exports ghostty-web's renderer HUD for convenience.

```javascript
import { BoobaTerminal, installRendererHud, parseRendererFromURL } from '@nimblemarkets/booba';

const booba = new BoobaTerminal('container', {
  renderer: parseRendererFromURL(), // respects ?renderer= query param
  cols: 80,
  rows: 24,
});

await booba.init();
booba.connectAuto(wsUrl, wtUrl, certHashUrl);

// Show HUD in bottom-right corner (viewport-fixed)
installRendererHud(booba.terminal);
```

Click the badge to cycle through available renderers: WebGPU (modern hardware-accelerated), WebGL (fallback), and Canvas2D (universal fallback).

**Note:** The `parseRendererFromURL()` function respects the server-injected `window.__boobaDefaultRenderer` global as a deprecated alias for backward compatibility.

### Options & Behavior

For complete option documentation (including `cycle`, `clickToToggle`, `bindToggleHotkey`), see [ghostty-web's renderer HUD documentation](https://github.com/NimbleMarkets/ghostty-web/blob/nm-kitty-meow/lib/renderer-hud.ts).

Key options:
- **`parent`** (HTMLElement): Where to mount the badge. Defaults to `document.body`.
- **`position`** ('fixed' | 'absolute'): Position mode. Default 'fixed' (viewport-relative).
- **`className`** (string): CSS class for custom styling.
- **`clickToToggle`** (boolean): Click badge to cycle renderer. Default `true`.
- **`bindToggleHotkey`** (boolean): Alt+Shift+R to cycle. Default `true`.
- **`cycle`** (array): Override the cycle order. Default: `['webgpu', 'webgl', 'canvas2d']`.

### Query Parameters

`parseRendererFromURL()` reads `?renderer=` from the URL:
- Valid values: `'auto'` (auto-detect), `'webgpu'`, `'webgl'`, `'canvas2d'`
- Invalid or missing: falls back to `window.__boobaDefaultRenderer` (set by the server), then `'auto'`

### Deprecated Aliases

For source compatibility, booba re-exports deprecated type aliases:
- `BoobaRendererHudOptions` → `RendererHudOptions`
- `BoobaRenderer` → `RendererBackend`

These will be removed in a future major version.

## Types

All types are exported for TypeScript consumers:

```typescript
import type {
    BoobaTerminalOptions,
    BoobaTheme,
    BoobaBufferRange,
    BoobaKeyEvent,
    BoobaRenderEvent,
    BoobaLinkProvider,
    BoobaLink,
    BoobaAdapter,
    BoobaConnectionState,
} from './booba/booba.js';
```

For adapter usage (WebSocket, WASM, custom), see [ADAPTER_USAGE.md](../ADAPTER_USAGE.md).
