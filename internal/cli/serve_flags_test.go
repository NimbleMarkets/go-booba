package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServeOptionsConfigDefaults(t *testing.T) {
	cfg, err := (ServeOptions{}).Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want %d", cfg.Port, 8080)
	}
}

func TestServeOptionsConfigParsesListenAndOrigins(t *testing.T) {
	opts := ServeOptions{
		Listen:    "127.0.0.1:9999",
		HTTP3Port: -1,
		Origins:   "https://app.example.com, https://*.example.net",
		Username:  "admin",
		Password:  "secret",
	}

	cfg, err := opts.Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 9999 {
		t.Fatalf("listen parsed to %s:%d, want 127.0.0.1:9999", cfg.Host, cfg.Port)
	}
	if cfg.HTTP3Port != -1 {
		t.Fatalf("HTTP3Port = %d, want -1", cfg.HTTP3Port)
	}
	if len(cfg.OriginPatterns) != 2 {
		t.Fatalf("OriginPatterns len = %d, want 2", len(cfg.OriginPatterns))
	}
	if cfg.BasicUsername != "admin" || cfg.BasicPassword != "secret" {
		t.Fatal("expected Basic Auth fields to be copied")
	}
}

func TestServeOptionsConfigRejectsBadListen(t *testing.T) {
	_, err := (ServeOptions{Listen: "bad"}).Config()
	if err == nil {
		t.Fatal("expected invalid listen address to fail")
	}
}

// SEC-1: password can be supplied via --password, --password-file, or
// $BOOBA_PASSWORD. Precedence is flag > file > env so operators can
// override a baked-in default without editing the command line.

func TestServeOptionsPasswordFromEnv(t *testing.T) {
	t.Setenv("BOOBA_PASSWORD", "env-secret")
	cfg, err := (ServeOptions{Username: "admin"}).Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.BasicPassword != "env-secret" {
		t.Errorf("BasicPassword = %q; want %q (from $BOOBA_PASSWORD)", cfg.BasicPassword, "env-secret")
	}
}

func TestServeOptionsPasswordFromFile(t *testing.T) {
	t.Setenv("BOOBA_PASSWORD", "")
	path := filepath.Join(t.TempDir(), "pass")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatalf("seed password file: %v", err)
	}
	cfg, err := (ServeOptions{Username: "admin", PasswordFile: path}).Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.BasicPassword != "file-secret" {
		t.Errorf("BasicPassword = %q; want %q (trimmed from file)", cfg.BasicPassword, "file-secret")
	}
}

func TestServeOptionsPasswordFlagBeatsFileAndEnv(t *testing.T) {
	t.Setenv("BOOBA_PASSWORD", "env-secret")
	path := filepath.Join(t.TempDir(), "pass")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatalf("seed password file: %v", err)
	}
	cfg, err := (ServeOptions{Username: "admin", Password: "flag-secret", PasswordFile: path}).Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.BasicPassword != "flag-secret" {
		t.Errorf("BasicPassword = %q; want %q (flag must win)", cfg.BasicPassword, "flag-secret")
	}
}

func TestServeOptionsPasswordFileBeatsEnv(t *testing.T) {
	t.Setenv("BOOBA_PASSWORD", "env-secret")
	path := filepath.Join(t.TempDir(), "pass")
	if err := os.WriteFile(path, []byte("file-secret"), 0o600); err != nil {
		t.Fatalf("seed password file: %v", err)
	}
	cfg, err := (ServeOptions{Username: "admin", PasswordFile: path}).Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.BasicPassword != "file-secret" {
		t.Errorf("BasicPassword = %q; want %q (file must beat env)", cfg.BasicPassword, "file-secret")
	}
}

func TestServeOptionsPasswordFileMissingFails(t *testing.T) {
	_, err := (ServeOptions{PasswordFile: "/does/not/exist/booba-pass"}).Config()
	if err == nil {
		t.Fatal("expected missing --password-file to fail")
	}
}
