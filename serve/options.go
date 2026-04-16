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
