//go:build !js

package serve

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiftHTTPMiddlewarePassthrough(t *testing.T) {
	httpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Lifted", "yes")
			next.ServeHTTP(w, r)
		})
	}
	called := false
	terminal := ConnectHandler(func(r *http.Request) error {
		called = true
		return nil
	})
	mw := LiftHTTPMiddleware(httpMW)
	r := httptest.NewRequest("GET", "/ws", nil)
	rec := httptest.NewRecorder()
	err := runLiftedChain(rec, r, []ConnectMiddleware{mw}, terminal)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Error("terminal handler was not called")
	}
}

func TestLiftHTTPMiddlewareNeitherNextNorWriteIs500(t *testing.T) {
	// A misbehaving middleware that returns without calling next and
	// without writing — the adapter must not swallow this silently.
	httpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// no next, no write
		})
	}
	called := false
	terminal := ConnectHandler(func(r *http.Request) error {
		called = true
		return nil
	})
	err := runLiftedChain(httptest.NewRecorder(), httptest.NewRequest("GET", "/ws", nil),
		[]ConnectMiddleware{LiftHTTPMiddleware(httpMW)}, terminal)
	ce, ok := err.(*ConnectError)
	if !ok {
		t.Fatalf("err type = %T; want *ConnectError", err)
	}
	if ce.Status != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500", ce.Status)
	}
	if called {
		t.Error("terminal must not be called when middleware skips next")
	}
}

func TestLiftHTTPMiddlewareWithoutBridgeFallsThroughToNext(t *testing.T) {
	// When LiftHTTPMiddleware is used outside runLiftedChain (no bridge
	// in context), it degrades by calling next directly. This protects
	// callers who install Lift but accidentally run the chain via the
	// simple runConnectChain path.
	httpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("lifted middleware should not run when bridge is absent")
		})
	}
	called := false
	terminal := ConnectHandler(func(r *http.Request) error {
		called = true
		return nil
	})
	mw := LiftHTTPMiddleware(httpMW)
	if err := mw(terminal)(httptest.NewRequest("GET", "/ws", nil)); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
	if !called {
		t.Error("next was not invoked in the no-bridge fallback path")
	}
}

func TestLiftHTTPMiddlewareWritesResponseAndSkipsNext(t *testing.T) {
	httpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "denied", http.StatusUnauthorized)
		})
	}
	called := false
	terminal := ConnectHandler(func(r *http.Request) error {
		called = true
		return nil
	})
	mw := LiftHTTPMiddleware(httpMW)

	r := httptest.NewRequest("GET", "/ws", nil)
	rec := httptest.NewRecorder()

	err := runLiftedChain(rec, r, []ConnectMiddleware{mw}, terminal)
	if !errors.Is(err, errResponseWritten) {
		t.Errorf("err = %v; want errResponseWritten", err)
	}
	if called {
		t.Error("terminal handler was called despite middleware writing a response")
	}
	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("status = %d; want 401", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "denied") {
		t.Errorf("body = %q; want to contain 'denied'", string(body))
	}
}
