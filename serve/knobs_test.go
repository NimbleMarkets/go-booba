//go:build !js

package serve

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestProcessMessageRejectsResizeOverMaxWindowDims(t *testing.T) {
	cfg := Config{MaxWindowDims: WindowSize{Width: 200, Height: 80}}
	sess := &resizeTrackingSession{Session: &resizeTestSession{}}
	rm, _ := json.Marshal(ResizeMessage{Cols: 5000, Rows: 5000})
	processMessage(context.Background(), nil, sess, OptionsMessage{}, MsgResize, rm, false, cfg)
	if sess.lastCols != 0 || sess.lastRows != 0 {
		t.Errorf("Resize was applied (cols=%d rows=%d); want rejected", sess.lastCols, sess.lastRows)
	}
}

func TestProcessMessageAcceptsResizeUnderMaxWindowDims(t *testing.T) {
	cfg := Config{MaxWindowDims: WindowSize{Width: 200, Height: 80}}
	sess := &resizeTrackingSession{Session: &resizeTestSession{}}
	rm, _ := json.Marshal(ResizeMessage{Cols: 100, Rows: 40})
	processMessage(context.Background(), nil, sess, OptionsMessage{}, MsgResize, rm, false, cfg)
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
	processMessage(context.Background(), nil, sess, OptionsMessage{}, MsgInput, huge, false, cfg)
	if sess.bytesWritten != 0 {
		t.Errorf("oversized input was written (bytes=%d); want 0", sess.bytesWritten)
	}
}

func TestHandleInputWSAcceptsPasteUnderCap(t *testing.T) {
	cfg := Config{MaxPasteBytes: 4096}
	sess := &writeTrackingSession{Session: &resizeTestSession{}}
	payload := make([]byte, 1000)
	processMessage(context.Background(), nil, sess, OptionsMessage{}, MsgInput, payload, false, cfg)
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
