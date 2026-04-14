package serve

import "time"

// Config holds server configuration.
type Config struct {
	Host           string        // Bind address (default "0.0.0.0")
	Port           int           // WebSocket port (default 8080)
	MaxConnections int           // 0 = unlimited
	IdleTimeout    time.Duration // 0 = no timeout
	ReadOnly       bool          // Disable client input
	Debug          bool          // Verbose logging
	TLSCert        string        // Optional TLS cert file path
	TLSKey         string        // Optional TLS key file path
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host: "0.0.0.0",
		Port: 8080,
	}
}
