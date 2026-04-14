# booba - Web-based BubbleTea TUIs using libghostty

<p>
    <a href="https://github.com/NimbleMarkets/booba/tags"><img src="https://img.shields.io/github/tag/NimbleMarkets/booba.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/NimbleMarkets/booba?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/NimbleMarkets/booba/blob/main/CODE_OF_CONDUCT.md"><img src="https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg"  alt="Code Of Conduct"></a>
</p>

`booba` is a Golang module that facilitates embedding [BubbleTea](https://github.com/charmbracelet/bubbletea) Terminal User Interfaces (TUIs) into a Web Browser. Generally, these are accessed via a local terminal or via SSH. This module exposes an HTTP-based terminal connection to a BubbleTea program.

There are two facets we address with this package:

 * Running a full BubbleTea program in a Web Browser (via WebAssembly)

 * Running a Terminal in a browser that connects over WebSockets to a BubbleTea backend

## How and What?

The primary enabling technologies of this are:

 * [`libghostty`](https://github.com/ghostty-org/ghostty) - Terminal emulation engine
 * [`ghostty-web`](https://github.com/coder/ghostty-web) - Web-based terminal using Ghostty's VT100 parser via WebAssembly
 * [`BubbleTea`](https://github.com/charmbracelet/bubbletea) - Terminal UI framework for Go
 * [`WebAssembly`](https://webassembly.org) - For running Go code in browsers

The name `booba` is a portmanteau of the words Boba and Boo, the key ingredient of Bubble Tea leading to a Ghost's exclamation of joy.

## TypeScript API

The `BoobaTerminal` class wraps ghostty-web's Terminal and provides a high-level API for embedding BubbleTea programs:

```javascript
import { BoobaTerminal } from './booba/booba.js';

const booba = new BoobaTerminal('terminal-container', {
    cols: 80,
    rows: 24,
    fontSize: 14,
    scrollback: 1000,
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

### Terminal Options

All [ghostty-web ITerminalOptions](https://github.com/coder/ghostty-web) are supported: `fontSize`, `fontFamily`, `cols`, `rows`, `cursorBlink`, `cursorStyle`, `scrollback`, `allowTransparency`, `convertEol`, `disableStdin`, `smoothScrollDuration`, and a full `theme` with 16-color palette and cursor/selection colors.

### Selection & Clipboard

```javascript
booba.getSelection()       // Get selected text
booba.hasSelection()       // Check if text is selected
booba.copySelection()      // Copy to clipboard
booba.selectAll()          // Select all text
booba.clearSelection()     // Clear selection
booba.select(col, row, len) // Select at position
booba.selectLines(start, end)
booba.getSelectionPosition() // Get selection range
```

### Scrollback & Viewport

```javascript
booba.scrollLines(amount)  // Scroll by lines
booba.scrollPages(amount)  // Scroll by pages
booba.scrollToTop()        // Scroll to top of history
booba.scrollToBottom()     // Scroll to current output
booba.scrollToLine(line)   // Scroll to specific line
booba.getViewportY()       // Get current scroll position
```

### Terminal Control

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

### Terminal Mode Queries

```javascript
booba.hasMouseTracking()   // Is mouse tracking enabled?
booba.hasBracketedPaste()  // Is bracketed paste enabled?
booba.hasFocusEvents()     // Are focus events enabled?
booba.getMode(mode, isAnsi) // Query arbitrary terminal mode
```

### Events

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

### Link Detection

```javascript
const disposable = booba.registerLinkProvider({
    provideLinks(y, callback) {
        // Detect links on row y, call callback with results
        callback(links);
    }
});
disposable.dispose(); // Unregister when done
```

### Custom Event Handlers

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

### Lifecycle

```javascript
booba.dispose()            // Clean up all resources
booba.terminal             // Access underlying ghostty-web Terminal
booba.cols                 // Current column count
booba.rows                 // Current row count
```

### Types

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

For adapter usage (WebSocket, WASM, custom), see [ADAPTER_USAGE.md](./ADAPTER_USAGE.md).

## Embedding a BubbleTea Application in a Web Browser

We can take entire BubbleTea applications and embed them into a Web Browser. The primary limitation is that all of its dependencies can also be compiled to WebAssembly.

TODO: instructions for doing this.
TODO: link to live example

## Web Frontend for BubbleTea-based service

Otherwise, one might have a BubbleTea program running on a remote machine. While one might use `ssh` to access it, `booba` enables an HTTP-based interface to it. Effectively, we serve up a Ghostty terminal from an HTTP endpoint and extend the terminal via WebSockets.

TODO: instructions for doing this.
TODO: link to live example

## Open Collaboration

We welcome contributions and feedback.  Please adhere to our [Code of Conduct](./CODE_OF_CONDUCT.md) when engaging our community.

 * [GitHub Issues](https://github.com/NimbleMarkets/booba/issues)
 * [GitHub Pull Requests](https://github.com/NimbleMarkets/booba/pulls)

## Acknowledgements

Thanks to the [Ghostty developers](https://github.com/ghostty-org/ghostty), the [ghostty-web](https://github.com/coder/ghostty-web) developers, and to [Charm.sh](https://charm.sh) for making the command line glamorous with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Thanks to [@BigJK](https://github.com/BigJk/bubbletea-in-wasm) for the initial inspiration when I was exploring this before `libghostty`.

## License

Released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE.txt](./LICENSE.txt).

Copyright (c) 2025 [Neomantra Corp](https://www.neomantra.com).   

----
Made with :heart: and :fire: by the team behind [Nimble.Markets](https://nimble.markets).
