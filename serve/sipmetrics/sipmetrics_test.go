//go:build !js

package sipmetrics_test

import (
	"context"
	"io"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/NimbleMarkets/go-booba/serve"
	"github.com/NimbleMarkets/go-booba/serve/sipmetrics"
)

type fakeSession struct {
	closed chan struct{}
	out    io.Reader
	in     io.Writer
}

func (s *fakeSession) Context() context.Context     { return context.Background() }
func (s *fakeSession) OutputReader() io.Reader      { return s.out }
func (s *fakeSession) InputWriter() io.Writer       { return s.in }
func (s *fakeSession) Resize(int, int)              {}
func (s *fakeSession) WindowSize() serve.WindowSize { return serve.WindowSize{} }
func (s *fakeSession) Done() <-chan struct{}        { return s.closed }
func (s *fakeSession) Close() error                 { close(s.closed); return nil }

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestSessionsActiveGaugeTracksLifecycle(t *testing.T) {
	reg := prometheus.NewRegistry()
	mw := sipmetrics.New(sipmetrics.WithRegistry(reg))
	base := &fakeSession{closed: make(chan struct{}), in: discardWriter{}}
	wrapped := mw(base)

	if got := gaugeValue(t, reg, "booba_sessions_active"); got != 1 {
		t.Errorf("sessions_active before close = %v; want 1", got)
	}
	_ = wrapped.Close()
	if got := gaugeValue(t, reg, "booba_sessions_active"); got != 0 {
		t.Errorf("sessions_active after close = %v; want 0", got)
	}
}

func TestNewIsSafeToCallTwiceOnSameRegistry(t *testing.T) {
	// Guard: calling sipmetrics.New twice against the same registry
	// and namespace must not panic. The second call should reuse the
	// already-registered collectors so both middleware instances share
	// the same underlying metrics.
	reg := prometheus.NewRegistry()

	mw1 := sipmetrics.New(sipmetrics.WithRegistry(reg))
	mw2 := sipmetrics.New(sipmetrics.WithRegistry(reg)) // must not panic

	sess1 := mw1(&fakeSession{closed: make(chan struct{}), in: discardWriter{}})
	sess2 := mw2(&fakeSession{closed: make(chan struct{}), in: discardWriter{}})
	if got := gaugeValue(t, reg, "booba_sessions_active"); got != 2 {
		t.Errorf("sessions_active after wrapping two sessions = %v; want 2", got)
	}
	_ = sess1.Close()
	_ = sess2.Close()
	if got := gaugeValue(t, reg, "booba_sessions_active"); got != 0 {
		t.Errorf("sessions_active after closing both = %v; want 0", got)
	}
}

func TestBytesCountersIncrementOnReadAndWrite(t *testing.T) {
	reg := prometheus.NewRegistry()
	mw := sipmetrics.New(sipmetrics.WithRegistry(reg))
	base := &fakeSession{
		closed: make(chan struct{}),
		out:    readerBytes("hello"), // 5 bytes
		in:     discardWriter{},
	}
	wrapped := mw(base)

	_, _ = wrapped.InputWriter().Write([]byte("hi"))
	buf := make([]byte, 16)
	_, _ = wrapped.OutputReader().Read(buf)
	_ = wrapped.Close()

	if got := counterValue(t, reg, "booba_session_bytes_received_total"); got != 2 {
		t.Errorf("bytes_received = %v; want 2", got)
	}
	if got := counterValue(t, reg, "booba_session_bytes_sent_total"); got != 5 {
		t.Errorf("bytes_sent = %v; want 5", got)
	}
}

// ---- test helpers ----

func gaugeValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if g := m.GetGauge(); g != nil {
				return g.GetValue()
			}
		}
	}
	t.Fatalf("gauge %s not found", name)
	return 0
}

func counterValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if c := m.GetCounter(); c != nil {
				return c.GetValue()
			}
		}
	}
	t.Fatalf("counter %s not found", name)
	return 0
}

type readerBytes string

func (r readerBytes) Read(p []byte) (int, error) {
	if len(r) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r)
	return n, io.EOF
}
