package sipclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/NimbleMarkets/go-booba/sip"
)

type fakeHandler struct {
	output  []byte
	title   string
	options sip.OptionsMessage
	kitty   int
	closed  []byte
}

func (h *fakeHandler) HandleOutput(p []byte)              { h.output = append(h.output, p...) }
func (h *fakeHandler) HandleTitle(s string)               { h.title = s }
func (h *fakeHandler) HandleOptions(o sip.OptionsMessage) { h.options = o }
func (h *fakeHandler) HandleKittyFlags(flags int)         { h.kitty = flags }
func (h *fakeHandler) HandleClose(p []byte)               { h.closed = append([]byte(nil), p...) }

func TestRouter_AllKnownTypes(t *testing.T) {
	cases := []struct {
		name    string
		msgType byte
		payload []byte
		check   func(t *testing.T, h *fakeHandler, pongs int)
	}{
		{
			name:    "output",
			msgType: sip.MsgOutput,
			payload: []byte("hello"),
			check: func(t *testing.T, h *fakeHandler, _ int) {
				if !bytes.Equal(h.output, []byte("hello")) {
					t.Errorf("output = %q; want 'hello'", h.output)
				}
			},
		},
		{
			name:    "title",
			msgType: sip.MsgTitle,
			payload: []byte("vim README.md"),
			check: func(t *testing.T, h *fakeHandler, _ int) {
				if h.title != "vim README.md" {
					t.Errorf("title = %q", h.title)
				}
			},
		},
		{
			name:    "options",
			msgType: sip.MsgOptions,
			payload: mustJSON(t, sip.OptionsMessage{ReadOnly: true}),
			check: func(t *testing.T, h *fakeHandler, _ int) {
				if !h.options.ReadOnly {
					t.Errorf("options.ReadOnly should be true")
				}
			},
		},
		{
			name:    "kitty",
			msgType: sip.MsgKittyKbd,
			payload: mustJSON(t, sip.KittyKbdMessage{Flags: 7}),
			check: func(t *testing.T, h *fakeHandler, _ int) {
				if h.kitty != 7 {
					t.Errorf("kitty flags = %d; want 7", h.kitty)
				}
			},
		},
		{
			name:    "ping triggers pong",
			msgType: sip.MsgPing,
			payload: nil,
			check: func(t *testing.T, _ *fakeHandler, pongs int) {
				if pongs != 1 {
					t.Errorf("pongs = %d; want 1", pongs)
				}
			},
		},
		{
			name:    "close",
			msgType: sip.MsgClose,
			payload: []byte("done"),
			check: func(t *testing.T, h *fakeHandler, _ int) {
				if !bytes.Equal(h.closed, []byte("done")) {
					t.Errorf("closed payload = %q", h.closed)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := &fakeHandler{}
			pongs := 0
			r := &Router{Handler: h, Pong: func() error { pongs++; return nil }}
			if err := r.Route(c.msgType, c.payload); err != nil {
				if !errors.Is(err, ErrSessionClosed) || c.msgType != sip.MsgClose {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			c.check(t, h, pongs)
		})
	}
}

func TestRouter_UnknownType_Errors(t *testing.T) {
	h := &fakeHandler{}
	r := &Router{Handler: h, Pong: func() error { return nil }}
	err := r.Route(byte('Z'), []byte("x"))
	if err == nil {
		t.Fatalf("want error for unknown type, got nil")
	}
}

func TestRouter_UnknownType_DebugLogs(t *testing.T) {
	h := &fakeHandler{}
	var logged struct {
		typ byte
		p   []byte
	}
	r := &Router{
		Handler: h,
		Pong:    func() error { return nil },
		Debug:   func(t byte, p []byte) { logged.typ = t; logged.p = p },
	}
	if err := r.Route(byte('Z'), []byte("x")); err != nil {
		t.Fatalf("debug mode should suppress unknown-type error, got: %v", err)
	}
	if logged.typ != 'Z' || string(logged.p) != "x" {
		t.Errorf("debug = (%q, %q); want ('Z', 'x')", logged.typ, logged.p)
	}
}

func TestRouter_MsgCloseReturnsSentinel(t *testing.T) {
	r := &Router{Handler: &fakeHandler{}, Pong: func() error { return nil }}
	err := r.Route(sip.MsgClose, nil)
	if !errors.Is(err, ErrSessionClosed) {
		t.Errorf("err = %v; want ErrSessionClosed", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
