package sipclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/NimbleMarkets/go-booba/sip"
)

// resizeTTY lets the test change the reported size on demand.
type resizeTTY struct {
	*fakeTTY
	mu         sync.Mutex
	cols, rows int
}

func newResizeTTY() *resizeTTY {
	return &resizeTTY{fakeTTY: newFakeTTY(""), cols: 80, rows: 24}
}

func (r *resizeTTY) Size() (int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cols, r.rows, nil
}

func (r *resizeTTY) SetSize(c, ro int) {
	r.mu.Lock()
	r.cols, r.rows = c, ro
	r.mu.Unlock()
}

func TestRunInteractive_ResizeForwarded(t *testing.T) {
	// Override watchResize so the test can drive the callback directly.
	orig := watchResize
	firedCh := make(chan func(), 1)
	watchResize = func(ctx context.Context, cb func()) {
		firedCh <- cb
		<-ctx.Done()
	}
	defer func() { watchResize = orig }()

	resizes := make(chan sip.ResizeMessage, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		for i := 0; i < 2; i++ {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			typ, payload, _ := sip.DecodeWSMessage(data)
			if typ == sip.MsgResize {
				var m sip.ResizeMessage
				_ = json.Unmarshal(payload, &m)
				resizes <- m
			}
		}
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgClose, nil))
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	hs := httptest.NewServer(mux)
	defer hs.Close()
	url := "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws"
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	cancelDial()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	tty := newResizeTTY()
	opts := &Options{URL: url, EscapeCharRaw: "^]"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runInteractive(ctx, newWSFrameConn(conn), tty, opts, nopWriter{}) }()

	// Wait for watchResize to register a callback.
	var fired func()
	select {
	case fired = <-firedCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("watchResize never installed a callback")
	}

	// Change size and fire the callback. The cb installs a 50ms trailing
	// timer, so wait a bit longer than that before expecting the resize
	// to land on the wire.
	tty.SetSize(120, 40)
	fired()

	// Collect two resizes: initial (80x24) + post-SIGWINCH (120x40).
	var got []sip.ResizeMessage
	timeout := time.After(3 * time.Second)
collect:
	for len(got) < 2 {
		select {
		case m := <-resizes:
			got = append(got, m)
		case <-timeout:
			break collect
		}
	}

	// Let runInteractive tear down cleanly.
	cancel()
	<-done

	if len(got) < 2 {
		t.Fatalf("got %d resizes; want 2. got=%+v", len(got), got)
	}
	if got[1].Cols != 120 || got[1].Rows != 40 {
		t.Errorf("second resize = %dx%d; want 120x40", got[1].Cols, got[1].Rows)
	}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
