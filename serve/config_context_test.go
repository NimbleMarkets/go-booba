//go:build !js

package serve

import (
	"context"
	"testing"
	"time"
)

func TestPasteMaxOrDefault(t *testing.T) {
	if got := pasteMaxOrDefault(0); got != 1<<20 {
		t.Errorf("pasteMaxOrDefault(0) = %d; want %d", got, 1<<20)
	}
	if got := pasteMaxOrDefault(4096); got != 4096 {
		t.Errorf("pasteMaxOrDefault(4096) = %d; want 4096", got)
	}
}

func TestResizeThrottleOrDefault(t *testing.T) {
	if got := resizeThrottleOrDefault(0); got != 16*time.Millisecond {
		t.Errorf("resizeThrottleOrDefault(0) = %v; want 16ms", got)
	}
	if got := resizeThrottleOrDefault(50 * time.Millisecond); got != 50*time.Millisecond {
		t.Errorf("resizeThrottleOrDefault(50ms) = %v; want 50ms", got)
	}
}

func TestWindowDimsOrDefault(t *testing.T) {
	got := windowDimsOrDefault(WindowSize{})
	want := WindowSize{Width: 4096, Height: 4096}
	if got != want {
		t.Errorf("windowDimsOrDefault(zero) = %+v; want %+v", got, want)
	}
	got = windowDimsOrDefault(WindowSize{Width: 200, Height: 50})
	if got != (WindowSize{Width: 200, Height: 50}) {
		t.Errorf("windowDimsOrDefault(200x50) = %+v; want unchanged", got)
	}
	// Half-zero falls back to default for that dimension.
	got = windowDimsOrDefault(WindowSize{Width: 200})
	if got.Width != 200 || got.Height != 4096 {
		t.Errorf("windowDimsOrDefault({200,0}) = %+v; want {200,4096}", got)
	}
}

func TestConfigFromContextRoundTrip(t *testing.T) {
	cfg := Config{Host: "h", Port: 9999, MaxPasteBytes: 4096}
	ctx := withConfig(context.Background(), cfg)
	got := ConfigFromContext(ctx)
	if got.Host != "h" || got.Port != 9999 || got.MaxPasteBytes != 4096 {
		t.Errorf("ConfigFromContext = %+v; want host=h port=9999 paste=4096", got)
	}
}

func TestConfigFromContextZeroValueWhenAbsent(t *testing.T) {
	got := ConfigFromContext(context.Background())
	if got.Host != "" || got.Port != 0 {
		t.Errorf("ConfigFromContext(empty ctx) = %+v; want zero value", got)
	}
}

func TestConfigFromContextPropagatesThroughDerivedContext(t *testing.T) {
	cfg := Config{Port: 1234}
	ctx := withConfig(context.Background(), cfg)
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	if got := ConfigFromContext(child); got.Port != 1234 {
		t.Errorf("ConfigFromContext through derived ctx = %+v; want Port=1234", got)
	}
}
