package sipclient

// SOLTracker tracks whether the next byte to be emitted is at the start of a
// line. A line break is either CR (\r) or LF (\n). The tracker is updated as
// bytes are observed (typically the bytes being forwarded to the server).
type SOLTracker struct {
	atStart bool
}

// NewSOLTracker returns a tracker initialized to AtStart=true, since a fresh
// connection begins on a new line.
func NewSOLTracker() *SOLTracker {
	return &SOLTracker{atStart: true}
}

// AtStart reports whether the next observed byte will be at start-of-line.
func (t *SOLTracker) AtStart() bool { return t.atStart }

// Observe updates the tracker with the given bytes. If the slice is empty, the
// state is unchanged.
func (t *SOLTracker) Observe(b []byte) {
	if len(b) == 0 {
		return
	}
	last := b[len(b)-1]
	t.atStart = last == '\r' || last == '\n'
}
