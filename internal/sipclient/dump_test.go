package sipclient

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/NimbleMarkets/go-booba/sip"
)

func TestDumpHandler_Frames(t *testing.T) {
	var buf bytes.Buffer
	h := NewDumpHandler(&buf)

	h.HandleOutput([]byte("hi"))
	h.HandleTitle("vim")
	h.HandleOptions(sip.OptionsMessage{ReadOnly: true})
	h.HandleKittyFlags(3)
	h.HandleClose([]byte("bye"))

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("want 5 lines, got %d: %q", len(lines), buf.String())
	}
	want := []map[string]any{
		{"type": "output", "data": "aGk="}, // base64("hi")
		{"type": "title", "title": "vim"},
		{"type": "options", "readOnly": true},
		{"type": "kitty", "flags": float64(3)},
		{"type": "close", "data": "Ynll"}, // base64("bye")
	}
	for i, line := range lines {
		var got map[string]any
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d not valid JSON: %v (%q)", i, err, line)
		}
		for k, v := range want[i] {
			if got[k] != v {
				t.Errorf("line %d key %q = %v; want %v", i, k, got[k], v)
			}
		}
	}
}
