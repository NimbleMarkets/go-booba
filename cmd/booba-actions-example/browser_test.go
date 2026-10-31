//go:build !js

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/go-booba/actions"
	"github.com/NimbleMarkets/go-booba/sip"
	"github.com/coder/websocket"
)

func TestBrowserDemo(t *testing.T) {
	handler, err := browserHandler("")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	for _, path := range []string{"/", "/static/booba/booba.js", "/static/booba/action_bar.js", "/static/ntwebterm/ghostty-vt.wasm"} {
		res, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || len(body) == 0 {
			t.Fatalf("%s: status %d, bytes %d", path, res.StatusCode, len(body))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()

	send := func(data string) {
		t.Helper()
		if err := conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgInput, []byte(data))); err != nil {
			t.Fatal(err)
		}
	}
	if err := conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgResize, []byte(`{"cols":80,"rows":24}`))); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	pending := ""
	var latest actions.Snapshot
	readUntil := func(wantContext, wantOutput string) actions.Snapshot {
		t.Helper()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %s/%s: %v; output=%q", wantContext, wantOutput, err, output.String())
			}
			kind, payload, err := sip.DecodeWSMessage(data)
			if err != nil {
				t.Fatal(err)
			}
			if kind != sip.MsgOutput {
				continue
			}
			output.Write(payload)
			pending += string(payload)
			for {
				start := strings.Index(pending, "\x1b]7501;")
				if start < 0 {
					break
				}
				end := strings.Index(pending[start:], "\x1b\\")
				if end < 0 {
					break
				}
				body := pending[start+7 : start+end]
				pending = pending[start+end+2:]
				if body == "?" {
					send("\x1b]7501;?:states=idle,working,done,blocked,error:kinds=permission,question,auth\x1b\\")
					continue
				}
				for _, field := range strings.Split(body, ":") {
					if !strings.HasPrefix(field, "msg=") {
						continue
					}
					msg, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(field, "msg="))
					if err != nil {
						t.Fatal(err)
					}
					_, snapshot, ok := strings.Cut(string(msg), actions.MessageMarker)
					if !ok {
						continue
					}
					if err := json.Unmarshal([]byte(snapshot), &latest); err != nil {
						t.Fatal(err)
					}
					if err := latest.Validate(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if (wantContext == "" || latest.Context == wantContext) && strings.Contains(output.String(), wantOutput) {
				return latest
			}
		}
	}

	initial := readUntil("files", "")
	send("\x1b[B\r")
	confirm := readUntil("confirm", "Open item 2?")
	if confirm.Revision <= initial.Revision || confirm.Actions[0].Label != "Confirm" {
		t.Fatalf("bad confirmation snapshot: %+v", confirm)
	}
	send("\x1b")
	readUntil("files", "")
	send("\r")
	readUntil("confirm", "")
	send("\r")
	readUntil("files", "Opened item 2")
	send("\x03")
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("program did not close")
			}
			break
		}
	}
}
