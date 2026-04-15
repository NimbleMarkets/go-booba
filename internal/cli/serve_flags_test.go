package cli

import "testing"

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
