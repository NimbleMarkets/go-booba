# booba_server Package

The `booba_server` package provides a simple, reusable way to serve BubbleTea applications over WebSocket.

## Features

- **Clean API**: One function call to set up a WebSocket handler
- **Protocol Handling**: Automatically handles the binary protocol (0x01 for input, 0x02 for resize)
- **Model Factory**: Creates a fresh model instance for each connection
- **Automatic Sizing**: Sends initial window size message on connection

## Usage

### Basic Example

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/NimbleMarkets/booba/internal/booba_server"
    tea "github.com/charmbracelet/bubbletea/v2"
)

func main() {
    // Create the server
    btServer := booba_server.NewServer()
    
    // Serve static assets
    http.Handle("/", http.FileServer(http.Dir("assets")))
    
    // Handle WebSocket connections
    http.HandleFunc("/ws", btServer.Handler(func() tea.Model {
        // This function is called for each new connection
        // Return a fresh model instance
        return myModel{}
    }))
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### With Custom Options

```go
http.HandleFunc("/ws", btServer.Handler(
    func() tea.Model {
        return myModel{}
    },
    tea.WithAltScreen(),           // Enable alternate screen
    tea.WithMouseCellMotion(),     // Enable mouse support
))
```

Mouse events from the browser terminal (clicks, scrolls, motion) are encoded as escape sequences by ghostty-web and sent through the `0x01` input protocol. When you pass `tea.WithMouseCellMotion()` or `tea.WithMouseAllMotion()`, BubbleTea will receive `tea.MouseMsg` events as expected.

## Protocol

The package uses a simple binary protocol for WebSocket communication:

### Client → Server

**Input Message** (0x01)
```
[0x01][...user input bytes...]
```

**Resize Message** (0x02)
```
[0x02][...JSON: {"cols": N, "rows": M}...]
```

### Server → Client

Raw terminal output (ANSI escape sequences) as binary data.

## Implementation Details

The package provides:

- `Server`: Main server struct with WebSocket upgrader configuration
- `NewServer()`: Creates a new server instance
- `Handler(modelFactory, ...options)`: Returns an `http.HandlerFunc` for WebSocket connections
- `webSocketAdapter`: Internal adapter that implements `io.ReadWriter` for BubbleTea

The adapter:
- Reads from WebSocket and decodes the protocol
- Writes terminal output directly to WebSocket
- Forwards resize events to the BubbleTea program via `WindowSizeMsg`

## Thread Safety

Each WebSocket connection runs in its own goroutine with its own BubbleTea program instance. There are no shared resources between connections.

## Error Handling

Errors are logged to the standard logger:
- WebSocket upgrade failures
- Protocol decoding errors
- BubbleTea program errors
- Connection read/write errors

## Future Enhancements

Potential additions:
- Configurable `CheckOrigin` policy
- Custom logger injection
- Connection metrics/monitoring
- Authentication middleware
- Rate limiting
