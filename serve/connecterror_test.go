//go:build !js

package serve

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if !strings.Contains(got, "401") {
		t.Errorf("Error() = %q; want it to contain status 401", got)
	}
}

func TestConnectErrorStringIncludesCause(t *testing.T) {
	cause := errors.New("token expired")
	e := &ConnectError{Status: 401, Cause: cause}
	got := e.Error()
	if !strings.Contains(got, "401") {
		t.Errorf("Error() = %q; want it to contain status 401", got)
	}
	if !strings.Contains(got, "token expired") {
		t.Errorf("Error() = %q; want it to include cause %q", got, "token expired")
	}
}

func TestWriteConnectErrorWritesStatusHeadersAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	err := &ConnectError{
		Status:  401,
		Headers: http.Header{"WWW-Authenticate": []string{`Basic realm="booba"`}},
		Body:    "Unauthorized",
	}
	writeConnectError(rec, err)
	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("status = %d; want 401", res.StatusCode)
	}
	if got := res.Header.Get("WWW-Authenticate"); got != `Basic realm="booba"` {
		t.Errorf("WWW-Authenticate = %q; want Basic realm=\"booba\"", got)
	}
	body := rec.Body.String()
	if body != "Unauthorized\n" && body != "Unauthorized" {
		t.Errorf("body = %q; want Unauthorized (with or without trailing newline)", body)
	}
}

func TestWriteConnectErrorPlainErrorIs500(t *testing.T) {
	rec := httptest.NewRecorder()
	writeConnectError(rec, errors.New("boom"))
	if rec.Code != 500 {
		t.Errorf("status = %d; want 500", rec.Code)
	}
}
