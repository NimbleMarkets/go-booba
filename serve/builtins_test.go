//go:build !js

package serve

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
