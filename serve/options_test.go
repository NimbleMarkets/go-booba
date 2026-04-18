//go:build !js

package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewServerWithNoOptionsIsUnchanged(t *testing.T) {
	cfg := DefaultConfig()
	srv := NewServer(cfg)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	if srv.newSession == nil {
		t.Error("default session factory was not installed")
	}
	if !reflect.DeepEqual(srv.config, cfg) {
		t.Errorf("srv.config = %+v; want %+v", srv.config, cfg)
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

func TestWithConnectMiddlewareAppendsInOrder(t *testing.T) {
	var calls []string
	mk := func(label string) ConnectMiddleware {
		return func(next ConnectHandler) ConnectHandler {
			return func(r *http.Request) error {
				calls = append(calls, label)
				return next(r)
			}
		}
	}
	srv := NewServer(DefaultConfig(),
		WithConnectMiddleware(mk("a"), mk("b")),
		WithConnectMiddleware(mk("c")),
	)
	// The connectMW slice also contains auto-installed built-ins. The
	// behavioral assertion below is the one that matters — it proves the
	// three user middlewares landed and ran in install order. A count
	// check here would be fragile against future built-in additions.
	if _, err := runConnectChain(httptest.NewRequest("GET", "/ws", nil), srv.connectMW); err != nil {
		t.Fatalf("runConnectChain err = %v", err)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("call order = %v; want %v (outermost-first across calls and args)", calls, want)
	}
}

type recordingSession struct {
	Session
	calls *[]string
	tag   string
}

func (r *recordingSession) OutputReader() io.Reader {
	*r.calls = append(*r.calls, r.tag)
	return r.Session.OutputReader()
}

func TestApplySessionMiddlewareNilIsIdentity(t *testing.T) {
	base := &resizeTestSession{}
	if got := applySessionMiddleware(base, nil); got != base {
		t.Errorf("applySessionMiddleware(base, nil) returned a different session")
	}
	if got := applySessionMiddleware(base, []SessionMiddleware{}); got != base {
		t.Errorf("applySessionMiddleware(base, empty slice) returned a different session")
	}
}

func TestWithSessionMiddlewareWrapsOutermostFirst(t *testing.T) {
	var calls []string
	mk := func(tag string) SessionMiddleware {
		return func(s Session) Session {
			return &recordingSession{Session: s, calls: &calls, tag: tag}
		}
	}
	srv := NewServer(DefaultConfig(),
		WithSessionMiddleware(mk("a"), mk("b")),
		WithSessionMiddleware(mk("c")),
	)
	// 3 user-installed + 1 built-in (idleTimeoutMiddleware, no-op for IdleTimeout==0)
	if got := len(srv.sessionMW); got != 4 {
		t.Fatalf("len(sessionMW) = %d; want 4", got)
	}
	// Apply the chain to a fake base session and verify the call order on OutputReader.
	base := &resizeTestSession{} // defined in bubbletea_test.go
	wrapped := applySessionMiddleware(base, srv.sessionMW)
	_ = wrapped.OutputReader()
	want := []string{"a", "b", "c"} // outermost first
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("call order = %v; want %v", calls, want)
	}
}

func TestWithMiddlewareWrapsHandlerOutermostFirst(t *testing.T) {
	var calls []string
	mk := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(sess Session) (tea.Model, []tea.ProgramOption) {
				calls = append(calls, name)
				return next(sess)
			}
		}
	}
	base := func(sess Session) (tea.Model, []tea.ProgramOption) {
		calls = append(calls, "base")
		return nil, nil
	}
	srv := NewServer(DefaultConfig(),
		WithMiddleware(mk("a"), mk("b"), mk("c")),
	)
	wrapped := applyHandlerMiddleware(base, srv.middleware)
	_, _ = wrapped(nil)
	want := []string{"a", "b", "c", "base"} // outermost first, then base
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("call order = %v; want %v", calls, want)
	}
}
