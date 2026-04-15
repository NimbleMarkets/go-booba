//go:build !js

package serve

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWSE2E_ConnectsAfterValidResizeAndSendsOptionsFirst(t *testing.T) {
	ts, _ := newWSE2ETestServer(t, DefaultConfig())

	conn, opts := mustConnectWS(t, ts.URL, nil, ResizeMessage{Cols: 80, Rows: 24})
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if opts.ReadOnly {
		t.Fatal("expected readOnly=false")
	}
}

func TestWSE2E_InvalidInitialResizeIsRejected(t *testing.T) {
	t.Run("wrong first message type", func(t *testing.T) {
		ts, _ := newWSE2ETestServer(t, DefaultConfig())

		conn := mustDialWS(t, ts.URL, nil)
		defer conn.Close(websocket.StatusNormalClosure, "test done")

		writeWS(t, conn, MsgInput, []byte("not-a-resize"))
		assertConnectionClosed(t, conn)
	})

	t.Run("invalid resize payload", func(t *testing.T) {
		ts, _ := newWSE2ETestServer(t, DefaultConfig())

		conn := mustDialWS(t, ts.URL, nil)
		defer conn.Close(websocket.StatusNormalClosure, "test done")

		payload := mustJSON(t, ResizeMessage{Cols: 0, Rows: 24})
		writeWS(t, conn, MsgResize, payload)
		assertConnectionClosed(t, conn)
	})
}

func TestWSE2E_RequiresInitialResizeBeforeOptions(t *testing.T) {
	ts, _ := newWSE2ETestServer(t, DefaultConfig())

	conn := mustDialWS(t, ts.URL, nil)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, _, err := conn.Read(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read error = %v, want context deadline exceeded", err)
	}
}

func TestWSE2E_ForwardsInputToSession(t *testing.T) {
	ts, created := newWSE2ETestServer(t, DefaultConfig())

	conn, _ := mustConnectWS(t, ts.URL, nil, ResizeMessage{Cols: 80, Rows: 24})
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	sess := waitForSession(t, created)
	writeWS(t, conn, MsgInput, []byte("hello"))

	if got := sess.waitForInput(t); got != "hello" {
		t.Fatalf("input = %q, want %q", got, "hello")
	}
}

func TestWSE2E_EmitsOutputToClient(t *testing.T) {
	ts, created := newWSE2ETestServer(t, DefaultConfig())

	conn, _ := mustConnectWS(t, ts.URL, nil, ResizeMessage{Cols: 80, Rows: 24})
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	sess := waitForSession(t, created)
	sess.emitOutput(t, "ready>")

	msgType, payload := readWSMessage(t, conn)
	if msgType != MsgOutput {
		t.Fatalf("message type = %q, want %q", msgType, MsgOutput)
	}
	if string(payload) != "ready>" {
		t.Fatalf("payload = %q, want %q", payload, "ready>")
	}
}

func TestWSE2E_MsgPingReturnsPong(t *testing.T) {
	ts, _ := newWSE2ETestServer(t, DefaultConfig())

	conn, _ := mustConnectWS(t, ts.URL, nil, ResizeMessage{Cols: 80, Rows: 24})
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	writeWS(t, conn, MsgPing, nil)

	msgType, payload := readWSMessage(t, conn)
	if msgType != MsgPong {
		t.Fatalf("message type = %q, want %q", msgType, MsgPong)
	}
	if len(payload) != 0 {
		t.Fatalf("payload length = %d, want 0", len(payload))
	}
}

func TestWSE2E_ReadOnlyBlocksInput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ReadOnly = true
	ts, created := newWSE2ETestServer(t, cfg)

	conn, opts := mustConnectWS(t, ts.URL, nil, ResizeMessage{Cols: 80, Rows: 24})
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if !opts.ReadOnly {
		t.Fatal("expected readOnly=true")
	}

	sess := waitForSession(t, created)
	writeWS(t, conn, MsgInput, []byte("blocked"))
	sess.assertNoInput(t)
}

func TestWSE2E_BasicAuthGateWorks(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BasicUsername = "admin"
	cfg.BasicPassword = "secret"
	ts, _ := newWSE2ETestServer(t, cfg)

	_, resp, err := dialWS(context.Background(), ts.URL, nil)
	if err == nil {
		t.Fatal("expected websocket dial without auth to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want %d", statusCode(resp), http.StatusUnauthorized)
	}

	headers := make(http.Header)
	headers.Set("Authorization", basicAuthHeader("admin", "secret"))
	conn, opts := mustConnectWS(t, ts.URL, headers, ResizeMessage{Cols: 80, Rows: 24})
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if opts.ReadOnly {
		t.Fatal("expected readOnly=false")
	}
}

func TestWSE2E_OriginChecksWorkOnHandshake(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OriginPatterns = []string{"https://*.example.com"}
	ts, _ := newWSE2ETestServer(t, cfg)

	badHeaders := make(http.Header)
	badHeaders.Set("Origin", "https://evil.example.net")
	_, resp, err := dialWS(context.Background(), ts.URL, badHeaders)
	if err == nil {
		t.Fatal("expected websocket dial with disallowed origin to fail")
	}
	if resp == nil || resp.StatusCode < 400 {
		t.Fatalf("status = %v, want client/server error", statusCode(resp))
	}

	goodHeaders := make(http.Header)
	goodHeaders.Set("Origin", "https://app.example.com")
	conn, _ := mustConnectWS(t, ts.URL, goodHeaders, ResizeMessage{Cols: 80, Rows: 24})
	defer conn.Close(websocket.StatusNormalClosure, "test done")
}

func TestWSE2E_MaxConnectionsLimitWorks(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConnections = 1
	ts, _ := newWSE2ETestServer(t, cfg)

	first := mustDialWS(t, ts.URL, nil)
	defer first.Close(websocket.StatusNormalClosure, "test done")

	_, resp, err := dialWS(context.Background(), ts.URL, nil)
	if err == nil {
		t.Fatal("expected second websocket dial to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %v, want %d", statusCode(resp), http.StatusServiceUnavailable)
	}
}

func TestWSE2E_ClientDisconnectClosesSession(t *testing.T) {
	ts, created := newWSE2ETestServer(t, DefaultConfig())

	conn, _ := mustConnectWS(t, ts.URL, nil, ResizeMessage{Cols: 80, Rows: 24})
	sess := waitForSession(t, created)

	if err := conn.Close(websocket.StatusNormalClosure, "test done"); err != nil {
		t.Fatalf("websocket close error: %v", err)
	}

	select {
	case <-sess.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session close after client disconnect")
	}
}

func newWSE2ETestServer(t *testing.T, cfg Config) (*httptest.Server, chan *e2eSession) {
	t.Helper()

	created := make(chan *e2eSession, 8)
	srv := NewServer(cfg)
	srv.SetSessionFactory(func(ctx context.Context, size WindowSize) (Session, error) {
		sess := newE2ESession(ctx, size)
		created <- sess
		return sess, nil
	})

	handler, err := srv.HTTPHandler()
	if err != nil {
		t.Fatalf("HTTPHandler() error = %v", err)
	}

	var ts *httptest.Server
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Skipf("skipping websocket E2E test: loopback listener unavailable: %v", r)
			}
		}()
		ts = httptest.NewServer(handler)
	}()
	if ts == nil {
		t.Fatal("expected test server to be initialized")
	}
	t.Cleanup(ts.Close)
	return ts, created
}

func mustConnectWS(t *testing.T, baseURL string, headers http.Header, size ResizeMessage) (*websocket.Conn, OptionsMessage) {
	t.Helper()

	conn := mustDialWS(t, baseURL, headers)
	writeWS(t, conn, MsgResize, mustJSON(t, size))

	msgType, payload := readWSMessage(t, conn)
	if msgType != MsgOptions {
		t.Fatalf("message type = %q, want %q", msgType, MsgOptions)
	}

	var opts OptionsMessage
	if err := json.Unmarshal(payload, &opts); err != nil {
		t.Fatalf("unmarshal options: %v", err)
	}
	return conn, opts
}

func mustDialWS(t *testing.T, baseURL string, headers http.Header) *websocket.Conn {
	t.Helper()

	conn, resp, err := dialWS(context.Background(), baseURL, headers)
	if err != nil {
		t.Fatalf("websocket dial error: %v (status=%v)", err, statusCode(resp))
	}
	return conn
}

func dialWS(parent context.Context, baseURL string, headers http.Header) (*websocket.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/ws"
	return websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: headers})
}

func writeWS(t *testing.T, conn *websocket.Conn, msgType byte, payload []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, EncodeWSMessage(msgType, payload)); err != nil {
		t.Fatalf("websocket write error: %v", err)
	}
}

func readWSMessage(t *testing.T, conn *websocket.Conn) (byte, []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("websocket read error: %v", err)
	}

	msgType, payload, err := DecodeWSMessage(data)
	if err != nil {
		t.Fatalf("DecodeWSMessage() error = %v", err)
	}
	return msgType, payload
}

func assertConnectionClosed(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	if err == nil {
		t.Fatal("expected connection to close")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed out waiting for connection close: %v", err)
	}
	if errors.Is(err, io.EOF) {
		return
	}
	if websocket.CloseStatus(err) == -1 {
		var closeErr websocket.CloseError
		if !errors.As(err, &closeErr) {
			t.Fatalf("expected websocket close error, got %v", err)
		}
	}
}

func waitForSession(t *testing.T, ch <-chan *e2eSession) *e2eSession {
	t.Helper()

	select {
	case sess := <-ch:
		return sess
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session creation")
		return nil
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func basicAuthHeader(username, password string) string {
	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}

func statusCode(resp *http.Response) any {
	if resp == nil {
		return nil
	}
	return resp.StatusCode
}

type e2eSession struct {
	ctx       context.Context
	done      chan struct{}
	outR      *io.PipeReader
	outW      *io.PipeWriter
	inputCh   chan string
	mu        sync.Mutex
	size      WindowSize
	closeOnce sync.Once
}

func newE2ESession(ctx context.Context, size WindowSize) *e2eSession {
	outR, outW := io.Pipe()
	return &e2eSession{
		ctx:     ctx,
		done:    make(chan struct{}),
		outR:    outR,
		outW:    outW,
		inputCh: make(chan string, 8),
		size:    size,
	}
}

func (s *e2eSession) Context() context.Context { return s.ctx }
func (s *e2eSession) OutputReader() io.Reader  { return s.outR }
func (s *e2eSession) InputWriter() io.Writer   { return writerFunc(s.recordInput) }
func (s *e2eSession) Done() <-chan struct{}    { return s.done }

func (s *e2eSession) Resize(cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size = WindowSize{Width: cols, Height: rows}
}

func (s *e2eSession) WindowSize() WindowSize {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.size
}

func (s *e2eSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		_ = s.outW.Close()
		_ = s.outR.Close()
	})
	return nil
}

func (s *e2eSession) emitOutput(t *testing.T, value string) {
	t.Helper()
	if _, err := io.Copy(s.outW, bytes.NewBufferString(value)); err != nil {
		t.Fatalf("emitOutput() error = %v", err)
	}
}

func (s *e2eSession) waitForInput(t *testing.T) string {
	t.Helper()
	select {
	case value := <-s.inputCh:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session input")
		return ""
	}
}

func (s *e2eSession) assertNoInput(t *testing.T) {
	t.Helper()
	select {
	case value := <-s.inputCh:
		t.Fatalf("unexpected session input: %q", value)
	case <-time.After(200 * time.Millisecond):
	}
}

func (s *e2eSession) recordInput(p []byte) (int, error) {
	s.inputCh <- string(p)
	return len(p), nil
}

type writerFunc func([]byte) (int, error)

func (fn writerFunc) Write(p []byte) (int, error) {
	return fn(p)
}
