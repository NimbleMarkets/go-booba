# Booba Adapter Usage Guide

The BubbleTea adapter abstraction allows you to connect to Booba-based BubbleTea programs in multiple ways. For the full `BoobaTerminal` API reference, see [docs/TYPESCRIPT_API.md](./docs/TYPESCRIPT_API.md).

## 1. WebSocket Mode (Backend Server)

Connect to a BubbleTea application running on a backend server via WebSocket.

```javascript
import { BoobaTerminal } from './booba/booba.js';

const booba = new BoobaTerminal('terminal-container', {
    cols: 80,
    rows: 24,
    scrollback: 1000,
    cursorBlink: true,
});

booba.onStatusChange = (state, message) => {
    console.log(`Connection ${state}: ${message}`);
};

booba.onTitleChange = (title) => {
    document.title = title || 'My Terminal';
};

await booba.init();
booba.connectWebSocket('ws://localhost:8080/ws');
booba.focus();
```

**Protocol**: Uses a custom binary protocol
- `0x01` + data: User input
- `0x02` + JSON: Terminal resize (`{"cols": N, "rows": M}`)

## 1.5. Auto Mode (WebTransport with WebSocket Fallback)

`connectAuto` tries WebTransport first when the browser supports it and a certificate hash endpoint is available, then falls back to WebSocket automatically.

```javascript
const wsUrl = 'ws://localhost:8080/ws';
const wtUrl = 'https://localhost:8080/wt';
const certHashUrl = 'https://localhost:8080/cert-hash';

await booba.init();
booba.connectAuto(wsUrl, wtUrl, certHashUrl);
```

If you disable WebTransport on the server, pass `null` for `wtUrl` and `certHashUrl` or use `connectWebSocket(...)` directly.

## 2. WASM Mode (Pure Embedding)

Connect to a BubbleTea application compiled to WebAssembly and running in the browser.

```javascript
import { BoobaTerminal } from './booba/booba.js';

const booba = new BoobaTerminal('terminal-container');

await booba.init();
booba.connectWasm(16); // Poll every 16ms (~60fps)
```

**Go side**: Use `booba.Run` or `booba.NewProgram` (from `github.com/NimbleMarkets/go-booba`) as the entry point — these wire up the JS bridge automatically when compiled with `GOARCH=wasm GOOS=js`. Build with `booba-wasm-build`:

```sh
go run github.com/NimbleMarkets/go-booba/cmd/booba-wasm-build -o web/app.wasm ./cmd/myapp/
```

For advanced use cases that need direct control over the JS bridge (custom `js.FuncOf` callbacks, manual buffer management, etc.), the [`wasm`](./wasm) subpackage exposes the low-level API.

**Required JS globals** (registered automatically by `booba.Run` / `booba.NewProgram`):
- `window.bubbletea_write(data: string): void`
- `window.bubbletea_read(): string`
- `window.bubbletea_resize(cols: number, rows: number): void`

## 3. Custom Adapter

Implement your own `BoobaAdapter` for custom transport mechanisms:

```typescript
import { BoobaAdapter, BoobaConnectionState } from './booba/adapter.js';

class MyCustomAdapter implements BoobaAdapter {
    boobaRead(): string | Uint8Array | null {
        // Your implementation
    }
    
    boobaWrite(data: string | Uint8Array): void {
        // Your implementation
    }
    
    boobaResize(cols: number, rows: number): void {
        // Your implementation
    }
    
    connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): void {
        // Your implementation
    }
    
    disconnect(): void {
        // Your implementation
    }
}

const adapter = new MyCustomAdapter();
booba.connectAdapter(adapter);
```

## TypeScript Naming Conventions

The adapter follows TypeScript naming conventions:

- **Interface names**: `PascalCase` (e.g., `BoobaAdapter`)
- **Method names**: `camelCase` (e.g., `boobaRead`, `boobaWrite`, `boobaResize`)
- **Type names**: `PascalCase` (e.g., `ConnectionState`)

## Adapter Methods

### `boobaRead(): string | Uint8Array | null`
Read output from the BubbleTea program. Returns `null` if no data is available.

### `boobaWrite(data: string | Uint8Array): void`
Send user input to the BubbleTea program.

### `boobaResize(cols: number, rows: number): void`
Notify the BubbleTea program of a terminal resize event.

### `connect(onData, onStateChange): void`
Set up the connection and register callbacks for received data and connection state changes.

### `disconnect(): void`
Close the connection and clean up resources.

## Terminal Features Available Across All Adapters

Regardless of which adapter you use, all `BoobaTerminal` features work the same way. The adapter only handles the transport (how data gets to/from the BubbleTea program). Features like selection, scrollback, paste, focus, link detection, and events are handled by the terminal layer above the adapter.

**Mouse tracking**: If your BubbleTea program enables mouse tracking (e.g., via `tea.WithMouseCellMotion()`), mouse events are encoded as escape sequences by ghostty-web and flow through the adapter's `boobaWrite` as regular input data. No adapter changes are needed.

**Bracketed paste**: Similarly, `booba.paste(data)` wraps the text in bracketed paste escape sequences when the program has enabled bracketed paste mode. The escape sequences flow through `boobaWrite` transparently.

**Lifecycle**: Always call `booba.dispose()` when tearing down the terminal to clean up event listeners and resources. This automatically calls `disconnect()` on the adapter.
