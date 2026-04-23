package sipclient

import (
	"context"
	"errors"
)

// FrameConn is a transport-agnostic framed connection. Each call to
// ReadFrame returns exactly one decoded [type][payload] message; each call
// to WriteFrame sends exactly one. Implementations wrap WebSocket and
// WebTransport transports, hiding their framing differences from callers.
type FrameConn interface {
	ReadFrame(ctx context.Context) (msgType byte, payload []byte, err error)
	WriteFrame(ctx context.Context, msgType byte, payload []byte) error
	// Close severs the connection cleanly with the given status. Safe to
	// call more than once; subsequent calls are no-ops.
	Close(status StatusCode, reason string) error
	// CloseNow forcibly closes without a close handshake. Safe to use in
	// defer and safe to call more than once.
	CloseNow() error
}

// StatusCode is a transport-agnostic close code. WS implementations map
// directly to websocket.StatusCode values; WT maps to CloseWithError's
// uint32 application code.
type StatusCode uint16

const (
	StatusNormal   StatusCode = 1000
	StatusProtocol StatusCode = 1002
	StatusInternal StatusCode = 1011
)

// errNormalClose is a sentinel used by implementations to signal a
// peer-initiated clean close. IsNormalClose recognizes it and the
// transport-specific equivalents (e.g., websocket.StatusNormalClosure).
var errNormalClose = errors.New("normal close")

// IsNormalClose reports whether err was a clean peer-initiated close. The
// WS implementation returns errors wrapping errNormalClose when it detects
// StatusNormalClosure; the WT implementation does the same for
// CloseWithError(0).
func IsNormalClose(err error) bool {
	return errors.Is(err, errNormalClose)
}
