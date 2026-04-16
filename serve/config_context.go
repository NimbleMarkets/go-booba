//go:build !js

package serve

import (
	"context"
	"time"
)

const (
	defaultMaxPasteBytes  = 1 << 20 // 1 MiB
	defaultResizeThrottle = 16 * time.Millisecond
	defaultMaxWindowCols  = 4096
	defaultMaxWindowRows  = 4096
)

func pasteMaxOrDefault(v int) int {
	if v <= 0 {
		return defaultMaxPasteBytes
	}
	return v
}

func resizeThrottleOrDefault(v time.Duration) time.Duration {
	if v <= 0 {
		return defaultResizeThrottle
	}
	return v
}

func windowDimsOrDefault(v WindowSize) WindowSize {
	if v.Width <= 0 {
		v.Width = defaultMaxWindowCols
	}
	if v.Height <= 0 {
		v.Height = defaultMaxWindowRows
	}
	return v
}

type configCtxKey struct{}

// withConfig returns a derived context carrying cfg. Used by the framework
// before invoking ConnectMiddleware so middleware can read knobs.
func withConfig(ctx context.Context, cfg Config) context.Context {
	return context.WithValue(ctx, configCtxKey{}, cfg)
}

// ConfigFromContext returns the Config attached to ctx by the framework,
// or the zero value if none is present. Returns Config by value so callers
// cannot mutate the framework's copy.
func ConfigFromContext(ctx context.Context) Config {
	v, _ := ctx.Value(configCtxKey{}).(Config)
	return v
}
