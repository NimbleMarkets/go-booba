package sipclient

import (
	"bytes"
	"strings"
	"testing"
)

func TestKittyGraphicsFilter_StripsAPC(t *testing.T) {
	var f kittyGraphicsFilter
	out := f.Filter([]byte("before\x1b_Ga=T,f=32;AAAA\x1b\\after"))
	if got, want := string(out), "beforeafter"; got != want {
		t.Fatalf("Filter() = %q; want %q", got, want)
	}
}

func TestKittyGraphicsFilter_StripsAPCAcrossChunks(t *testing.T) {
	var f kittyGraphicsFilter
	var out bytes.Buffer
	out.Write(f.Filter([]byte("a\x1b_Ga=T,")))
	out.Write(f.Filter([]byte("f=32;AAAA\x1b")))
	out.Write(f.Filter([]byte("\\b")))
	if got, want := out.String(), "ab"; got != want {
		t.Fatalf("stream output = %q; want %q", got, want)
	}
}

func TestKittyGraphicsFilter_PreservesOtherAPC(t *testing.T) {
	var f kittyGraphicsFilter
	in := []byte("x\x1b_Xnot kitty\x1b\\y")
	out := f.Filter(in)
	if got := string(out); got != string(in) {
		t.Fatalf("Filter() = %q; want passthrough %q", got, in)
	}
}

func TestKittyGraphicsFilter_StripsVirtualPlaceholder(t *testing.T) {
	var f kittyGraphicsFilter
	out := f.Filter([]byte("a\U0010EEEE\u0305\u030Db"))
	if got, want := string(out), "a b"; got != want {
		t.Fatalf("Filter() = %q; want %q", got, want)
	}
}

func TestKittyGraphicsFilter_StripsSplitVirtualPlaceholder(t *testing.T) {
	var f kittyGraphicsFilter
	placeholder := []byte("\U0010EEEE\u0305\u030D")
	var out bytes.Buffer
	out.Write(f.Filter([]byte("a")))
	out.Write(f.Filter(placeholder[:2]))
	out.Write(f.Filter(placeholder[2:]))
	out.Write(f.Filter([]byte("b")))
	if got, want := out.String(), "a b"; got != want {
		t.Fatalf("stream output = %q; want %q", got, want)
	}
}

func TestLocalKittyGraphicsSupported(t *testing.T) {
	cases := []struct {
		name            string
		env             []string
		kittyKeyboardOK bool
		want            bool
	}{
		{"keyboard query ok", nil, true, true},
		{"kitty env", []string{"KITTY_WINDOW_ID=1"}, false, true},
		{"wezterm env", []string{"WEZTERM_PANE=1"}, false, true},
		{"ghostty term program", []string{"TERM_PROGRAM=ghostty"}, false, true},
		{"plain xterm", []string{"TERM=xterm-256color"}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := localKittyGraphicsSupported(c.env, c.kittyKeyboardOK); got != c.want {
				t.Fatalf("localKittyGraphicsSupported(%s) = %v; want %v", strings.Join(c.env, ","), got, c.want)
			}
		})
	}
}
