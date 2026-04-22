//go:build !js

package main_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
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
