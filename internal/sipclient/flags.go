package sipclient

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ParseTargetURL validates the positional URL arg. It accepts ws://, wss://, and
// https:// (WebTransport), defaults the path to /ws (ws/wss) or /wt (https) when
// empty, and returns a cleaned *url.URL.
func ParseTargetURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("url is required (e.g., ws://host:8080/ws)")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "ws", "wss", "https":
	default:
		return nil, fmt.Errorf("unsupported scheme %q (want ws, wss, or https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("host is required in url")
	}
	if u.Path == "" {
		if strings.EqualFold(u.Scheme, "https") {
			u.Path = "/wt"
		} else {
			u.Path = "/ws"
		}
	}
	return u, nil
}

// EscapeChar represents a parsed --escape-char value.
// None means the escape mechanism is disabled.
type EscapeChar struct {
	Byte byte
	None bool
}

// ParseEscapeChar accepts "^X" notation (where X is an uppercase letter, @,
// [, \, ], ^, _, or ?) or the literal "none" (case-insensitive) to disable.
func ParseEscapeChar(s string) (EscapeChar, error) {
	if strings.EqualFold(s, "none") {
		return EscapeChar{None: true}, nil
	}
	if len(s) != 2 || s[0] != '^' {
		return EscapeChar{}, fmt.Errorf("invalid escape char %q (want ^X or 'none')", s)
	}
	c := s[1]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	switch {
	case c == '@':
		return EscapeChar{Byte: 0x00}, nil
	case c >= 'A' && c <= 'Z':
		return EscapeChar{Byte: c - '@'}, nil // '@' is 0x40, so 'A' - '@' = 1
	case c == '[':
		return EscapeChar{Byte: 0x1b}, nil
	case c == '\\':
		return EscapeChar{Byte: 0x1c}, nil
	case c == ']':
		return EscapeChar{Byte: 0x1d}, nil
	case c == '^':
		return EscapeChar{Byte: 0x1e}, nil
	case c == '_':
		return EscapeChar{Byte: 0x1f}, nil
	case c == '?':
		return EscapeChar{Byte: 0x7f}, nil
	default:
		return EscapeChar{}, fmt.Errorf("invalid escape char %q", s)
	}
}

// ParseHeaders turns repeated "Key: Value" flag values into an http.Header.
func ParseHeaders(raws []string) (http.Header, error) {
	h := http.Header{}
	for _, raw := range raws {
		i := strings.IndexByte(raw, ':')
		if i <= 0 {
			return nil, fmt.Errorf("invalid --header %q (want 'Key: Value')", raw)
		}
		key := strings.TrimSpace(raw[:i])
		val := strings.TrimSpace(raw[i+1:])
		if key == "" {
			return nil, fmt.Errorf("invalid --header %q (empty key)", raw)
		}
		h.Add(key, val)
	}
	return h, nil
}
