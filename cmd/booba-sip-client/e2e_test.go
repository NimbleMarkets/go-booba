//go:build !js

package main_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/go-booba/internal/sipclient"
	"github.com/NimbleMarkets/go-booba/serve"
)

// TestE2E_DumpFramesAgainstRealServer spins up a real serve.Server backed by a
// session factory that writes "hello" then closes, exposes it via httptest, runs
// sipclient.RunDump against it, and asserts an output frame containing "hello"
// appears in the JSON frame stream.
func TestE2E_DumpFramesAgainstRealServer(t *testing.T) {
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

	handler, err := srv.HTTPHandler()
	if err != nil {
		t.Fatalf("HTTPHandler: %v", err)
	}
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	var stdout, stderr bytes.Buffer
	opts := &sipclient.Options{
		URL:            wsURL,
		EscapeCharRaw:  "^]",
		ConnectTimeout: 5 * time.Second,
		DumpTimeout:    3 * time.Second,
		DumpFrames:     true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sipclient.RunDump(ctx, &stdout, &stderr, opts); err != nil {
		t.Fatalf("RunDump: %v (stderr=%s)", err, stderr.String())
	}

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
			s, _ := m["data"].(string)
			decoded, derr := base64.StdEncoding.DecodeString(s)
			if derr != nil {
				t.Fatalf("bad base64: %v", derr)
			}
			if strings.Contains(string(decoded), "hello") {
				sawHello = true
			}
		}
	}
	if !sawHello {
		t.Errorf("no output frame containing 'hello'. stdout:\n%s", stdout.String())
	}
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

// TestE2E_DumpFramesOverWebTransport mirrors TestE2E_DumpFramesAgainstRealServer
// but exercises the full stack over a real QUIC listener using WebTransport.
// It spins up a serve.Server with both a TCP HTTP port and a UDP HTTP/3 port,
// then dials with sipclient.RunDump using an https:// URL.
func TestE2E_DumpFramesOverWebTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("WT e2e requires UDP loopback; skipping in -short mode")
	}

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

	// Grab a free TCP port for the HTTP listener (required by serve.start even
	// when we only care about the WT/UDP port).
	tcpL, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	tcpPort := tcpL.Addr().(*net.TCPAddr).Port
	_ = tcpL.Close()

	// Grab a free UDP port for the HTTP/3 / WebTransport listener.
	udpL, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	udpPort := udpL.LocalAddr().(*net.UDPAddr).Port
	_ = udpL.Close()

	cfg := serve.DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = tcpPort
	cfg.HTTP3Port = udpPort

	srv := serve.NewServer(cfg, serve.WithSessionFactory(factory))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	serveErrCh := make(chan error, 1)
	go func() {
		// nil handler is safe: runSession falls through to a wait-only branch
		// when both handler and cmdName are unset. The session factory handles
		// session creation; output forwarding happens in handleWebTransport.
		serveErrCh <- srv.Serve(ctx, nil)
	}()
	// Give the server time to bind both listeners before dialing.
	time.Sleep(300 * time.Millisecond)

	wtURL := "https://127.0.0.1:" + strconv.Itoa(udpPort) + "/wt"
	var stdout, stderr bytes.Buffer
	opts := &sipclient.Options{
		URL:                wtURL,
		EscapeCharRaw:      "^]",
		InsecureSkipVerify: true,
		ConnectTimeout:     5 * time.Second,
		DumpTimeout:        3 * time.Second,
		DumpFrames:         true,
	}
	// RunDump may return a transport error on some platforms (notably Linux
	// CI under quic-go) when the server's stream close races with the
	// client's final read. That's acceptable as long as the expected
	// output frame already arrived — the test's purpose is to verify
	// wire-format interoperability, not graceful session teardown.
	runErr := sipclient.RunDump(ctx, &stdout, &stderr, opts)

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
			s, _ := m["data"].(string)
			decoded, derr := base64.StdEncoding.DecodeString(s)
			if derr != nil {
				t.Fatalf("bad base64: %v", derr)
			}
			if strings.Contains(string(decoded), "hello") {
				sawHello = true
			}
		}
	}

	if !sawHello && runErr != nil {
		t.Fatalf("RunDump: %v (stderr=%s, stdout=%s)", runErr, stderr.String(), stdout.String())
	}
	if runErr != nil {
		t.Logf("RunDump returned non-nil after observing hello frame (tolerated): %v", runErr)
	}
	if !sawHello {
		t.Errorf("no output frame containing 'hello'. stdout:\n%s", stdout.String())
	}
}
