//go:build !js

package serve

// Option configures a Server at construction time. Options are applied
// in the order they are passed to NewServer.
type Option func(*Server)
