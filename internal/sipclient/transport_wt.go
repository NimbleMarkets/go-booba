package sipclient

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"

	"github.com/NimbleMarkets/go-booba/sip"
)

// http3NoError is the HTTP/3 H3_NO_ERROR application code. When a WebTransport
// session closes cleanly, the underlying HTTP/3 connection may surface this
// code through the QUIC stream before webtransport-go wraps it as a
// SessionError — observed on Linux quic-go but not on macOS.
const http3NoError quic.ApplicationErrorCode = 0x100

// isNormalWTClose reports whether err represents a clean peer-initiated
// WebTransport close. Accepts any of: a webtransport.SessionError with code 0
// or StatusNormal (1000), or a quic.ApplicationError with code 0x100
// (H3_NO_ERROR).
func isNormalWTClose(err error) bool {
	var sessErr *webtransport.SessionError
	if errors.As(err, &sessErr) {
		if sessErr.ErrorCode == 0 || sessErr.ErrorCode == webtransport.SessionErrorCode(StatusNormal) {
			return true
		}
	}
	var appErr *quic.ApplicationError
	if errors.As(err, &appErr) && appErr.ErrorCode == http3NoError {
		return true
	}
	return false
}

// wtFrameConn wraps a *webtransport.Session plus its single bidirectional
// stream into the FrameConn interface.
type wtFrameConn struct {
	session *webtransport.Session
	stream  *webtransport.Stream

	readMu    sync.Mutex // serializes concurrent ReadFrame callers
	closeOnce sync.Once
	closed    atomic.Bool
}

func newWTFrameConn(session *webtransport.Session, stream *webtransport.Stream) *wtFrameConn {
	return &wtFrameConn{session: session, stream: stream}
}

func (w *wtFrameConn) ReadFrame(ctx context.Context) (byte, []byte, error) {
	// Enforce ctx by racing the read against ctx.Done().
	done := make(chan struct{})
	var result struct {
		msgType byte
		payload []byte
		err     error
	}
	go func() {
		defer close(done)
		result.msgType, result.payload, result.err = w.readFrame()
	}()
	select {
	case <-done:
		return result.msgType, result.payload, result.err
	case <-ctx.Done():
		// Unblock the background read by canceling the stream read side.
		w.stream.CancelRead(0)
		<-done
		return 0, nil, ctx.Err()
	}
}

func (w *wtFrameConn) readFrame() (byte, []byte, error) {
	w.readMu.Lock()
	defer w.readMu.Unlock()

	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(w.stream, lenBuf); err != nil {
		if isNormalWTClose(err) {
			return 0, nil, fmt.Errorf("%w: %v", errNormalClose, err)
		}
		if w.closed.Load() {
			return 0, nil, fmt.Errorf("%w: %v", errNormalClose, err)
		}
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return 0, nil, fmt.Errorf("%w: zero length message", ErrProtocol)
	}
	if uint64(length) > uint64(sip.MaxMessageSize) {
		return 0, nil, fmt.Errorf("%w: message length %d exceeds MaxMessageSize %d", ErrProtocol, length, sip.MaxMessageSize)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(w.stream, body); err != nil {
		return 0, nil, err
	}
	return body[0], body[1:], nil
}

func (w *wtFrameConn) WriteFrame(_ context.Context, msgType byte, payload []byte) error {
	_, err := w.stream.Write(sip.EncodeWTMessage(msgType, payload))
	return err
}

func (w *wtFrameConn) Close(status StatusCode, reason string) error {
	var err error
	w.closeOnce.Do(func() {
		w.closed.Store(true)
		err = w.session.CloseWithError(webtransport.SessionErrorCode(status), reason)
	})
	return err
}

func (w *wtFrameConn) CloseNow() error {
	var err error
	w.closeOnce.Do(func() {
		w.closed.Store(true)
		err = w.session.CloseWithError(0, "")
	})
	return err
}

// compile-time assertion
var _ FrameConn = (*wtFrameConn)(nil)
