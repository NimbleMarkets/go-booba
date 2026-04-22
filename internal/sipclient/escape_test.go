package sipclient

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSOLTracker(t *testing.T) {
	tr := NewSOLTracker()
	if !tr.AtStart() {
		t.Fatalf("initial state should be AtStart=true")
	}
	tr.Observe([]byte("hello"))
	if tr.AtStart() {
		t.Errorf("after 'hello', AtStart should be false")
	}
	tr.Observe([]byte("\r"))
	if !tr.AtStart() {
		t.Errorf("after '\\r', AtStart should be true")
	}
	tr.Observe([]byte("x"))
	if tr.AtStart() {
		t.Errorf("after 'x', AtStart should be false")
	}
	tr.Observe([]byte("\n"))
	if !tr.AtStart() {
		t.Errorf("after '\\n', AtStart should be true")
	}
	tr.Observe([]byte("ab\r\ncd"))
	if tr.AtStart() {
		t.Errorf("after 'ab\\r\\ncd', AtStart should be false (cd terminates the line)")
	}
	tr.Observe([]byte{})
	if tr.AtStart() {
		t.Errorf("empty Observe should not change state")
	}
}

func TestRunEscapePrompt(t *testing.T) {
	info := PromptInfo{URL: "ws://host/ws", Started: time.Now().Add(-3 * time.Second)}
	cases := []struct {
		name    string
		input   string
		want    EscapeAction
		wantOut []string // substrings that must appear in output
	}{
		{"quit", "quit\n", ActionDisconnect, []string{"booba-sip-client>"}},
		{"exit", "exit\n", ActionDisconnect, nil},
		{"q", "q\n", ActionDisconnect, nil},
		{"continue", "continue\n", ActionContinue, nil},
		{"blank returns continue", "\n", ActionContinue, nil},
		{"help prints and stays then continues", "help\n\n", ActionContinue, []string{"quit", "continue", "status", "help"}},
		{"status prints info then continues", "status\n\n", ActionContinue, []string{"ws://host/ws"}},
		{"unknown prints help then continues", "wat\n\n", ActionContinue, []string{"unknown command"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := RunEscapePrompt(strings.NewReader(c.input), &out, info)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("action = %v; want %v", got, c.want)
			}
			for _, s := range c.wantOut {
				if !strings.Contains(out.String(), s) {
					t.Errorf("output = %q; want contains %q", out.String(), s)
				}
			}
		})
	}
}

func TestRunEscapePrompt_CRTerminator(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"bare CR", "quit\r"},
		{"CRLF", "quit\r\n"},
		{"bare LF", "quit\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := RunEscapePrompt(strings.NewReader(c.input), &out, PromptInfo{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != ActionDisconnect {
				t.Errorf("action = %v; want ActionDisconnect", got)
			}
		})
	}
}

func TestRunEscapePrompt_EOFDisconnects(t *testing.T) {
	var out bytes.Buffer
	got, err := RunEscapePrompt(strings.NewReader(""), &out, PromptInfo{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ActionDisconnect {
		t.Errorf("EOF should return ActionDisconnect; got %v", got)
	}
}
