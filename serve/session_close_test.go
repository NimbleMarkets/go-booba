//go:build !js

package serve

import (
	"context"
	"testing"
)

func TestPTYSessionCloseIdempotent(t *testing.T) {
	sess, err := defaultSessionFactory(context.Background(), WindowSize{Width: 80, Height: 24})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("second Close: %v; want nil (idempotent)", err)
	}
}
