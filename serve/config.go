package serve

import "time"

// Config holds server configuration.
type Config struct {
	Host           string        // Bind address (default "127.0.0.1")
	Port           int           // HTTP/WebSocket port (default 8080)
	HTTP3Port      int           // HTTP/3 WebTransport port (default Port, -1 = disabled)
	MaxConnections int           // 0 = unlimited
	IdleTimeout    time.Duration // 0 = no timeout
	ReadOnly       bool          // Disable client input
	Debug          bool          // Verbose logging
	CertFile       string        // Optional TLS cert file path for HTTPS/WSS/WebTransport
	KeyFile        string        // Optional TLS key file path for HTTPS/WSS/WebTransport
	// OriginPatterns is an optional allowlist of additional browser
	// origins that may connect. Same-host requests (Origin host == Host
	// header) are always permitted — patterns only extend the allowlist.
	//
	// Each entry is a path.Match shell glob, NOT a regex:
	//   *      matches any run of non-'/' bytes
	//   ?      matches one non-'/' byte
	//   [abc]  / [a-z]  character class
	//   \x     escapes the meta-character x
	//
	// Patterns are tested against both the full "scheme://host" form and
	// the bare host, so "*.example.com" and "https://*.example.com" both
	// match https://app.example.com.
	//
	// Examples: "https://app.example.com", "*.example.com",
	// "https://*.internal".
	OriginPatterns []string
	BasicUsername  string // Optional Basic Auth username
	BasicPassword  string // Optional Basic Auth password

	// MaxPasteBytes caps the size of a single inbound Sip message
	// (typically a bracketed-paste payload). Zero or negative means
	// default (1 MiB).
	MaxPasteBytes int

	// ResizeThrottle coalesces rapid inbound resize messages into the
	// most recent value. Zero or negative means default (16ms).
	ResizeThrottle time.Duration

	// MaxWindowDims rejects resize messages exceeding these dimensions
	// (initial Resize closes the connection; subsequent resizes are
	// silently dropped). Zero or negative in either Width or Height
	// means default for that dimension (4096 each).
	MaxWindowDims WindowSize

	// InitialResizeTimeout is the maximum time the server will wait
	// for the client's initial Resize message after a WS upgrade or
	// WT CONNECT. Zero or negative means default (10 seconds).
	InitialResizeTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host: "127.0.0.1",
		Port: 8080,
	}
}
