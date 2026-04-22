//go:build unix

package sipclient

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	watchResize = func(ctx context.Context, cb func()) {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		defer signal.Stop(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				cb()
			}
		}
	}
}
