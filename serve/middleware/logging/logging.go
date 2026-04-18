//go:build !js

// Package logging provides a layer-3 Middleware that records session
// start and end events via slog.
package logging

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/NimbleMarkets/go-booba/serve"
)

type config struct {
	logger *slog.Logger
}

// Option configures the logging middleware.
type Option func(*config)

// WithLogger sets the slog.Logger used to record session events. If
// nil or unset, slog.Default() is used.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// New returns a Middleware that logs session start and end around the
// wrapped Handler invocation.
func New(opts ...Option) serve.Middleware {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}
	return func(next serve.Handler) serve.Handler {
		return func(sess serve.Session) (m tea.Model, popts []tea.ProgramOption) {
			addr := serve.RemoteAddrFromContext(sess.Context())
			start := time.Now()
			cfg.logger.Info("session start", slog.String("remote_addr", addr))
			// "session end" fires via defer so panics in next(sess)
			// still produce a closing log entry. The panic itself
			// propagates; logging this middleware is not a recover.
			defer func() {
				cfg.logger.Info("session end",
					slog.String("remote_addr", addr),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				)
			}()
			m, popts = next(sess)
			return
		}
	}
}
