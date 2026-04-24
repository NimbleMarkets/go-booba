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
