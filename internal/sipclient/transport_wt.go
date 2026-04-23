package sipclient

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/quic-go/webtransport-go"

	"github.com/NimbleMarkets/go-booba/sip"
)

// wtFrameConn wraps a *webtransport.Session plus its single bidirectional
// stream into the FrameConn interface.
type wtFrameConn struct {
	session *webtransport.Session
	stream  *webtransport.Stream

	readMu    sync.Mutex // serializes concurrent ReadFrame callers
	closeOnce sync.Once
	closed    bool
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
		// Check if the session was closed with a normal-close code.
		var sessErr *webtransport.SessionError
		if errors.As(err, &sessErr) && sessErr.ErrorCode == webtransport.SessionErrorCode(StatusNormal) {
			return 0, nil, fmt.Errorf("%w: %v", errNormalClose, err)
		}
		if w.closed {
			return 0, nil, fmt.Errorf("%w: %v", errNormalClose, err)
		}
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return 0, nil, errors.New("zero length message")
	}
	if uint64(length) > uint64(sip.MaxMessageSize) {
		return 0, nil, fmt.Errorf("message length %d exceeds MaxMessageSize %d", length, sip.MaxMessageSize)
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
		w.closed = true
		err = w.session.CloseWithError(webtransport.SessionErrorCode(status), reason)
	})
	return err
}

func (w *wtFrameConn) CloseNow() error {
	var err error
	w.closeOnce.Do(func() {
		w.closed = true
		err = w.session.CloseWithError(0, "")
	})
	return err
}

// compile-time assertion
var _ FrameConn = (*wtFrameConn)(nil)
