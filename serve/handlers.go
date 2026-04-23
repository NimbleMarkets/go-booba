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

	"github.com/NimbleMarkets/go-booba/sip"
)

const (
	readBufSize  = 4096
	writeBufSize = 32768
)

// handleWebSocket handles a single WebSocket connection for a session.
func handleWebSocket(ctx context.Context, conn *websocket.Conn, sess Session, opts sip.OptionsMessage, debug bool, cfg Config) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		if err := conn.CloseNow(); err != nil {
			log.Printf("websocket close now: %v", err)
		}
	}()

	// Send options message
	optBytes, _ := json.Marshal(opts)
	if err := writeWSMessage(ctx, conn, sip.MsgOptions, optBytes); err != nil {
		log.Printf("options message write error: %v", err)
		return
	}

	apply, stopThrottle := newResizeApplier(sess, resizeThrottleOrDefault(cfg.ResizeThrottle))
	defer stopThrottle()

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
		handleInputWS(ctx, conn, sess, opts, debug, cfg, apply)
	}()

	wg.Wait()
	_ = conn.Close(websocket.StatusNormalClosure, "session ended")
}

// streamOutputWS reads from PTY and sends as MsgOutput over WebSocket.
func streamOutputWS(ctx context.Context, conn *websocket.Conn, sess Session) {
	buf := make([]byte, writeBufSize)
	for {
		n, err := sess.OutputReader().Read(buf)
		if n > 0 {
			if werr := writeWSMessage(ctx, conn, sip.MsgOutput, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("pty read error: %v", err)
			}
			if werr := writeWSMessage(ctx, conn, sip.MsgClose, nil); werr != nil &&
				!errors.Is(werr, context.Canceled) {
				log.Printf("close message write error: %v", werr)
			}
			_ = conn.Close(websocket.StatusNormalClosure, "session ended")
			return
		}
	}
}

// handleInputWS reads messages from WebSocket and dispatches them.
func handleInputWS(ctx context.Context, conn *websocket.Conn, sess Session, opts sip.OptionsMessage, debug bool, cfg Config, apply func(WindowSize)) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		msgType, payload, err := sip.DecodeWSMessage(data)
		if err != nil {
			continue
		}
		processMessage(ctx, conn, sess, opts, msgType, payload, debug, cfg, apply)
	}
}

// processMessage dispatches a protocol message.
func processMessage(ctx context.Context, conn *websocket.Conn, sess Session, opts sip.OptionsMessage, msgType byte, payload []byte, debug bool, cfg Config, apply func(WindowSize)) {
	switch msgType {
	case sip.MsgInput:
		if opts.ReadOnly {
			return
		}
		if len(payload) == 0 {
			return
		}
		if len(payload) > pasteMaxOrDefault(cfg.MaxPasteBytes) {
			log.Printf("input message exceeds MaxPasteBytes (%d > %d); closing", len(payload), pasteMaxOrDefault(cfg.MaxPasteBytes))
			if conn != nil {
				_ = conn.CloseNow()
			}
			return
		}
		if _, err := sess.InputWriter().Write(payload); err != nil {
			log.Printf("session input write error: %v", err)
		}
	case sip.MsgResize:
		var rm sip.ResizeMessage
		if err := json.Unmarshal(payload, &rm); err == nil && rm.Cols > 0 && rm.Rows > 0 {
			maxDims := windowDimsOrDefault(cfg.MaxWindowDims)
			if rm.Cols > maxDims.Width || rm.Rows > maxDims.Height {
				if debug {
					log.Printf("websocket resize rejected (%dx%d > %dx%d)", rm.Cols, rm.Rows, maxDims.Width, maxDims.Height)
				}
				return
			}
			if debug {
				log.Printf("websocket resize: %dx%d", rm.Cols, rm.Rows)
			}
			apply(WindowSize{Width: rm.Cols, Height: rm.Rows})
		}
	case sip.MsgPing:
		if err := writeWSMessage(ctx, conn, sip.MsgPong, nil); err != nil {
			log.Printf("pong write error: %v", err)
		}
	case sip.MsgKittyKbd:
		if debug {
			log.Printf("kitty keyboard flags: %s", payload)
		}
	default:
		// Unknown types are ignored for forward compatibility; surface them
		// under Debug so protocol regressions don't go silent in dev.
		if debug {
			log.Printf("websocket unknown message type 0x%02x (%d bytes)", msgType, len(payload))
		}
	}
}

func writeWSMessage(ctx context.Context, conn *websocket.Conn, msgType byte, payload []byte) error {
	msg := sip.EncodeWSMessage(msgType, payload)
	return conn.Write(ctx, websocket.MessageBinary, msg)
}

// handleWebTransport handles a single WebTransport session.
func handleWebTransport(ctx context.Context, sess Session, stream *webtransport.Stream, opts sip.OptionsMessage, debug bool, cfg Config) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() { _ = stream.Close() }()

	// Send options message
	optBytes, _ := json.Marshal(opts)
	if err := writeWTMessage(stream, sip.MsgOptions, optBytes); err != nil {
		log.Printf("options message write error: %v", err)
		return
	}

	apply, stopThrottle := newResizeApplier(sess, resizeThrottleOrDefault(cfg.ResizeThrottle))
	defer stopThrottle()

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
		handleInputWT(ctx, sess, stream, opts, debug, cfg, apply)
	}()

	wg.Wait()
}

// streamOutputWT reads from PTY and sends as MsgOutput over WebTransport.
func streamOutputWT(ctx context.Context, sess Session, stream *webtransport.Stream) {
	buf := make([]byte, writeBufSize)
	for {
		n, err := sess.OutputReader().Read(buf)
		if n > 0 {
			if werr := writeWTMessage(stream, sip.MsgOutput, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("pty read error: %v", err)
			}
			if werr := writeWTMessage(stream, sip.MsgClose, nil); werr != nil {
				log.Printf("close message write error: %v", werr)
			}
			// CancelRead stops the server from reading more input; do NOT call
			// CancelWrite which sends RESET_STREAM and can discard the MsgClose
			// frame before the peer reads it. Use Close for a graceful FIN.
			stream.CancelRead(0)
			_ = stream.Close()
			return
		}
	}
}

// handleInputWT reads length-prefixed messages from WebTransport stream.
func handleInputWT(ctx context.Context, sess Session, stream *webtransport.Stream, opts sip.OptionsMessage, debug bool, cfg Config, apply func(WindowSize)) {
	lenBuf := make([]byte, 4)
	for {
		// Read 4-byte length prefix
		if _, err := io.ReadFull(stream, lenBuf); err != nil {
			return
		}
		msgLen := binary.BigEndian.Uint32(lenBuf)
		if msgLen == 0 || msgLen > sip.MaxMessageSize {
			return
		}

		// Read message body
		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(stream, msgBuf); err != nil {
			return
		}

		msgType := msgBuf[0]
		payload := msgBuf[1:]

		processWTMessage(ctx, stream, sess, opts, msgType, payload, debug, cfg, apply)
	}
}

// processWTMessage dispatches a WebTransport protocol message.
func processWTMessage(ctx context.Context, stream *webtransport.Stream, sess Session, opts sip.OptionsMessage, msgType byte, payload []byte, debug bool, cfg Config, apply func(WindowSize)) {
	switch msgType {
	case sip.MsgInput:
		if opts.ReadOnly {
			return
		}
		if len(payload) == 0 {
			return
		}
		if len(payload) > pasteMaxOrDefault(cfg.MaxPasteBytes) {
			log.Printf("input message exceeds MaxPasteBytes (%d > %d); closing", len(payload), pasteMaxOrDefault(cfg.MaxPasteBytes))
			if stream != nil {
				stream.CancelRead(0)
				stream.CancelWrite(0)
				_ = stream.Close()
			}
			return
		}
		if _, err := sess.InputWriter().Write(payload); err != nil {
			log.Printf("session input write error: %v", err)
		}
	case sip.MsgResize:
		var rm sip.ResizeMessage
		if err := json.Unmarshal(payload, &rm); err == nil && rm.Cols > 0 && rm.Rows > 0 {
			maxDims := windowDimsOrDefault(cfg.MaxWindowDims)
			if rm.Cols > maxDims.Width || rm.Rows > maxDims.Height {
				if debug {
					log.Printf("webtransport resize rejected (%dx%d > %dx%d)", rm.Cols, rm.Rows, maxDims.Width, maxDims.Height)
				}
				return
			}
			if debug {
				log.Printf("webtransport resize: %dx%d", rm.Cols, rm.Rows)
			}
			apply(WindowSize{Width: rm.Cols, Height: rm.Rows})
		}
	case sip.MsgPing:
		if err := writeWTMessage(stream, sip.MsgPong, nil); err != nil {
			log.Printf("pong write error: %v", err)
		}
	case sip.MsgKittyKbd:
		if debug {
			log.Printf("kitty keyboard flags: %s", payload)
		}
	default:
		// Unknown types are ignored for forward compatibility; surface them
		// under Debug so protocol regressions don't go silent in dev.
		if debug {
			log.Printf("webtransport unknown message type 0x%02x (%d bytes)", msgType, len(payload))
		}
	}
}

// writeWTMessage writes a length-prefixed message to a WebTransport stream.
func writeWTMessage(stream *webtransport.Stream, msgType byte, payload []byte) error {
	msg := sip.EncodeWTMessage(msgType, payload)
	_, err := stream.Write(msg)
	return err
}
