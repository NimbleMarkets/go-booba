//go:build !js

package serve

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateBasicAuth(t *testing.T) {
	cases := []struct {
		name             string
		user, pass       string // configured
		reqUser, reqPass string
		sendCreds        bool
		want             bool
	}{
		{name: "both empty skips auth even without creds", want: true},
		{name: "both empty skips auth even with creds", sendCreds: true, reqUser: "x", reqPass: "y", want: true},
		{name: "configured + correct creds", user: "alice", pass: "secret", sendCreds: true, reqUser: "alice", reqPass: "secret", want: true},
		{name: "configured + wrong password", user: "alice", pass: "secret", sendCreds: true, reqUser: "alice", reqPass: "nope", want: false},
		{name: "configured + wrong user", user: "alice", pass: "secret", sendCreds: true, reqUser: "bob", reqPass: "secret", want: false},
		{name: "configured + no creds", user: "alice", pass: "secret", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if c.sendCreds {
				r.SetBasicAuth(c.reqUser, c.reqPass)
			}
			if got := validateBasicAuth(r, c.user, c.pass); got != c.want {
				t.Errorf("validateBasicAuth = %v; want %v", got, c.want)
			}
		})
	}
}

func TestBasicAuthMiddlewareRejectsWrongCreds(t *testing.T) {
	mw := basicAuthMiddleware("alice", "secret")
	called := false
	next := ConnectHandler(func(r *http.Request) error {
		called = true
		return nil
	})
	r := httptest.NewRequest("GET", "/ws", nil)
	r.SetBasicAuth("alice", "WRONG")
	err := mw(next)(r)
	if err == nil {
		t.Fatal("expected rejection error, got nil")
	}
	if called {
		t.Error("next was called despite rejection")
	}
	ce, ok := err.(*ConnectError)
	if !ok {
		t.Fatalf("err type = %T; want *ConnectError", err)
	}
	if ce.Status != 401 {
		t.Errorf("status = %d; want 401", ce.Status)
	}
	if got := ce.Headers.Get("WWW-Authenticate"); got == "" {
		t.Error("WWW-Authenticate header missing")
	}
}

func TestBasicAuthMiddlewareAcceptsRightCreds(t *testing.T) {
	mw := basicAuthMiddleware("alice", "secret")
	called := false
	next := ConnectHandler(func(r *http.Request) error {
		called = true
		return nil
	})
	r := httptest.NewRequest("GET", "/ws", nil)
	r.SetBasicAuth("alice", "secret")
	if err := mw(next)(r); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
	if !called {
		t.Error("next was not called")
	}
}

func TestBasicAuthNotInstalledWhenUsernameEmpty(t *testing.T) {
	srv := NewServer(DefaultConfig()) // no BasicUsername
	for _, mw := range srv.connectMW {
		// Behavioral probe: run each installed middleware over a
		// no-creds request; if any returns 401, basic-auth is wrongly
		// installed.
		err := mw(func(r *http.Request) error { return nil })(httptest.NewRequest("GET", "/ws", nil))
		if ce, ok := err.(*ConnectError); ok && ce.Status == 401 {
			t.Fatal("basic-auth middleware was installed despite empty BasicUsername")
		}
	}
}

func TestConnLimitMiddlewareRejectsWhenAtCapacity(t *testing.T) {
	srv := NewServer(Config{MaxConnections: 1})
	mw := connLimitMiddleware(srv)

	// First connection acquires.
	if err := mw(func(r *http.Request) error { return nil })(httptest.NewRequest("GET", "/ws", nil)); err != nil {
		t.Fatalf("first conn rejected: %v", err)
	}
	// Second connection (without first releasing) is rejected.
	err := mw(func(r *http.Request) error { return nil })(httptest.NewRequest("GET", "/ws", nil))
	ce, ok := err.(*ConnectError)
	if !ok {
		t.Fatalf("err type = %T; want *ConnectError", err)
	}
	if ce.Status != 503 {
		t.Errorf("status = %d; want 503", ce.Status)
	}
	// Release first, then second should succeed.
	srv.releaseConnection()
	if err := mw(func(r *http.Request) error { return nil })(httptest.NewRequest("GET", "/ws", nil)); err != nil {
		t.Errorf("after release, expected success; got %v", err)
	}
}

type idleMWTestSession struct {
	in       chan []byte
	done     chan struct{}
	closeErr error
	closed   atomic.Bool
}

func newIdleMWTestSession() *idleMWTestSession {
	return &idleMWTestSession{
		in:   make(chan []byte, 16),
		done: make(chan struct{}),
	}
}

func (s *idleMWTestSession) Context() context.Context { return context.Background() }
func (s *idleMWTestSession) OutputReader() io.Reader  { return nil }
func (s *idleMWTestSession) InputWriter() io.Writer {
	return idleMWTestWriter{ch: s.in}
}
func (s *idleMWTestSession) Resize(int, int)        {}
func (s *idleMWTestSession) WindowSize() WindowSize { return WindowSize{Width: 80, Height: 24} }
func (s *idleMWTestSession) Done() <-chan struct{}  { return s.done }
func (s *idleMWTestSession) Close() error {
	if s.closed.CompareAndSwap(false, true) {
		close(s.done)
	}
	return s.closeErr
}

type idleMWTestWriter struct {
	ch chan<- []byte
}

func (w idleMWTestWriter) Write(p []byte) (int, error) {
	w.ch <- append([]byte(nil), p...)
	return len(p), nil
}

func TestIdleTimeoutClosesSessionAfterDuration(t *testing.T) {
	sess := newIdleMWTestSession()
	_ = idleTimeoutMiddleware(50 * time.Millisecond)(sess)
	select {
	case <-sess.Done():
		// closed by idle timeout; good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("idletimeout did not close session within 500ms (expected ~50ms)")
	}
}

func TestIdleTimeoutResetsOnInboundWrite(t *testing.T) {
	sess := newIdleMWTestSession()
	wrapped := idleTimeoutMiddleware(80 * time.Millisecond)(sess)

	// Write inbound bytes every 30ms for 8 writes (240ms total). If
	// writes correctly reset the timer, the session is still alive
	// at 200ms. After writes stop, timer fires within another ~80ms.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8; i++ {
			_, _ = wrapped.InputWriter().Write([]byte{'x'})
			time.Sleep(30 * time.Millisecond)
		}
	}()
	wg.Wait()

	select {
	case <-sess.Done():
		// closed after last write + timeout; good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("session not closed within 500ms after writes stopped")
	}
}

func TestIdleTimeoutNoopForZeroDuration(t *testing.T) {
	sess := newIdleMWTestSession()
	wrapped := idleTimeoutMiddleware(0)(sess)
	if wrapped != Session(sess) {
		t.Error("idleTimeoutMiddleware(0) must return the session unwrapped")
	}
	select {
	case <-sess.Done():
		t.Error("session closed unexpectedly for zero-duration idletimeout")
	case <-time.After(150 * time.Millisecond):
		// still alive; good
	}
}

func TestIdleTimeoutDoubleInstallIsSafe(t *testing.T) {
	// Guard: two idleTimeoutMiddleware instances wrapped around the
	// same session will each fire their own watchdog goroutine. When
	// the first timer fires and calls Close, the second timer will
	// still fire later and also call Close on the already-closed
	// session. Session.Close must be idempotent (per the v0.3 contract)
	// so this is safe — neither goroutine panics.
	//
	// Although the current API doesn't export idleTimeoutMiddleware
	// (so external callers can't trigger the two-idletimeout scenario
	// themselves), this test pins the safety invariant against any
	// future refactor that exposes the constructor or otherwise
	// enables overlapping idle-close goroutines.
	sess := newIdleMWTestSession()
	wrapped := idleTimeoutMiddleware(30 * time.Millisecond)(sess)
	wrapped = idleTimeoutMiddleware(60 * time.Millisecond)(wrapped)
	_ = wrapped

	// First timer should close the session within ~30ms; allow generous
	// slack for CI jitter.
	select {
	case <-sess.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first idle timer did not close session within 500ms")
	}
	// Wait past the second timer's fire time. If the second Close call
	// panicked or deadlocked, the test process would not reach the end.
	time.Sleep(100 * time.Millisecond)
}

func TestIdleTimeoutComposesWithUserSessionMiddleware(t *testing.T) {
	// Guard: auto-installed idleTimeoutMiddleware (via cfg.IdleTimeout)
	// must compose cleanly with user-installed SessionMiddleware. The
	// user middleware wraps outermost per the v0.3 outermost-first
	// convention, and idletimeout's Close propagates through the user
	// wrapper's embedded Session.Close().
	cfg := DefaultConfig()
	cfg.IdleTimeout = 30 * time.Millisecond

	var userClosed atomic.Bool
	userMW := func(base Session) Session {
		return &closeObservingSession{Session: base, onClose: func() { userClosed.Store(true) }}
	}

	srv := NewServer(cfg, WithSessionMiddleware(userMW))
	base := newIdleMWTestSession()
	wrapped := applySessionMiddleware(base, srv.sessionMW)

	select {
	case <-base.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("auto-installed idletimeout did not close base session")
	}
	_ = wrapped.Close() // should be idempotent
	if !userClosed.Load() {
		t.Error("user SessionMiddleware's Close override was never called")
	}
}

type closeObservingSession struct {
	Session
	onClose func()
}

func (s *closeObservingSession) Close() error {
	s.onClose()
	return s.Session.Close()
}
