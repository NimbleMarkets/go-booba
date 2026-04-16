//go:build !js

package serve

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWSRunsConnectChainAndRespectsRejection(t *testing.T) {
	deny := func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			return &ConnectError{Status: 403, Body: "no"}
		}
	}
	srv := NewServer(DefaultConfig(), WithConnectMiddleware(deny))

	rec := httptest.NewRecorder()
	srv.handleWS(rec, httptest.NewRequest("GET", "/ws", nil))

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != 403 {
		t.Errorf("status = %d; want 403", res.StatusCode)
	}
	if !strings.Contains(rec.Body.String(), "no") {
		t.Errorf("body = %q; want to contain 'no'", rec.Body.String())
	}
}

func TestHandleWSChainSeesConfigInContext(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxPasteBytes = 12345
	var seen int
	probe := func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			seen = ConfigFromContext(r.Context()).MaxPasteBytes
			return &ConnectError{Status: 418} // short-circuit so we don't proceed
		}
	}
	srv := NewServer(cfg, WithConnectMiddleware(probe))

	rec := httptest.NewRecorder()
	srv.handleWS(rec, httptest.NewRequest("GET", "/ws", nil))

	if seen != 12345 {
		t.Errorf("middleware saw MaxPasteBytes = %d; want 12345", seen)
	}
}
