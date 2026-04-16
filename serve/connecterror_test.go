//go:build !js

package serve

import (
	"errors"
	"testing"
)

func TestConnectErrorDefaultWTMapping(t *testing.T) {
	cases := []struct {
		status int
		want   uint32
	}{
		{200, 0x00},
		{301, 0x00},
		{401, 0x01},
		{418, 0x01},
		{500, 0x02},
		{503, 0x02},
	}
	for _, c := range cases {
		got := (&ConnectError{Status: c.status}).WTErrorCode()
		if got != c.want {
			t.Errorf("status=%d → WTErrorCode=%d; want %d", c.status, got, c.want)
		}
	}
}

func TestConnectErrorExplicitWTCodeOverride(t *testing.T) {
	e := &ConnectError{Status: 401, WTCode: 0x99}
	if got := e.WTErrorCode(); got != 0x99 {
		t.Errorf("WTErrorCode = %d; want 0x99", got)
	}
}

func TestConnectErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("upstream")
	e := &ConnectError{Status: 502, Cause: cause}
	if !errors.Is(e, cause) {
		t.Error("errors.Is(e, cause) = false; want true")
	}
}

func TestConnectErrorErrorString(t *testing.T) {
	e := &ConnectError{Status: 401, Body: "Unauthorized"}
	got := e.Error()
	if got == "" {
		t.Error("Error() returned empty string")
	}
	if !contains(got, "401") {
		t.Errorf("Error() = %q; want it to contain status 401", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
