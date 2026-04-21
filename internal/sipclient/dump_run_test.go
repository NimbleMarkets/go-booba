package sipclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/go-booba/sip"
	"github.com/coder/websocket"
)

// fakeServer is a minimal /ws endpoint that sends one options frame, one
// output frame, then a close frame. Mirrors the shape a real booba server
// produces without pulling serve/ into the test.
func fakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Error(err)
			return
		}
		ctx := r.Context()
		opts, _ := json.Marshal(sip.OptionsMessage{ReadOnly: false})
		_ = conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgOptions, opts))
		_ = conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgOutput, []byte("hello\r\n")))
		_ = conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgClose, nil))
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	return httptest.NewServer(mux)
}

func TestRunDump_HappyPath(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	var stdout, stderr bytes.Buffer
	opts := &Options{URL: url, EscapeCharRaw: "^]", ConnectTimeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RunDump(ctx, &stdout, &stderr, opts); err != nil {
		t.Fatalf("RunDump: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), stdout.String())
	}
	var m0, m1, m2 map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &m0)
	_ = json.Unmarshal([]byte(lines[1]), &m1)
	_ = json.Unmarshal([]byte(lines[2]), &m2)
	if m0["type"] != "options" {
		t.Errorf("line 0 type = %v; want options", m0["type"])
	}
	if m1["type"] != "output" {
		t.Errorf("line 1 type = %v; want output", m1["type"])
	}
	if m2["type"] != "close" {
		t.Errorf("line 2 type = %v; want close", m2["type"])
	}
}
