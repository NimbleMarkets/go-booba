//go:build !js

package serve

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"sync"

	"github.com/coder/websocket"
	"github.com/quic-go/webtransport-go"
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

// handleWebTransport handles a single WebTransport session.
func handleWebTransport(ctx context.Context, sess Session, stream *webtransport.Stream, opts OptionsMessage) {
	defer stream.Close()

	// Send options message
	optBytes, _ := json.Marshal(opts)
	writeWTMessage(stream, MsgOptions, optBytes)

	var wg sync.WaitGroup

	// Stream PTY output → client
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamOutputWT(ctx, sess, stream)
	}()

	// Read client input → PTY
	handleInputWT(ctx, sess, stream)

	wg.Wait()
}

// streamOutputWT reads from PTY and sends as MsgOutput over WebTransport.
func streamOutputWT(ctx context.Context, sess Session, stream *webtransport.Stream) {
	buf := make([]byte, writeBufSize)
	for {
		n, err := sess.OutputReader().Read(buf)
		if n > 0 {
			if werr := writeWTMessage(stream, MsgOutput, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("pty read error: %v", err)
			}
			writeWTMessage(stream, MsgClose, nil)
			return
		}
	}
}

// handleInputWT reads length-prefixed messages from WebTransport stream.
func handleInputWT(ctx context.Context, sess Session, stream *webtransport.Stream) {
	lenBuf := make([]byte, 4)
	for {
		// Read 4-byte length prefix
		if _, err := io.ReadFull(stream, lenBuf); err != nil {
			return
		}
		msgLen := binary.BigEndian.Uint32(lenBuf)
		if msgLen == 0 || msgLen > MaxMessageSize {
			return
		}

		// Read message body
		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(stream, msgBuf); err != nil {
			return
		}

		msgType := msgBuf[0]
		payload := msgBuf[1:]

		processWTMessage(ctx, stream, sess, msgType, payload)
	}
}

// processWTMessage dispatches a WebTransport protocol message.
func processWTMessage(ctx context.Context, stream *webtransport.Stream, sess Session, msgType byte, payload []byte) {
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
		writeWTMessage(stream, MsgPong, nil)
	case MsgKittyKbd:
		log.Printf("kitty keyboard flags: %s", payload)
	default:
		// Unknown types silently ignored
	}
}

// writeWTMessage writes a length-prefixed message to a WebTransport stream.
func writeWTMessage(stream *webtransport.Stream, msgType byte, payload []byte) error {
	msg := EncodeWTMessage(msgType, payload)
	_, err := stream.Write(msg)
	return err
}
