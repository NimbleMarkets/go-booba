package sipclient

import (
	"strings"
	"testing"
)

func TestParseTargetURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{"ws with port no path", "ws://localhost:8080", "ws://localhost:8080/ws", ""},
		{"ws with custom path", "ws://localhost:8080/custom", "ws://localhost:8080/custom", ""},
		{"wss with path", "wss://host/path", "wss://host/path", ""},
		{"ws no port", "ws://example.com/", "ws://example.com/", ""},
		{"http scheme rejected", "http://localhost", "", "unsupported scheme"},
		{"no scheme rejected", "localhost:8080", "", "unsupported scheme"},
		{"empty rejected", "", "", "url is required"},
		{"no host rejected", "ws:///path", "", "host is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, err := ParseTargetURL(c.in)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("err = %q; want contains %q", err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := u.String(); got != c.want {
				t.Errorf("ParseTargetURL(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseEscapeChar(t *testing.T) {
	cases := []struct {
		in       string
		wantByte byte
		wantNone bool
		wantErr  string
	}{
		{"^]", 0x1d, false, ""},
		{"^A", 0x01, false, ""},
		{"^a", 0x01, false, ""},
		{"^@", 0x00, false, ""},
		{"^_", 0x1f, false, ""},
		{"^[", 0x1b, false, ""},
		{"^?", 0x7f, false, ""},
		{"none", 0, true, ""},
		{"NONE", 0, true, ""},
		{"", 0, false, "invalid escape char"},
		{"^", 0, false, "invalid escape char"},
		{"abc", 0, false, "invalid escape char"},
		{"^1", 0, false, "invalid escape char"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseEscapeChar(c.in)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("err = %q; want contains %q", err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.None != c.wantNone {
				t.Errorf("None = %v; want %v", got.None, c.wantNone)
			}
			if got.Byte != c.wantByte {
				t.Errorf("Byte = 0x%02x; want 0x%02x", got.Byte, c.wantByte)
			}
		})
	}
}
