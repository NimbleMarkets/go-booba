//go:build !js

package serve

// Option is a functional option for [NewServer]. See NewServer for how
// options are sequenced and how additive vs. last-wins options compose.
type Option func(*Server)

// WithSessionFactory replaces the default SessionFactory. Multiple
// applications are last-wins. Passing nil restores the default.
func WithSessionFactory(f SessionFactory) Option {
	return func(s *Server) {
		if f == nil {
			s.newSession = defaultSessionFactory
			return
		}
		s.newSession = f
	}
}

// WithConnectMiddleware appends ConnectMiddleware to the layer-1 chain.
// Multiple calls append; within a single call the order is preserved.
// Built-in basic auth and connection-limit middleware are appended after
// the user chain so they run innermost (last).
func WithConnectMiddleware(mw ...ConnectMiddleware) Option {
	return func(s *Server) {
		s.connectMW = append(s.connectMW, mw...)
	}
}
