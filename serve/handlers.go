//go:build !js

package serve

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"

	"github.com/coder/websocket"
)

const (
	readBufSize  = 4096
	writeBufSize = 32768
)

// handleWebSocket handles a single WebSocket connection for a session.
func handleWebSocket(ctx context.Context, conn *websocket.Conn, sess Session, opts OptionsMessage) {
	defer conn.CloseNow()

	// Send options message
	optBytes, _ := json.Marshal(opts)
	writeWSMessage(ctx, conn, MsgOptions, optBytes)

	var wg sync.WaitGroup

	// Stream PTY output → client
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamOutputWS(ctx, conn, sess)
	}()

	// Read client input → PTY
	handleInputWS(ctx, conn, sess)

	wg.Wait()
	conn.Close(websocket.StatusNormalClosure, "session ended")
}

// streamOutputWS reads from PTY and sends as MsgOutput over WebSocket.
func streamOutputWS(ctx context.Context, conn *websocket.Conn, sess Session) {
	buf := make([]byte, writeBufSize)
	for {
		n, err := sess.OutputReader().Read(buf)
		if n > 0 {
			if werr := writeWSMessage(ctx, conn, MsgOutput, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("pty read error: %v", err)
			}
			writeWSMessage(ctx, conn, MsgClose, nil)
			return
		}
	}
}

// handleInputWS reads messages from WebSocket and dispatches them.
func handleInputWS(ctx context.Context, conn *websocket.Conn, sess Session) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		msgType, payload, err := DecodeWSMessage(data)
		if err != nil {
			continue
		}
		processMessage(ctx, conn, sess, msgType, payload)
	}
}

// processMessage dispatches a protocol message.
func processMessage(ctx context.Context, conn *websocket.Conn, sess Session, msgType byte, payload []byte) {
	switch msgType {
	case MsgInput:
		if len(payload) > 0 {
			sess.InputWriter().Write(payload)
		}
	case MsgResize:
		var rm ResizeMessage
		if err := json.Unmarshal(payload, &rm); err == nil && rm.Cols > 0 && rm.Rows > 0 {
			sess.Resize(rm.Cols, rm.Rows)
		}
	case MsgPing:
		writeWSMessage(ctx, conn, MsgPong, nil)
	case MsgKittyKbd:
		log.Printf("kitty keyboard flags: %s", payload)
	default:
		// Unknown message types silently ignored (forward compatibility)
	}
}

func writeWSMessage(ctx context.Context, conn *websocket.Conn, msgType byte, payload []byte) error {
	msg := EncodeWSMessage(msgType, payload)
	return conn.Write(ctx, websocket.MessageBinary, msg)
}
