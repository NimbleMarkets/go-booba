package sipclient

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/websocket"

	"github.com/NimbleMarkets/go-booba/sip"
)

// wsFrameConn wraps a *websocket.Conn into the FrameConn interface.
type wsFrameConn struct {
	conn      *websocket.Conn
	closeOnce sync.Once
}

func newWSFrameConn(conn *websocket.Conn) *wsFrameConn {
	// Disable the default 32 KiB read limit so large frames are allowed.
	conn.SetReadLimit(-1)
	return &wsFrameConn{conn: conn}
}

func (w *wsFrameConn) ReadFrame(ctx context.Context) (byte, []byte, error) {
	_, data, err := w.conn.Read(ctx)
	if err != nil {
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			return 0, nil, fmt.Errorf("%w: %v", errNormalClose, err)
		}
		return 0, nil, err
	}
	msgType, payload, derr := sip.DecodeWSMessage(data)
	if derr != nil {
		return 0, nil, fmt.Errorf("%w: decode: %v", ErrProtocol, derr)
	}
	return msgType, payload, nil
}

func (w *wsFrameConn) WriteFrame(ctx context.Context, msgType byte, payload []byte) error {
	return w.conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(msgType, payload))
}

// Close sends a WebSocket close frame with the given status and reason, then
// waits for the peer's echo in a background goroutine so the caller is not
// blocked by the close handshake round-trip. CloseNow is used as the fallback
// if the close has already been initiated.
func (w *wsFrameConn) Close(status StatusCode, reason string) error {
	var err error
	w.closeOnce.Do(func() {
		// Run the full close handshake in a goroutine so the caller returns
		// immediately after the close frame is sent. The peer will echo the
		// close frame when it next calls Read; the library handles the echo
		// automatically.
		go func() { _ = w.conn.Close(websocket.StatusCode(status), reason) }()
	})
	return err
}

func (w *wsFrameConn) CloseNow() error {
	var err error
	w.closeOnce.Do(func() {
		err = w.conn.CloseNow()
	})
	return err
}

// compile-time assertion
var _ FrameConn = (*wsFrameConn)(nil)
