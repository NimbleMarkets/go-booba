package sipclient

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const kittyPlaceholder = '\U0010EEEE'

type kittyAPCState uint8

const (
	kittyAPCNormal kittyAPCState = iota
	kittyAPCAfterEsc
	kittyAPCAfterEscUnderscore
	kittyAPCInside
	kittyAPCInsideEsc
)

// kittyGraphicsFilter strips Kitty graphics APC sequences from a byte stream.
// It also blanks Kitty's Unicode virtual-placement placeholder cells so
// terminals without graphics support do not show U+10EEEE fallback glyphs.
type kittyGraphicsFilter struct {
	apcState      kittyAPCState
	utf8Pending   []byte
	dropCombining bool
}

func (f *kittyGraphicsFilter) Filter(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	noAPC := f.stripKittyAPC(in)
	return f.stripKittyPlaceholders(noAPC)
}

func (f *kittyGraphicsFilter) stripKittyAPC(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for _, b := range in {
		switch f.apcState {
		case kittyAPCNormal:
			if b == 0x1b {
				f.apcState = kittyAPCAfterEsc
				continue
			}
			out = append(out, b)
		case kittyAPCAfterEsc:
			if b == '_' {
				f.apcState = kittyAPCAfterEscUnderscore
				continue
			}
			out = append(out, 0x1b)
			if b == 0x1b {
				f.apcState = kittyAPCAfterEsc
				continue
			}
			out = append(out, b)
			f.apcState = kittyAPCNormal
		case kittyAPCAfterEscUnderscore:
			if b == 'G' {
				f.apcState = kittyAPCInside
				continue
			}
			out = append(out, 0x1b, '_')
			if b == 0x1b {
				f.apcState = kittyAPCAfterEsc
				continue
			}
			out = append(out, b)
			f.apcState = kittyAPCNormal
		case kittyAPCInside:
			switch b {
			case 0x1b:
				f.apcState = kittyAPCInsideEsc
			case 0x07, 0x9c:
				f.apcState = kittyAPCNormal
			}
		case kittyAPCInsideEsc:
			if b == '\\' {
				f.apcState = kittyAPCNormal
				continue
			}
			if b == 0x1b {
				continue
			}
			f.apcState = kittyAPCInside
		}
	}
	return out
}

func (f *kittyGraphicsFilter) stripKittyPlaceholders(in []byte) []byte {
	if len(f.utf8Pending) > 0 {
		merged := make([]byte, 0, len(f.utf8Pending)+len(in))
		merged = append(merged, f.utf8Pending...)
		merged = append(merged, in...)
		in = merged
		f.utf8Pending = nil
	}

	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); {
		r, size := utf8.DecodeRune(in[i:])
		if r == utf8.RuneError && size == 1 && !utf8.FullRune(in[i:]) {
			f.utf8Pending = append(f.utf8Pending[:0], in[i:]...)
			break
		}
		if f.dropCombining {
			if unicode.Is(unicode.Mn, r) {
				i += size
				continue
			}
			f.dropCombining = false
		}
		if r == kittyPlaceholder {
			out = append(out, ' ')
			f.dropCombining = true
			i += size
			continue
		}
		out = append(out, in[i:i+size]...)
		i += size
	}
	return out
}

func localKittyGraphicsSupported(env []string, kittyKeyboardOK bool) bool {
	if kittyKeyboardOK {
		return true
	}
	values := map[string]string{}
	for _, entry := range env {
		key, val, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = val
		}
	}
	if values["KITTY_WINDOW_ID"] != "" || values["WEZTERM_PANE"] != "" {
		return true
	}
	switch strings.ToLower(values["TERM_PROGRAM"]) {
	case "ghostty", "wezterm", "kitty":
		return true
	}
	term := strings.ToLower(values["TERM"])
	return strings.Contains(term, "xterm-kitty") ||
		strings.Contains(term, "ghostty") ||
		strings.Contains(term, "wezterm")
}
