//go:build !js

package serve

import (
	"context"
	"testing"
)

func TestRemoteAddrFromContextRoundTrip(t *testing.T) {
	ctx := WithRemoteAddr(context.Background(), "203.0.113.7:54321")
	if got := RemoteAddrFromContext(ctx); got != "203.0.113.7:54321" {
		t.Errorf("RemoteAddrFromContext = %q; want 203.0.113.7:54321", got)
	}
}

func TestRemoteAddrFromContextAbsentReturnsEmpty(t *testing.T) {
	if got := RemoteAddrFromContext(context.Background()); got != "" {
		t.Errorf("RemoteAddrFromContext(empty) = %q; want empty", got)
	}
}

func TestWithRemoteAddrEmptyLeavesContextUnchanged(t *testing.T) {
	parent := context.Background()
	got := WithRemoteAddr(parent, "")
	if got != parent {
		t.Error("WithRemoteAddr(\"\") must return the original context unchanged")
	}
}
