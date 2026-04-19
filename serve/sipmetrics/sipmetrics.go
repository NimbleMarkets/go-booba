//go:build !js

// Package sipmetrics provides a SessionMiddleware that records
// Prometheus metrics for session lifecycle and byte throughput.
//
// Metrics (prefix defaults to "booba"):
//   - <ns>_sessions_active                (gauge)
//   - <ns>_session_duration_seconds       (histogram)
//   - <ns>_session_bytes_received_total   (counter)
//   - <ns>_session_bytes_sent_total       (counter)
//   - <ns>_session_errors_total           (counter, on Close error)
package sipmetrics

import (
	"errors"
	"io"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/NimbleMarkets/go-booba/serve"
)

type config struct {
	reg       prometheus.Registerer
	namespace string
}

// Option configures the sipmetrics middleware.
type Option func(*config)

// WithRegistry sets the prometheus.Registerer for metrics. Defaults
// to prometheus.DefaultRegisterer.
func WithRegistry(r prometheus.Registerer) Option {
	return func(c *config) { c.reg = r }
}

// WithNamespace sets the metric name prefix. Defaults to "booba".
func WithNamespace(ns string) Option {
	return func(c *config) { c.namespace = ns }
}

type metrics struct {
	active   prometheus.Gauge
	duration prometheus.Histogram
	rx, tx   prometheus.Counter
	errs     prometheus.Counter
}

// New returns a SessionMiddleware that records Prometheus metrics.
func New(opts ...Option) serve.SessionMiddleware {
	cfg := &config{namespace: "booba"}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.reg == nil {
		cfg.reg = prometheus.DefaultRegisterer
	}
	m := &metrics{
		active: registerOrReuse(cfg.reg, prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: cfg.namespace, Name: "sessions_active",
			Help: "Number of sessions currently active.",
		})),
		duration: registerOrReuse(cfg.reg, prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: cfg.namespace, Name: "session_duration_seconds",
			Help: "Session duration in seconds.",
		})),
		rx: registerOrReuse(cfg.reg, prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: cfg.namespace, Name: "session_bytes_received_total",
			Help: "Total bytes received from clients across all sessions.",
		})),
		tx: registerOrReuse(cfg.reg, prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: cfg.namespace, Name: "session_bytes_sent_total",
			Help: "Total bytes sent to clients across all sessions.",
		})),
		errs: registerOrReuse(cfg.reg, prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: cfg.namespace, Name: "session_errors_total",
			Help: "Total session Close errors observed.",
		})),
	}

	return func(base serve.Session) serve.Session {
		m.active.Inc()
		return &metricSession{Session: base, m: m, start: time.Now()}
	}
}

// registerOrReuse attempts to register c. If c collides with an
// already-registered collector of the same fully-qualified name, the
// existing collector is returned instead. This lets callers invoke
// sipmetrics.New multiple times against the same registry + namespace
// without tripping prometheus.MustRegister's panic-on-duplicate.
func registerOrReuse[T prometheus.Collector](reg prometheus.Registerer, c T) T {
	if err := reg.Register(c); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			return are.ExistingCollector.(T)
		}
		panic(err)
	}
	return c
}

type metricSession struct {
	serve.Session
	m     *metrics
	start time.Time
}

func (s *metricSession) InputWriter() io.Writer {
	inner := s.Session.InputWriter()
	return &meteringWriter{inner: inner, c: s.m.rx}
}

func (s *metricSession) OutputReader() io.Reader {
	inner := s.Session.OutputReader()
	return &meteringReader{inner: inner, c: s.m.tx}
}

func (s *metricSession) Close() error {
	err := s.Session.Close()
	s.m.active.Dec()
	s.m.duration.Observe(time.Since(s.start).Seconds())
	if err != nil {
		s.m.errs.Inc()
	}
	return err
}

type meteringWriter struct {
	inner io.Writer
	c     prometheus.Counter
}

func (w *meteringWriter) Write(p []byte) (int, error) {
	n, err := w.inner.Write(p)
	if n > 0 {
		w.c.Add(float64(n))
	}
	return n, err
}

type meteringReader struct {
	inner io.Reader
	c     prometheus.Counter
}

func (r *meteringReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		r.c.Add(float64(n))
	}
	return n, err
}
