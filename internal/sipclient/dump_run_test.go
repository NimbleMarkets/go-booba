package sipclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	if err := json.Unmarshal([]byte(lines[0]), &m0); err != nil {
		t.Fatalf("line 0 not valid JSON: %v (%q)", err, lines[0])
	}
	if err := json.Unmarshal([]byte(lines[1]), &m1); err != nil {
		t.Fatalf("line 1 not valid JSON: %v (%q)", err, lines[1])
	}
	if err := json.Unmarshal([]byte(lines[2]), &m2); err != nil {
		t.Fatalf("line 2 not valid JSON: %v (%q)", err, lines[2])
	}
	if m0["type"] != "options" {
		t.Errorf("line 0 type = %v; want options", m0["type"])
	}
	if m1["type"] != "output" {
		t.Errorf("line 1 type = %v; want output", m1["type"])
	} else {
		data, err := base64.StdEncoding.DecodeString(m1["data"].(string))
		if err != nil {
			t.Fatalf("output data not valid base64: %v", err)
		}
		if string(data) != "hello\r\n" {
			t.Errorf("output payload = %q; want %q", data, "hello\r\n")
		}
	}
	if m2["type"] != "close" {
		t.Errorf("line 2 type = %v; want close", m2["type"])
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr output (Debug is off): %q", stderr.String())
	}
}

func TestRunDump_DumpInput(t *testing.T) {
	// This server echoes back any MsgInput payload as a MsgOutput frame.
	received := make(chan []byte, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		// RunDump always sends an initial MsgResize; consume and ignore it.
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		msgType, _, _ := sip.DecodeWSMessage(data)
		if msgType != sip.MsgResize {
			t.Errorf("server got type=%q as first frame; want MsgResize", msgType)
			return
		}
		// Read the second frame (expected: MsgInput).
		_, data, err = conn.Read(r.Context())
		if err != nil {
			return
		}
		msgType, payload, _ := sip.DecodeWSMessage(data)
		if msgType != sip.MsgInput {
			t.Errorf("server got type=%q; want MsgInput", msgType)
		}
		received <- payload
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgOutput, payload))
		_ = conn.Write(r.Context(), websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgClose, nil))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	abs, err := filepath.Abs("testdata/greeting.txt")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	opts := &Options{
		URL:            url,
		EscapeCharRaw:  "^]",
		DumpInputPath:  abs,
		ConnectTimeout: 5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RunDump(ctx, &stdout, &stderr, opts); err != nil {
		t.Fatalf("RunDump: %v", err)
	}

	select {
	case got := <-received:
		if string(got) != "hello-from-client\n" {
			t.Errorf("server received %q; want %q", got, "hello-from-client\n")
		}
	default:
		t.Fatalf("server never received a MsgInput")
	}
	if !strings.Contains(stdout.String(), `"type":"output"`) {
		t.Errorf("stdout should contain an output frame; got %q", stdout.String())
	}
}
