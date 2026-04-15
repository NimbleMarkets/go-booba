//go:build !js

package serve

import (
	"bytes"
	"context"
	"io"
	"net/http"
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

func TestHTTPHandlerServesIndexWithoutListener(t *testing.T) {
	srv := NewServer(DefaultConfig())
	handler, err := srv.HTTPHandler()
	if err != nil {
		t.Fatalf("HTTPHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got == "" {
		t.Fatal("expected content type to be set")
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected index body to be non-empty")
	}
}

func TestSetSessionFactoryOverridesSessionCreation(t *testing.T) {
	srv := NewServer(DefaultConfig())
	want := &stubSession{
		ctx:  context.Background(),
		done: make(chan struct{}),
	}
	srv.SetSessionFactory(func(ctx context.Context, size WindowSize) (Session, error) {
		return want, nil
	})

	got, err := srv.createSession(context.Background(), WindowSize{Width: 80, Height: 24})
	if err != nil {
		t.Fatalf("createSession() error = %v", err)
	}
	if got != want {
		t.Fatal("expected injected session factory to be used")
	}
}

type stubSession struct {
	ctx  context.Context
	done chan struct{}
	buf  bytes.Buffer
	size WindowSize
}

func (s *stubSession) Context() context.Context { return s.ctx }
func (s *stubSession) OutputReader() io.Reader  { return &s.buf }
func (s *stubSession) InputWriter() io.Writer   { return &s.buf }
func (s *stubSession) Resize(cols, rows int)    { s.size = WindowSize{Width: cols, Height: rows} }
func (s *stubSession) WindowSize() WindowSize   { return s.size }
func (s *stubSession) Done() <-chan struct{}    { return s.done }
func (s *stubSession) Close() error             { return nil }
