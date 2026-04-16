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
	OriginPatterns []string      // Additional allowed origins for browser clients
	BasicUsername  string        // Optional Basic Auth username
	BasicPassword  string        // Optional Basic Auth password

	// MaxPasteBytes caps the size of a single inbound Sip message
	// (typically a bracketed-paste payload). Zero means default (1 MiB).
	MaxPasteBytes int

	// ResizeThrottle coalesces rapid inbound resize messages into the
	// most recent value. Zero means default (16ms).
	ResizeThrottle time.Duration

	// MaxWindowDims rejects resize messages exceeding these dimensions
	// (initial Resize closes the connection; subsequent resizes are
	// silently dropped). Zero in either Width or Height means default
	// for that dimension (4096 each).
	MaxWindowDims WindowSize
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host: "127.0.0.1",
		Port: 8080,
	}
}
