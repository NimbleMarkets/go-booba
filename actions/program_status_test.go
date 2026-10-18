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

func TestDecodeProgramStatus(t *testing.T) {
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
	wire, err := EncodeProgramStatus(s, ProgramStatus{App: "demo", Summary: "Choose an item"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeProgramStatus(wire)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := StatusID(s.Instance)
	if r.Clear || r.ID != id || r.Status.State != "idle" || r.Status.App != "demo" || r.Status.Summary != "Choose an item" {
		t.Fatalf("report %+v", r)
	}
	if r.Snapshot == nil || r.Snapshot.Instance != s.Instance || len(r.Snapshot.Actions) != len(s.Actions) {
		t.Fatalf("snapshot %+v", r.Snapshot)
	}
	// BEL termination and plain messages decode too.
	bel := strings.TrimSuffix(wire, "\x1b\\") + "\a"
	if _, err := DecodeProgramStatus(bel); err != nil {
		t.Fatal(err)
	}
	plain, err := DecodeProgramStatus("\x1b]7501;state=working:id=app:msg=" + base64.StdEncoding.EncodeToString([]byte("Loading")) + "\a")
	if err != nil || plain.Snapshot != nil || plain.Status.Summary != "Loading" || plain.ID != "app" {
		t.Fatalf("plain %+v %v", plain, err)
	}
	clear, _ := ClearProgramStatus(s.Instance)
	r, err = DecodeProgramStatus(clear)
	if err != nil || !r.Clear || r.ID != id {
		t.Fatalf("clear %+v %v", r, err)
	}
	if !IsProgramStatusQuery(QueryProgramStatus) || !IsProgramStatusQuery("\x1b]7501;?\a") || IsProgramStatusQuery(wire) {
		t.Fatal("query detection")
	}
	for _, bad := range []string{QueryProgramStatus, "\x1b]7501;id=x\a", "\x1b]52;c;AAAA\a", "\x1b]7501;state=idle:msg=!!!\a",
		"\x1b]7501;state=idle:msg=" + base64.StdEncoding.EncodeToString([]byte("x"+MessageMarker+"{bad")) + "\a"} {
		if _, err := DecodeProgramStatus(bad); err == nil {
			t.Fatalf("decoded %q", bad)
		}
	}
}
