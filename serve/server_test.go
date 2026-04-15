//go:build !js

package serve

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestHTTPHandlerRejectsUnsafeConfig(t *testing.T) {
	srv := NewServer(Config{Host: "0.0.0.0"})
	handler, err := srv.HTTPHandler()
	if err == nil {
		t.Fatalf("HTTPHandler() error = nil, handler = %v; want unsafe config rejection", handler)
	}
	if !strings.Contains(err.Error(), "non-loopback listeners require TLS") {
		t.Fatalf("HTTPHandler() error = %v, want non-loopback TLS rejection", err)
	}
}

func TestDefaultConfigUsesLoopback(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("default host = %q, want %q", cfg.Host, "127.0.0.1")
	}
}

func TestValidateConfigRejectsPartialTLSConfig(t *testing.T) {
	srv := NewServer(Config{CertFile: "server.crt"})
	err := srv.validateConfig()
	if err == nil || !strings.Contains(err.Error(), "provided together") {
		t.Fatalf("validateConfig() error = %v, want partial TLS config rejection", err)
	}
}

func TestValidateConfigRejectsBasicAuthWithoutTLS(t *testing.T) {
	srv := NewServer(Config{
		Host:          "127.0.0.1",
		BasicUsername: "admin",
		BasicPassword: "secret",
	})
	err := srv.validateConfig()
	if err == nil || !strings.Contains(err.Error(), "Basic Auth requires TLS") {
		t.Fatalf("validateConfig() error = %v, want Basic Auth TLS rejection", err)
	}
}

func TestValidateConfigRejectsRemotePlaintextListener(t *testing.T) {
	srv := NewServer(Config{Host: "0.0.0.0"})
	err := srv.validateConfig()
	if err == nil || !strings.Contains(err.Error(), "non-loopback listeners require TLS") {
		t.Fatalf("validateConfig() error = %v, want remote plaintext rejection", err)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"":               true,
		"localhost":      true,
		"127.0.0.1":      true,
		"127.0.0.1:8080": true,
		"::1":            true,
		"0.0.0.0":        false,
		"192.168.1.10":   false,
	}

	for host, want := range cases {
		if got := isLoopbackHost(host); got != want {
			t.Fatalf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
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

func TestTryAcquireConnectionHonorsLimit(t *testing.T) {
	srv := NewServer(Config{MaxConnections: 2})

	if !srv.tryAcquireConnection() {
		t.Fatal("expected first acquire to succeed")
	}
	if !srv.tryAcquireConnection() {
		t.Fatal("expected second acquire to succeed")
	}
	if srv.tryAcquireConnection() {
		t.Fatal("expected third acquire to fail")
	}

	srv.releaseConnection()
	if !srv.tryAcquireConnection() {
		t.Fatal("expected acquire after release to succeed")
	}
}

func TestTryAcquireConnectionIsAtomic(t *testing.T) {
	srv := NewServer(Config{MaxConnections: 1})

	const goroutines = 16
	results := make(chan bool, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- srv.tryAcquireConnection()
		}()
	}

	wg.Wait()
	close(results)

	successes := 0
	for ok := range results {
		if ok {
			successes++
		}
	}

	if successes != 1 {
		t.Fatalf("successful acquires = %d, want 1", successes)
	}
}

func TestHTTPSHelpersRespectCertFiles(t *testing.T) {
	srv := NewServer(DefaultConfig())
	if srv.mainTLSEnabled() {
		t.Fatal("expected TLS to be disabled without cert files")
	}
	if got := srv.httpScheme(); got != "http" {
		t.Fatalf("httpScheme() = %q, want %q", got, "http")
	}

	srv = NewServer(Config{CertFile: "server.crt", KeyFile: "server.key"})
	if !srv.mainTLSEnabled() {
		t.Fatal("expected TLS to be enabled with cert files")
	}
	if got := srv.httpScheme(); got != "https" {
		t.Fatalf("httpScheme() = %q, want %q", got, "https")
	}
}

func TestTLSConfigsUseExpectedProtocols(t *testing.T) {
	info, err := GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error = %v", err)
	}

	srv := NewServer(Config{CertFile: "server.crt", KeyFile: "server.key"})
	srv.certInfo = info

	httpsCfg := srv.httpsTLSConfig()
	if httpsCfg == nil {
		t.Fatal("expected HTTPS TLS config")
	}
	if got, want := strings.Join(httpsCfg.NextProtos, ","), "h2,http/1.1"; got != want {
		t.Fatalf("HTTPS NextProtos = %q, want %q", got, want)
	}
	if httpsCfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("HTTPS MinVersion = %v, want %v", httpsCfg.MinVersion, tls.VersionTLS12)
	}

	http3Cfg := srv.http3TLSConfig()
	if http3Cfg == nil {
		t.Fatal("expected HTTP/3 TLS config")
	}
	if got, want := strings.Join(http3Cfg.NextProtos, ","), "h3"; got != want {
		t.Fatalf("HTTP/3 NextProtos = %q, want %q", got, want)
	}
	if http3Cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("HTTP/3 MinVersion = %v, want %v", http3Cfg.MinVersion, tls.VersionTLS13)
	}
}

func TestNewWebTransportServerDefaultsToSamePort(t *testing.T) {
	info, err := GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error = %v", err)
	}

	srv := NewServer(Config{Host: "127.0.0.1", Port: 8080})
	srv.certInfo = info

	wt := srv.newWebTransportServer()
	if wt == nil {
		t.Fatal("expected WebTransport server to be created")
	}
	if got := wt.H3.Addr; got != "127.0.0.1:8080" {
		t.Fatalf("H3.Addr = %q, want %q", got, "127.0.0.1:8080")
	}
}

func TestConfigureTransportDisablesSelfSignedCertForRemoteHost(t *testing.T) {
	srv := NewServer(Config{Host: "0.0.0.0"})
	if err := srv.configureTransport(); err != nil {
		t.Fatalf("configureTransport() error = %v", err)
	}
	if srv.certInfo != nil {
		t.Fatal("expected remote plaintext config to avoid self-signed WebTransport bootstrap")
	}
}

func TestAttachIdleTimeoutClosesIdleSession(t *testing.T) {
	srv := NewServer(Config{IdleTimeout: 50 * time.Millisecond})
	sess := newIdleTestSession()

	ctx, cancel, activity := srv.attachIdleTimeout(context.Background(), sess)
	defer cancel()

	activity()
	time.Sleep(25 * time.Millisecond)
	activity()

	select {
	case <-sess.Done():
		t.Fatal("session closed before idle timeout elapsed")
	case <-time.After(20 * time.Millisecond):
	}

	select {
	case <-sess.Done():
	case <-time.After(120 * time.Millisecond):
		t.Fatal("timed out waiting for idle session close")
	}

	if err := ctx.Err(); err == nil {
		t.Fatal("expected idle timeout context to be canceled")
	}
}

func TestDebugfHonorsDebugFlag(t *testing.T) {
	var buf bytes.Buffer
	origWriter := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(origWriter)
	defer log.SetFlags(origFlags)

	NewServer(DefaultConfig()).debugf("hidden %d", 1)
	if got := buf.String(); got != "" {
		t.Fatalf("unexpected log output with debug disabled: %q", got)
	}

	srv := NewServer(Config{Debug: true})
	srv.debugf("visible %d", 2)
	if got := buf.String(); !strings.Contains(got, "visible 2") {
		t.Fatalf("log output = %q, want debug message", got)
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

type idleTestSession struct {
	done      chan struct{}
	closeOnce sync.Once
}

func newIdleTestSession() *idleTestSession {
	return &idleTestSession{done: make(chan struct{})}
}

func (s *idleTestSession) Context() context.Context { return context.Background() }
func (s *idleTestSession) OutputReader() io.Reader  { return bytes.NewReader(nil) }
func (s *idleTestSession) InputWriter() io.Writer   { return io.Discard }
func (s *idleTestSession) Resize(cols, rows int)    {}
func (s *idleTestSession) WindowSize() WindowSize   { return WindowSize{} }
func (s *idleTestSession) Done() <-chan struct{}    { return s.done }
func (s *idleTestSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
	})
	return nil
}
