//go:build !js

package serve

import (
	"context"
	"testing"
)

type stringID string

func (s stringID) String() string { return string(s) }

func TestIdentityFromContextRoundTrip(t *testing.T) {
	ctx := WithIdentity(context.Background(), stringID("alice"))
	id, ok := IdentityFromContext(ctx)
	if !ok {
		t.Fatal("IdentityFromContext returned ok=false")
	}
	if id.String() != "alice" {
		t.Errorf("id.String() = %q; want alice", id.String())
	}
}

func TestIdentityFromContextAbsent(t *testing.T) {
	if _, ok := IdentityFromContext(context.Background()); ok {
		t.Error("IdentityFromContext on empty ctx returned ok=true")
	}
}

func TestWithIdentityNilLeavesContextUnchanged(t *testing.T) {
	parent := context.Background()
	got := WithIdentity(parent, nil)
	if got != parent {
		t.Error("WithIdentity(nil) must return the original context, not a new one")
	}
	if _, ok := IdentityFromContext(got); ok {
		t.Error("WithIdentity(nil) should not store an identity")
	}
}
