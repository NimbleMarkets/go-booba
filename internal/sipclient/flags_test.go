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
