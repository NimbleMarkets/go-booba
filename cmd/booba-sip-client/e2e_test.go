//go:build !js

package main_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/NimbleMarkets/go-booba/internal/sipclient"
	"github.com/NimbleMarkets/go-booba/serve"
	"github.com/NimbleMarkets/go-booba/sip"
)

// TestE2E_DumpFramesAgainstRealServer spins up a real serve.Server backed by a
// session factory that writes "hello" then closes, exposes it via httptest, runs
// sipclient.RunDump against it, and asserts an output frame containing "hello"
// appears in the JSON frame stream.
//
// Because the SIP protocol requires the client to send an initial Resize before
// the server will respond, we interpose a thin transparent proxy that dials the
// real serve.Server, sends the required initial Resize on behalf of the
// --dump-frames client (which is stateless and doesn't send one), and then
// bridges all subsequent frames bidirectionally. The proxy is trivial and
// introduces no framing logic of its own — all SIP encoding, options negotiation,
// and session lifecycle run through the real serve.Server.
func TestE2E_DumpFramesAgainstRealServer(t *testing.T) {
	// --- 1. Build the real serve.Server with a session that emits "hello" ---

	factory := func(ctx context.Context, size serve.WindowSize) (serve.Session, error) {
		outR, outW := io.Pipe()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() { _ = outW.Close() }()
			_, _ = outW.Write([]byte("hello"))
		}()
		return &helloSession{ctx: ctx, outR: outR, done: done}, nil
	}

	cfg := serve.DefaultConfig()
	srv := serve.NewServer(cfg, serve.WithSessionFactory(factory))

	realHandler, err := srv.HTTPHandler()
	if err != nil {
		t.Fatalf("HTTPHandler: %v", err)
	}

	realTS := httptest.NewServer(realHandler)
	t.Cleanup(realTS.Close)
	realWSURL := "ws" + strings.TrimPrefix(realTS.URL, "http") + "/ws"

	// --- 2. Thin WS proxy that injects the initial Resize for the client ---
	//
	// RunDump doesn't speak the full SIP handshake (no resize send), but the
	// real serve.Server requires a resize before it will produce any output.
	// The proxy: accepts the client WebSocket, dials the real server, sends a
	// synthetic Resize(80×24), then bridges all frames in both directions
	// until either side closes.

	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Accept the client connection.
		clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("proxy: accept client: %v", err)
			return
		}
		defer func() { _ = clientConn.CloseNow() }()

		ctx := r.Context()

		// Dial the real serve server.
		serverConn, _, err := websocket.Dial(ctx, realWSURL, nil)
		if err != nil {
			t.Logf("proxy: dial serve: %v", err)
			return
		}
		defer func() { _ = serverConn.CloseNow() }()

		serverConn.SetReadLimit(sip.MaxMessageSize)

		// Send the initial Resize(80×24) that the real server requires.
		resizeJSON, _ := json.Marshal(sip.ResizeMessage{Cols: 80, Rows: 24})
		resizeFrame := sip.EncodeWSMessage(sip.MsgResize, resizeJSON)
		if err := serverConn.Write(ctx, websocket.MessageBinary, resizeFrame); err != nil {
			t.Logf("proxy: send resize: %v", err)
			return
		}

		// Bridge: server → client
		go func() {
			for {
				_, data, err := serverConn.Read(ctx)
				if err != nil {
					_ = clientConn.CloseNow()
					return
				}
				if err := clientConn.Write(ctx, websocket.MessageBinary, data); err != nil {
					return
				}
			}
		}()

		// Bridge: client → server (blocks until client closes)
		for {
			_, data, err := clientConn.Read(ctx)
			if err != nil {
				return
			}
			if err := serverConn.Write(ctx, websocket.MessageBinary, data); err != nil {
				return
			}
		}
	})

	proxyTS := httptest.NewServer(proxyMux)
	t.Cleanup(proxyTS.Close)

	// --- 3. Run sipclient.RunDump against the proxy ---

	wsURL := "ws" + strings.TrimPrefix(proxyTS.URL, "http") + "/ws"
	var stdout, stderr bytes.Buffer
	opts := &sipclient.Options{
		URL:            wsURL,
		EscapeCharRaw:  "^]",
		ConnectTimeout: 5 * time.Second,
		DumpTimeout:    5 * time.Second,
		DumpFrames:     true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := sipclient.RunDump(ctx, &stdout, &stderr, opts); err != nil {
		t.Fatalf("RunDump: %v (stderr=%s)", err, stderr.String())
	}

	// --- 4. Assert an output frame containing "hello" appears ---

	sawHello := false
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad frame JSON: %v (%q)", err, line)
		}
		if m["type"] == "output" {
			if s, _ := m["data"].(string); strings.Contains(decodeBase64(t, s), "hello") {
				sawHello = true
			}
		}
	}
	if !sawHello {
		t.Errorf("no output frame containing 'hello'.\nstdout:\n%s", stdout.String())
	}
}

func decodeBase64(t *testing.T, s string) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("bad base64: %v", err)
	}
	return string(b)
}

// helloSession is a minimal serve.Session that emits "hello" as its only
// output and then signals done. It accepts but discards all input.
type helloSession struct {
	ctx  context.Context
	outR *io.PipeReader
	done chan struct{}
}

func (s *helloSession) Context() context.Context { return s.ctx }
func (s *helloSession) OutputReader() io.Reader  { return s.outR }
func (s *helloSession) InputWriter() io.Writer   { return io.Discard }
func (s *helloSession) Resize(_, _ int)          {}
func (s *helloSession) WindowSize() serve.WindowSize {
	return serve.WindowSize{Width: 80, Height: 24}
}
func (s *helloSession) Done() <-chan struct{} { return s.done }
func (s *helloSession) Close() error {
	_ = s.outR.CloseWithError(io.EOF)
	return nil
}
