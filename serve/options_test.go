//go:build !js

package serve

import "testing"

func TestNewServerWithNoOptionsIsUnchanged(t *testing.T) {
	cfg := DefaultConfig()
	srv := NewServer(cfg)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	if srv.newSession == nil {
		t.Error("default session factory was not installed")
	}
}

func TestNewServerOptionsApplyInOrder(t *testing.T) {
	cfg := DefaultConfig()
	var calls []string
	mark := func(name string) Option {
		return func(s *Server) { calls = append(calls, name) }
	}
	_ = NewServer(cfg, mark("a"), mark("b"), mark("c"))
	want := []string{"a", "b", "c"}
	if len(calls) != len(want) {
		t.Fatalf("len(calls) = %d; want %d (calls = %v)", len(calls), len(want), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("calls[%d] = %q; want %q", i, calls[i], want[i])
		}
	}
}
