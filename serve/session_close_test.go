//go:build !js

package serve

import (
	"context"
	"testing"
	"time"
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

// Close must signal cancellation before waiting for the input reader's owner,
// and must not invalidate the PTY while that owner is still using it.
func TestPTYCloseWaitsForBubbleTea(t *testing.T) {
	sess, err := newPtySession(context.Background(), WindowSize{Width: 80, Height: 24})
	if err != nil {
		t.Fatal(err)
	}
	release, ok := sess.beginBubbleTea()
	if !ok {
		t.Fatal("could not reserve PTY")
	}
	closed := make(chan error, 1)
	go func() { closed <- sess.Close() }()
	select {
	case <-sess.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("not canceled")
	}
	select {
	case err := <-closed:
		t.Fatalf("closed before owner exited: %v", err)
	default:
	}
	// The slave is still valid while its owner drains its input loop.
	if _, _, err := sess.pty.Size(); err != nil {
		t.Fatalf("PTY closed early: %v", err)
	}
	release()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not finish")
	}
	if _, ok := sess.beginBubbleTea(); ok {
		t.Fatal("started on closed PTY")
	}
}
