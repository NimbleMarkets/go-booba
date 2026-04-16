//go:build !js

package serve

import "time"

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
