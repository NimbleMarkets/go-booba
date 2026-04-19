//go:build !js

package osc52gate

import (
	"bytes"
	"io"
	"testing"
)

// assembleOSC52 returns the bytes for a valid OSC 52 escape:
//
//	ESC ] 52 ; <sel> ; <data> BEL
func assembleOSC52(sel, data string) []byte {
	return []byte("\x1b]52;" + sel + ";" + data + "\x07")
}

func TestScannerAllowPassesThrough(t *testing.T) {
	inner := bytes.NewReader(append(append([]byte("hello "), assembleOSC52("c", "SGVsbG8=")...), []byte(" world")...))
	r := newScanner(inner, ModeAllow, nil)
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := append(append([]byte("hello "), assembleOSC52("c", "SGVsbG8=")...), []byte(" world")...)
	if !bytes.Equal(got, want) {
		t.Errorf("allow: got %q; want %q", got, want)
	}
}

func TestScannerDenyStripsEscape(t *testing.T) {
	inner := bytes.NewReader(append(append([]byte("hello "), assembleOSC52("c", "SGVsbG8=")...), []byte(" world")...))
	r := newScanner(inner, ModeDeny, nil)
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := []byte("hello  world")
	if !bytes.Equal(got, want) {
		t.Errorf("deny: got %q; want %q", got, want)
	}
}

func TestScannerDenyHandlesSplitRead(t *testing.T) {
	// The escape sequence spans multiple Reads; deny must still strip it.
	payload := append(append([]byte("pre-"), assembleOSC52("c", "SGk=")...), []byte("-post")...)
	inner := &chunkedReader{data: payload, chunkSize: 3}
	r := newScanner(inner, ModeDeny, nil)
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := []byte("pre--post")
	if !bytes.Equal(got, want) {
		t.Errorf("deny (split reads): got %q; want %q", got, want)
	}
}

func TestScannerAuditCallsCallback(t *testing.T) {
	inner := bytes.NewReader(assembleOSC52("p", "YQ=="))
	var observedSel, observedData string
	r := newScanner(inner, ModeAudit, func(sel string, dataLen int) {
		observedSel = sel
		observedData = "ok"
		_ = dataLen
	})
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, assembleOSC52("p", "YQ==")) {
		t.Errorf("audit: got %q; want pass-through", got)
	}
	if observedSel != "p" || observedData != "ok" {
		t.Errorf("audit callback: sel=%q observed=%q", observedSel, observedData)
	}
}

func TestScannerPassesThroughMalformedEscapes(t *testing.T) {
	// ESC ] 52 ; c ; then EOF with no terminator → not a valid escape.
	// Deny mode must NOT silently drop data; buffered bytes flush.
	incomplete := []byte("\x1b]52;c;SGk=")
	r := newScanner(bytes.NewReader(incomplete), ModeDeny, nil)
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, incomplete) {
		t.Errorf("malformed: got %q; want pass-through %q", got, incomplete)
	}
}

func TestScannerSTTerminator(t *testing.T) {
	// OSC 52 can also end with ESC \ (ST) instead of BEL.
	payload := []byte("\x1b]52;c;SGk=\x1b\\")
	r := newScanner(bytes.NewReader(append([]byte("a"), append(payload, 'b')...)), ModeDeny, nil)
	got, _ := io.ReadAll(r)
	want := []byte("ab")
	if !bytes.Equal(got, want) {
		t.Errorf("ST terminator: got %q; want %q", got, want)
	}
}

func TestScannerAllowSTTerminator(t *testing.T) {
	// Allow mode must pass an ST-terminated escape through verbatim —
	// no spurious BEL appended after the ESC\ sequence. This pins the
	// behavior that the earlier tail-trim heuristic implemented (and
	// that the explicit stTerminated flag now handles cleanly).
	payload := []byte("\x1b]52;c;SGk=\x1b\\")
	r := newScanner(bytes.NewReader(append([]byte("a"), append(payload, 'b')...)), ModeAllow, nil)
	got, _ := io.ReadAll(r)
	want := append([]byte("a"), append(payload, 'b')...)
	if !bytes.Equal(got, want) {
		t.Errorf("allow + ST terminator: got %q; want %q", got, want)
	}
}

// chunkedReader returns data chunkSize bytes at a time to exercise
// split-Read handling in the scanner.
type chunkedReader struct {
	data      []byte
	chunkSize int
	pos       int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := r.chunkSize
	if remaining := len(r.data) - r.pos; n > remaining {
		n = remaining
	}
	if n > len(p) {
		n = len(p)
	}
	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}
