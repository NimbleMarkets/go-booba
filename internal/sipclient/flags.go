package sipclient

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ParseTargetURL validates the positional URL arg. It accepts ws:// and wss://,
// defaults the path to /ws when empty, and returns a cleaned *url.URL.
func ParseTargetURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("url is required (e.g., ws://host:8080/ws)")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "ws", "wss":
	default:
		return nil, fmt.Errorf("unsupported scheme %q (want ws or wss)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("host is required in url")
	}
	if u.Path == "" {
		u.Path = "/ws"
	}
	return u, nil
}
