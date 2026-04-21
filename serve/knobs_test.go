//go:build !js

package serve

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/NimbleMarkets/go-booba/sip"
)

func TestProcessMessageRejectsResizeOverMaxWindowDims(t *testing.T) {
	cfg := Config{MaxWindowDims: WindowSize{Width: 200, Height: 80}}
	sess := &resizeTrackingSession{Session: &resizeTestSession{}}
	rm, _ := json.Marshal(sip.ResizeMessage{Cols: 5000, Rows: 5000})
	applyDirect := func(ws WindowSize) { sess.Resize(ws.Width, ws.Height) }
	processMessage(context.Background(), nil, sess, sip.OptionsMessage{}, sip.MsgResize, rm, false, cfg, applyDirect)
	if sess.lastCols != 0 || sess.lastRows != 0 {
		t.Errorf("Resize was applied (cols=%d rows=%d); want rejected", sess.lastCols, sess.lastRows)
	}
}

func TestProcessMessageAcceptsResizeUnderMaxWindowDims(t *testing.T) {
	cfg := Config{MaxWindowDims: WindowSize{Width: 200, Height: 80}}
	sess := &resizeTrackingSession{Session: &resizeTestSession{}}
	rm, _ := json.Marshal(sip.ResizeMessage{Cols: 100, Rows: 40})
	applyDirect := func(ws WindowSize) { sess.Resize(ws.Width, ws.Height) }
	processMessage(context.Background(), nil, sess, sip.OptionsMessage{}, sip.MsgResize, rm, false, cfg, applyDirect)
	if sess.lastCols != 100 || sess.lastRows != 40 {
		t.Errorf("Resize was not applied (cols=%d rows=%d); want 100x40", sess.lastCols, sess.lastRows)
	}
}

type resizeTrackingSession struct {
	Session
	lastCols, lastRows int
}

func (r *resizeTrackingSession) Resize(cols, rows int) {
	r.lastCols, r.lastRows = cols, rows
}

func TestHandleInputWSClosesOnOversizedPaste(t *testing.T) {
	cfg := Config{MaxPasteBytes: 4096}
	sess := &writeTrackingSession{Session: &resizeTestSession{}}
	huge := make([]byte, 10000)
	noopApply := func(WindowSize) {}
	processMessage(context.Background(), nil, sess, sip.OptionsMessage{}, sip.MsgInput, huge, false, cfg, noopApply)
	if sess.bytesWritten != 0 {
		t.Errorf("oversized input was written (bytes=%d); want 0", sess.bytesWritten)
	}
}

func TestHandleInputWSAcceptsPasteUnderCap(t *testing.T) {
	cfg := Config{MaxPasteBytes: 4096}
	sess := &writeTrackingSession{Session: &resizeTestSession{}}
	payload := make([]byte, 1000)
	noopApply := func(WindowSize) {}
	processMessage(context.Background(), nil, sess, sip.OptionsMessage{}, sip.MsgInput, payload, false, cfg, noopApply)
	if sess.bytesWritten != 1000 {
		t.Errorf("under-cap input bytes=%d; want 1000", sess.bytesWritten)
	}
}

type writeTrackingSession struct {
	Session
	bytesWritten int
}

func (w *writeTrackingSession) InputWriter() io.Writer {
	return writeFunc(func(p []byte) (int, error) {
		w.bytesWritten += len(p)
		return len(p), nil
	})
}

type writeFunc func(p []byte) (int, error)

func (f writeFunc) Write(p []byte) (int, error) { return f(p) }

func TestResizeThrottleCoalesces(t *testing.T) {
	sess := &resizeTrackingSession{Session: &resizeTestSession{}}
	apply, stop := newResizeApplier(sess, 50*time.Millisecond)
	defer stop()

	// Fire 10 resizes within the throttle window.
	for i := 1; i <= 10; i++ {
		apply(WindowSize{Width: i * 10, Height: i * 5})
	}

	// Wait long enough for the throttle to fire.
	time.Sleep(120 * time.Millisecond)

	if sess.lastCols != 100 || sess.lastRows != 50 {
		t.Errorf("after coalescing got %dx%d; want 100x50 (latest value)", sess.lastCols, sess.lastRows)
	}
}
