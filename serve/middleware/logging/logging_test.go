//go:build !js

package logging_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/NimbleMarkets/go-booba/serve"
	"github.com/NimbleMarkets/go-booba/serve/middleware/logging"
)

type fakeSession struct {
	ctx context.Context
}

func (s *fakeSession) Context() context.Context       { return s.ctx }
func (s *fakeSession) OutputReader() io.Reader        { return nil }
func (s *fakeSession) InputWriter() io.Writer         { return nil }
func (s *fakeSession) Resize(int, int)                {}
func (s *fakeSession) WindowSize() serve.WindowSize   { return serve.WindowSize{} }
func (s *fakeSession) Done() <-chan struct{}          { return nil }
func (s *fakeSession) Close() error                   { return nil }

func TestLoggingEmitsStartAndEndWithRemoteAddr(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	mw := logging.New(logging.WithLogger(logger))

	base := func(sess serve.Session) (tea.Model, []tea.ProgramOption) {
		return nil, nil
	}
	wrapped := mw(base)

	ctx := serve.WithRemoteAddr(context.Background(), "198.51.100.4:443")
	_, _ = wrapped(&fakeSession{ctx: ctx})

	out := buf.String()
	if !strings.Contains(out, "session start") {
		t.Errorf("log output = %q; missing 'session start'", out)
	}
	if !strings.Contains(out, "session end") {
		t.Errorf("log output = %q; missing 'session end'", out)
	}
	if !strings.Contains(out, "198.51.100.4:443") {
		t.Errorf("log output = %q; missing remote_addr field", out)
	}
	if !strings.Contains(out, "duration_ms") {
		t.Errorf("log output = %q; missing duration_ms field", out)
	}
}
