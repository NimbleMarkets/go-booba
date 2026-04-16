//go:build !js

package serve

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quic-go/webtransport-go"
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

func TestHandleWSChainPassThroughReachesCheckAuth(t *testing.T) {
	// With basic auth configured and no connect middleware, an
	// unauthenticated request must still be rejected by the existing
	// checkAuth call that runs after the (empty) chain. This pins the
	// handoff between the new chain wiring and the pre-existing
	// built-ins so Tasks 14/15 can migrate them without regressing.
	cfg := DefaultConfig()
	cfg.BasicUsername = "alice"
	cfg.BasicPassword = "secret"
	srv := NewServer(cfg)

	rec := httptest.NewRecorder()
	srv.handleWS(rec, httptest.NewRequest("GET", "/ws", nil))

	if rec.Code != 401 {
		t.Errorf("unauth request status = %d; want 401 (checkAuth should still run)", rec.Code)
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

func TestHandleWTRunsConnectChainAndRespectsRejection(t *testing.T) {
	deny := func(next ConnectHandler) ConnectHandler {
		return func(r *http.Request) error {
			return &ConnectError{Status: 403, Body: "no-wt"}
		}
	}
	srv := NewServer(DefaultConfig(), WithConnectMiddleware(deny))

	rec := httptest.NewRecorder()
	// Pass a nil wtServer — the chain rejects before Upgrade is called.
	srv.handleWT(rec, httptest.NewRequest("CONNECT", "/wt", nil), (*webtransport.Server)(nil))

	if rec.Code != 403 {
		t.Errorf("status = %d; want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no-wt") {
		t.Errorf("body = %q; want to contain 'no-wt'", rec.Body.String())
	}
}
