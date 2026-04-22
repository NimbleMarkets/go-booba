package sipclient

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseKittyResponse(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantFlags int
		wantOK    bool
	}{
		{"no response", "", 0, false},
		{"plain flags", "\x1b[?3u", 3, true},
		{"with garbage prefix", "junk\x1b[?15u", 15, true},
		{"zero flags", "\x1b[?0u", 0, true},
		{"wrong terminator", "\x1b[?3x", 0, false},
		{"non-numeric", "\x1b[?Au", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			flags, ok := parseKittyResponse([]byte(c.input))
			if ok != c.wantOK {
				t.Errorf("ok = %v; want %v", ok, c.wantOK)
			}
			if flags != c.wantFlags {
				t.Errorf("flags = %d; want %d", flags, c.wantFlags)
			}
		})
	}
}

func TestQueryKittyFlags_Timeout(t *testing.T) {
	// A reader that never returns any bytes simulates a terminal without
	// Kitty support — QueryKittyFlags must time out and report "not
	// supported".
	r, w := io.Pipe()
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()
	var out bytes.Buffer
	flags, ok := QueryKittyFlags(r, &out, 50*time.Millisecond)
	if ok {
		t.Errorf("unsupported terminal should return ok=false")
	}
	if flags != 0 {
		t.Errorf("flags = %d; want 0", flags)
	}
	if !strings.Contains(out.String(), "\x1b[?u") {
		t.Errorf("expected CSI ? u query in output, got %q", out.String())
	}
}

func TestQueryKittyFlags_Response(t *testing.T) {
	// Pre-fill the reader with a valid response, then QueryKittyFlags
	// should read it and return the flags.
	r := strings.NewReader("\x1b[?7u")
	var out bytes.Buffer
	flags, ok := QueryKittyFlags(r, &out, 500*time.Millisecond)
	if !ok || flags != 7 {
		t.Errorf("flags=%d ok=%v; want 7 true", flags, ok)
	}
}

func TestPushPopKittyFlags(t *testing.T) {
	var buf bytes.Buffer
	if err := PushKittyFlags(&buf, 3); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "\x1b[>3u" {
		t.Errorf("push = %q; want \\x1b[>3u", got)
	}
	buf.Reset()
	if err := PopKittyFlags(&buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "\x1b[<u" {
		t.Errorf("pop = %q; want \\x1b[<u", got)
	}
}
