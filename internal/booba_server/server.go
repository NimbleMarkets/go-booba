// Package bubbletea_server provides WebSocket server functionality for BubbleTea applications.
// It handles WebSocket connections and bridges them to BubbleTea programs using a custom protocol.
package booba_server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	tea "charm.land/bubbletea/v2"
	"github.com/gorilla/websocket"
)

// Protocol message types
const (
	MsgInput  = 0x01 // User input from client
	MsgResize = 0x02 // Terminal resize event
)

// Server handles WebSocket connections for BubbleTea programs.
type Server struct {
	upgrader websocket.Upgrader
}

// NewServer creates a new BubbleTea WebSocket server.
func NewServer() *Server {
	return &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // TODO: Make this configurable
			},
		},
	}
}

// Handler returns an http.HandlerFunc that handles WebSocket connections.
// The modelFactory function is called for each connection to create a new BubbleTea model.
func (s *Server) Handler(modelFactory func() tea.Model, options ...tea.ProgramOption) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		defer conn.Close()

		// Create the BubbleTea model
		model := modelFactory()

		// Create WebSocket adapter
		adapter := newWebSocketAdapter(conn)

		// Wait for the client to send its actual terminal size before
		// starting the program. The client sends a resize message (0x02)
		// immediately on connect.
		initialSize := adapter.waitForInitialSize()
		log.Printf("Initial terminal size: %dx%d", initialSize.Width, initialSize.Height)

		// Create BubbleTea program with WebSocket I/O
		prog := tea.NewProgram(model, append([]tea.ProgramOption{
			tea.WithInput(adapter),
			tea.WithOutput(adapter),
		}, options...)...)

		adapter.program = prog

		// Send the initial size now that the program exists
		go func() {
			prog.Send(initialSize)
		}()

		// Run the program (blocks until program exits)
		if _, err := prog.Run(); err != nil {
			log.Println("BubbleTea program error:", err)
		}
	}
}

// webSocketAdapter adapts a WebSocket connection to io.ReadWriter for BubbleTea.
type webSocketAdapter struct {
	conn    *websocket.Conn
	buf     bytes.Buffer
	program *tea.Program
}

// waitForInitialSize reads from the WebSocket until a resize message arrives,
// buffering any input messages that arrive first. Returns the initial window size.
// Falls back to 80x24 if the first message isn't a resize.
func (a *webSocketAdapter) waitForInitialSize() tea.WindowSizeMsg {
	for {
		_, message, err := a.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error waiting for initial size: %v", err)
			return tea.WindowSizeMsg{Width: 80, Height: 24}
		}

		if len(message) == 0 {
			continue
		}

		msgType := message[0]
		payload := message[1:]

		switch msgType {
		case MsgResize:
			var size struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			}
			if err := json.Unmarshal(payload, &size); err == nil && size.Cols > 0 && size.Rows > 0 {
				return tea.WindowSizeMsg{Width: size.Cols, Height: size.Rows}
			}
			log.Printf("Invalid initial resize message, using default: %v", err)
			return tea.WindowSizeMsg{Width: 80, Height: 24}

		case MsgInput:
			// Buffer any input that arrives before the resize
			if len(payload) > 0 {
				a.buf.Write(payload)
			}

		default:
			log.Printf("Unknown message type waiting for initial size: 0x%02x", msgType)
		}
	}
}

func newWebSocketAdapter(conn *websocket.Conn) *webSocketAdapter {
	return &webSocketAdapter{
		conn: conn,
	}
}

// Read implements io.Reader, reading from the WebSocket and handling the protocol.
func (a *webSocketAdapter) Read(p []byte) (n int, err error) {
	if a.buf.Len() > 0 {
		return a.buf.Read(p)
	}

	for {
		_, message, err := a.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return 0, err
		}

		if len(message) == 0 {
			continue
		}

		msgType := message[0]
		payload := message[1:]

		switch msgType {
		case MsgInput:
			log.Printf("WebSocket input: %d bytes", len(payload))
			if len(payload) == 0 {
				continue
			}
			a.buf.Write(payload)
			return a.buf.Read(p)

		case MsgResize:
			if len(payload) >= 4 {
				var size struct {
					Cols int `json:"cols"`
					Rows int `json:"rows"`
				}
				if err := json.Unmarshal(payload, &size); err == nil {
					log.Printf("WebSocket resize: %dx%d", size.Cols, size.Rows)
					if a.program != nil {
						a.program.Send(tea.WindowSizeMsg{Width: size.Cols, Height: size.Rows})
					}
				} else {
					log.Printf("WebSocket resize error: %v", err)
				}
			}
			// Continue loop to read next message

		default:
			log.Printf("Unknown message type: 0x%02x", msgType)
		}
	}
}

// Write implements io.Writer, writing to the WebSocket.
func (a *webSocketAdapter) Write(p []byte) (n int, err error) {
	//TODO	log.Printf("WebSocket write: %d bytes", len(p))
	err = a.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		log.Printf("WebSocket write error: %v", err)
		return 0, err
	}
	return len(p), nil
}

var _ io.ReadWriter = (*webSocketAdapter)(nil)
