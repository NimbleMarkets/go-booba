package actions

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestProgramStatusEnvelope(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshots.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct{ Snapshot json.RawMessage }
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	var s Snapshot
	if err := json.Unmarshal(fixtures[0].Snapshot, &s); err != nil {
		t.Fatal(err)
	}
	wire, err := EncodeProgramStatus(s, ProgramStatus{Summary: "Choose an item"})
	if err != nil {
		t.Fatal(err)
	}
	id, _ := StatusID(s.Instance)
	if !strings.HasPrefix(wire, "\x1b]7501;state=idle:id="+id+":msg=") || !strings.HasSuffix(wire, "\x1b\\") {
		t.Fatalf("bad envelope: %q", wire)
	}
	_, encoded, _ := strings.Cut(wire, ":msg=")
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(encoded, "\x1b\\"))
	if err != nil {
		t.Fatal(err)
	}
	summary, payload, ok := strings.Cut(string(decoded), MessageMarker)
	if !ok || summary != "Choose an item" {
		t.Fatal(string(decoded))
	}
	if _, err := Decode([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	s.Actions[0].Label = strings.Repeat("界", 85)
	for i := 0; i < 10; i++ {
		a := s.Actions[0]
		a.ID = string(rune('a' + i))
		s.Actions = append(s.Actions, a)
	}
	if _, err := EncodeProgramStatus(s, ProgramStatus{Summary: "Choose"}); err == nil {
		t.Fatal("expected oversized report rejection")
	}
}
