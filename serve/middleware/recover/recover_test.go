//go:build !js

package recover_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/NimbleMarkets/go-booba/serve"
	"github.com/NimbleMarkets/go-booba/serve/middleware/recover"
)

type fakeSession struct{ serve.Session }

func TestRecoverCatchesPanicAndReturnsQuittingModel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mw := recover.New(recover.WithLogger(logger))

	panicking := func(sess serve.Session) (tea.Model, []tea.ProgramOption) {
		panic("boom")
	}
	wrapped := mw(panicking)

	model, opts := wrapped(&fakeSession{})
	if model == nil {
		t.Fatal("wrapped handler returned nil model after panic")
	}
	if len(opts) != 0 {
		t.Errorf("opts = %v; want empty after panic", opts)
	}
	// Panic model must Init → tea.Quit.
	if cmd := model.Init(); cmd == nil {
		t.Error("panic model Init() returned nil; expected tea.Quit")
	}
	// Log was produced at Error level and mentions the panic payload.
	logged := buf.String()
	if !strings.Contains(logged, "handler panicked") {
		t.Errorf("log output = %q; want it to contain 'handler panicked'", logged)
	}
	if !strings.Contains(logged, "boom") {
		t.Errorf("log output = %q; want it to contain the panic payload 'boom'", logged)
	}
}

func TestRecoverPassesThroughOnNoPanic(t *testing.T) {
	mw := recover.New()
	var zero tea.Model
	called := false
	base := func(sess serve.Session) (tea.Model, []tea.ProgramOption) {
		called = true
		return zero, []tea.ProgramOption{}
	}
	wrapped := mw(base)
	_, _ = wrapped(&fakeSession{})
	if !called {
		t.Error("wrapped handler did not invoke the base handler")
	}
}
