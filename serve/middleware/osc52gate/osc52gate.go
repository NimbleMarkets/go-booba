//go:build !js

// Package osc52gate provides a SessionMiddleware that filters OSC 52
// clipboard-write escape sequences in the outbound byte stream
// (server → client). Three modes:
//
//   - ModeAllow: pass-through (explicit no-op, documents the policy).
//   - ModeDeny:  strip the escape from the stream before the client sees it.
//   - ModeAudit: pass-through + log attempts via slog.
package osc52gate

import (
	"io"
	"log/slog"

	"github.com/NimbleMarkets/go-booba/serve"
)

// Mode selects how the middleware treats an observed OSC 52 escape.
type Mode int

const (
	// ModeAllow passes the escape through unchanged.
	ModeAllow Mode = iota
	// ModeDeny strips the escape from the output byte stream.
	ModeDeny
	// ModeAudit passes through and logs the attempt.
	ModeAudit
)

type config struct {
	logger *slog.Logger
}

// Option configures the osc52gate middleware.
type Option func(*config)

// WithLogger sets the slog.Logger used by ModeAudit. If nil or unset,
// slog.Default() is used.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// New returns a SessionMiddleware that filters the session's outbound
// byte stream for OSC 52 clipboard-write escapes.
func New(mode Mode, opts ...Option) serve.SessionMiddleware {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}
	var audit auditFn
	if mode == ModeAudit {
		audit = func(sel string, dataLen int) {
			cfg.logger.Info("osc52 clipboard write observed",
				slog.String("selection", sel),
				slog.Int("bytes", dataLen),
			)
		}
	}
	return func(base serve.Session) serve.Session {
		return &gatedSession{Session: base, mode: mode, audit: audit}
	}
}

type gatedSession struct {
	serve.Session
	mode  Mode
	audit auditFn
}

func (g *gatedSession) OutputReader() io.Reader {
	return newScanner(g.Session.OutputReader(), g.mode, g.audit)
}

type auditFn func(selection string, dataLen int)

// newScanner constructs a stateful OSC 52 scanner over inner. The
// audit callback is only invoked in ModeAudit; pass nil otherwise.
func newScanner(inner io.Reader, mode Mode, audit auditFn) io.Reader {
	return &scanner{
		inner: inner,
		mode:  mode,
		audit: audit,
	}
}

type scanState int

const (
	stNormal  scanState = iota
	stEsc               // saw ESC
	stBracket           // saw ESC ]
	stPrefix            // consuming "52"
	stSemi1             // saw ESC ] 52 ;
	stSel               // consuming selection char(s) until ;
	stData              // consuming data until terminator
	stMaybeST           // saw ESC inside data, awaiting backslash
)

type scanner struct {
	inner io.Reader
	mode  Mode
	audit auditFn

	// State-machine fields.
	state    scanState
	buffered []byte // bytes consumed by the state machine that may need to flush
	selBuf   []byte // captured selection
	dataLen  int    // captured data length
	prefixN  int    // 0 or 1 — we have matched '5' of "52"

	// Output buffer for bytes already passed through the state machine
	// and ready for the caller to read.
	outBuf []byte
}

func (s *scanner) Read(p []byte) (int, error) {
	for {
		// Emit any previously-buffered output first.
		if len(s.outBuf) > 0 {
			n := copy(p, s.outBuf)
			s.outBuf = s.outBuf[n:]
			return n, nil
		}
		buf := make([]byte, len(p))
		n, err := s.inner.Read(buf)
		if n > 0 {
			s.feed(buf[:n])
		}
		if len(s.outBuf) == 0 && err != nil {
			// EOF (or error) mid-escape: flush any buffered bytes
			// through as-is so we don't silently swallow data on
			// malformed input.
			if len(s.buffered) > 0 {
				s.outBuf = append(s.outBuf, s.buffered...)
				s.buffered = s.buffered[:0]
				s.state = stNormal
			}
		}
		if len(s.outBuf) > 0 {
			m := copy(p, s.outBuf)
			s.outBuf = s.outBuf[m:]
			if err == io.EOF && len(s.outBuf) == 0 {
				return m, io.EOF
			}
			return m, nil
		}
		if err != nil {
			return 0, err
		}
		// err == nil, outBuf empty: either inner returned (0, nil) or
		// we consumed bytes mid-escape. Loop and read more rather than
		// recurse (unbounded for large escapes or (0, nil) inners).
	}
}

// feed processes all bytes in b, updating state-machine fields and
// appending pass-through bytes to s.outBuf.
func (s *scanner) feed(b []byte) {
	for _, c := range b {
		s.step(c)
	}
}

func (s *scanner) step(c byte) {
	switch s.state {
	case stNormal:
		if c == 0x1b {
			s.buffered = append(s.buffered[:0], c)
			s.state = stEsc
			return
		}
		s.emit(c)
	case stEsc:
		if c == ']' {
			s.buffered = append(s.buffered, c)
			s.state = stBracket
			return
		}
		s.flushBuffered()
		s.emit(c)
		s.state = stNormal
	case stBracket:
		if c == '5' {
			s.buffered = append(s.buffered, c)
			s.prefixN = 1
			s.state = stPrefix
			return
		}
		s.flushBuffered()
		s.emit(c)
		s.state = stNormal
	case stPrefix:
		if s.prefixN == 1 && c == '2' {
			s.buffered = append(s.buffered, c)
			s.state = stSemi1
			return
		}
		s.flushBuffered()
		s.emit(c)
		s.state = stNormal
	case stSemi1:
		if c == ';' {
			s.buffered = append(s.buffered, c)
			s.selBuf = s.selBuf[:0]
			s.state = stSel
			return
		}
		s.flushBuffered()
		s.emit(c)
		s.state = stNormal
	case stSel:
		s.buffered = append(s.buffered, c)
		if c == ';' {
			s.dataLen = 0
			s.state = stData
			return
		}
		s.selBuf = append(s.selBuf, c)
	case stData:
		if c == 0x07 { // BEL terminator
			s.finishEscape(false)
			return
		}
		if c == 0x1b { // possible ST
			s.buffered = append(s.buffered, c)
			s.state = stMaybeST
			return
		}
		s.buffered = append(s.buffered, c)
		s.dataLen++
	case stMaybeST:
		if c == '\\' {
			s.buffered = append(s.buffered, c)
			s.finishEscape(true)
			return
		}
		// Not ST — that ESC was part of data. Stay in data state and
		// continue; the ESC and this byte count as data.
		s.buffered = append(s.buffered, c)
		s.dataLen++
		s.state = stData
	}
}

func (s *scanner) emit(c byte) {
	s.outBuf = append(s.outBuf, c)
}

// flushBuffered is called when the state machine gives up mid-escape
// (malformed input) — the buffered bytes pass through unchanged.
func (s *scanner) flushBuffered() {
	s.outBuf = append(s.outBuf, s.buffered...)
	s.buffered = s.buffered[:0]
}

// finishEscape is called when a complete OSC 52 sequence is observed.
// stTerminated is true when the sequence ended with ESC\ (ST) — in
// that case buffered already contains the terminator bytes. When
// false, the BEL terminator was consumed by step() without being
// buffered, and the Allow/Audit branches must append it explicitly.
func (s *scanner) finishEscape(stTerminated bool) {
	switch s.mode {
	case ModeAllow, ModeAudit:
		if s.mode == ModeAudit && s.audit != nil {
			s.audit(string(s.selBuf), s.dataLen)
		}
		s.outBuf = append(s.outBuf, s.buffered...)
		if !stTerminated {
			s.outBuf = append(s.outBuf, 0x07)
		}
	case ModeDeny:
		// drop everything buffered — nothing added to outBuf
	}
	s.buffered = s.buffered[:0]
	s.selBuf = s.selBuf[:0]
	s.dataLen = 0
	s.state = stNormal
}
