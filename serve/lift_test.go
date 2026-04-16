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
