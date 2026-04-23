package sipclient

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/NimbleMarkets/go-booba/sip"
)

// newWSPair returns a paired (server, client) FrameConn for testing. The
// server handler accepts one connection; the returned pair is ready to read
// and write frames.
func newWSPair(t *testing.T) (server, client FrameConn, cleanup func()) {
	t.Helper()
	srvCh := make(chan *websocket.Conn, 1)
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		srvCh <- conn
		<-r.Context().Done()
	}))
	wsURL := "ws" + strings.TrimPrefix(hs.URL, "http") + "/"
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	clientConn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	cancel()
	if err != nil {
		hs.Close()
		t.Fatalf("dial: %v", err)
	}
	serverConn := <-srvCh
	cleanup = func() {
		_ = clientConn.CloseNow()
		_ = serverConn.CloseNow()
		hs.Close()
	}
	return newWSFrameConn(serverConn), newWSFrameConn(clientConn), cleanup
}

func TestWSFrameConn_RoundTrip(t *testing.T) {
	server, client, cleanup := newWSPair(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cases := []struct {
		name    string
		msgType byte
		payload []byte
	}{
		{"output", sip.MsgOutput, []byte("hello")},
		{"input", sip.MsgInput, []byte("")},
		{"ping", sip.MsgPing, nil},
		{"large", sip.MsgOutput, bytes.Repeat([]byte("x"), 64*1024)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := client.WriteFrame(ctx, c.msgType, c.payload); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}
			typ, payload, err := server.ReadFrame(ctx)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}
			if typ != c.msgType {
				t.Errorf("type = %q; want %q", typ, c.msgType)
			}
			if !bytes.Equal(payload, c.payload) && (len(payload) != 0 || len(c.payload) != 0) {
				t.Errorf("payload mismatch: got %d bytes, want %d", len(payload), len(c.payload))
			}
		})
	}
}

func TestWSFrameConn_NormalCloseDetected(t *testing.T) {
	server, client, cleanup := newWSPair(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Close(StatusNormal, "bye"); err != nil {
		t.Fatalf("server.Close: %v", err)
	}
	_, _, err := client.ReadFrame(ctx)
	if err == nil {
		t.Fatalf("expected error after peer close")
	}
	if !IsNormalClose(err) {
		t.Errorf("IsNormalClose(%v) = false; want true", err)
	}
}

func TestWSFrameConn_CanceledContext(t *testing.T) {
	_, client, cleanup := newWSPair(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately
	_, _, err := client.ReadFrame(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v; want context.Canceled", err)
	}
}
