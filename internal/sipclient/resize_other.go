//go:build !unix

package sipclient

import (
	"context"
	"time"
)

// Fallback for non-Unix platforms: poll size every 500ms and fire cb on
// each tick (the cb itself coalesces and compares dimensions). Cheap and
// good enough until a real CONSOLE_SCREEN_BUFFER_SIZE_EVENT watcher is
// worth writing.
func init() {
	watchResize = func(ctx context.Context, cb func()) {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				cb()
			}
		}
	}
}
