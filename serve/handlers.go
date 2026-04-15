//go:build !js

package serve

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		if err := conn.CloseNow(); err != nil {
			log.Printf("websocket close now: %v", err)
		}
	}()

	// Send options message
	optBytes, _ := json.Marshal(opts)
	if err := writeWSMessage(ctx, conn, MsgOptions, optBytes); err != nil {
		log.Printf("options message write error: %v", err)
		return
	}

	var wg sync.WaitGroup
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel()
			if err := sess.Close(); err != nil && !errors.Is(err, io.EOF) {
				log.Printf("session close error: %v", err)
			}
		})
	}

	// Stream PTY output → client
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cleanup()
		streamOutputWS(ctx, conn, sess)
	}()

	// Read client input → PTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cleanup()
		handleInputWS(ctx, conn, sess, opts)
	}()

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
			if werr := writeWSMessage(ctx, conn, MsgClose, nil); werr != nil &&
				!errors.Is(werr, context.Canceled) {
				log.Printf("close message write error: %v", werr)
			}
			_ = conn.Close(websocket.StatusNormalClosure, "session ended")
			return
		}
	}
}

// handleInputWS reads messages from WebSocket and dispatches them.
func handleInputWS(ctx context.Context, conn *websocket.Conn, sess Session, opts OptionsMessage) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		msgType, payload, err := DecodeWSMessage(data)
		if err != nil {
			continue
		}
		processMessage(ctx, conn, sess, opts, msgType, payload)
	}
}

// processMessage dispatches a protocol message.
func processMessage(ctx context.Context, conn *websocket.Conn, sess Session, opts OptionsMessage, msgType byte, payload []byte) {
	switch msgType {
	case MsgInput:
		if opts.ReadOnly {
			return
		}
		if len(payload) > 0 {
			if _, err := sess.InputWriter().Write(payload); err != nil {
				log.Printf("session input write error: %v", err)
			}
		}
	case MsgResize:
		var rm ResizeMessage
		if err := json.Unmarshal(payload, &rm); err == nil && rm.Cols > 0 && rm.Rows > 0 {
			sess.Resize(rm.Cols, rm.Rows)
		}
	case MsgPing:
		if err := writeWSMessage(ctx, conn, MsgPong, nil); err != nil {
			log.Printf("pong write error: %v", err)
		}
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer stream.Close()

	// Send options message
	optBytes, _ := json.Marshal(opts)
	if err := writeWTMessage(stream, MsgOptions, optBytes); err != nil {
		log.Printf("options message write error: %v", err)
		return
	}

	var wg sync.WaitGroup
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel()
			if err := sess.Close(); err != nil && !errors.Is(err, io.EOF) {
				log.Printf("session close error: %v", err)
			}
		})
	}

	// Stream PTY output → client
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cleanup()
		streamOutputWT(ctx, sess, stream)
	}()

	// Read client input → PTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cleanup()
		handleInputWT(ctx, sess, stream, opts)
	}()

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
			if werr := writeWTMessage(stream, MsgClose, nil); werr != nil {
				log.Printf("close message write error: %v", werr)
			}
			stream.CancelRead(0)
			stream.CancelWrite(0)
			_ = stream.Close()
			return
		}
	}
}

// handleInputWT reads length-prefixed messages from WebTransport stream.
func handleInputWT(ctx context.Context, sess Session, stream *webtransport.Stream, opts OptionsMessage) {
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

		processWTMessage(ctx, stream, sess, opts, msgType, payload)
	}
}

// processWTMessage dispatches a WebTransport protocol message.
func processWTMessage(ctx context.Context, stream *webtransport.Stream, sess Session, opts OptionsMessage, msgType byte, payload []byte) {
	switch msgType {
	case MsgInput:
		if opts.ReadOnly {
			return
		}
		if len(payload) > 0 {
			if _, err := sess.InputWriter().Write(payload); err != nil {
				log.Printf("session input write error: %v", err)
			}
		}
	case MsgResize:
		var rm ResizeMessage
		if err := json.Unmarshal(payload, &rm); err == nil && rm.Cols > 0 && rm.Rows > 0 {
			sess.Resize(rm.Cols, rm.Rows)
		}
	case MsgPing:
		if err := writeWTMessage(stream, MsgPong, nil); err != nil {
			log.Printf("pong write error: %v", err)
		}
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
