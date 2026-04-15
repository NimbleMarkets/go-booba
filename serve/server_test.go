//go:build !js

package serve

import (
	"net/http/httptest"
	"testing"
)

func TestCheckOriginAllowsSameHost(t *testing.T) {
	srv := NewServer(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/ws", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://example.com")

	if !srv.checkOrigin(req) {
		t.Fatal("expected same-host origin to be allowed")
	}
}

func TestCheckOriginAllowsConfiguredPattern(t *testing.T) {
	srv := NewServer(Config{OriginPatterns: []string{"https://*.example.com"}})
	req := httptest.NewRequest("GET", "http://booba.test/ws", nil)
	req.Host = "booba.test"
	req.Header.Set("Origin", "https://app.example.com")

	if !srv.checkOrigin(req) {
		t.Fatal("expected configured origin pattern to be allowed")
	}
}

func TestCheckOriginRejectsUnexpectedOrigin(t *testing.T) {
	srv := NewServer(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/ws", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://evil.example.net")

	if srv.checkOrigin(req) {
		t.Fatal("expected unexpected origin to be rejected")
	}
}

func TestSameOriginHostIgnoresPort(t *testing.T) {
	if !sameOriginHost("example.com", "example.com:8080") {
		t.Fatal("expected same host with port to match")
	}
}
