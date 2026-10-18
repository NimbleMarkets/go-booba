package termhost

import (
	"bytes"
	"strings"
	"testing"

	"github.com/NimbleMarkets/go-booba/actions"
)

func testSnapshot() actions.Snapshot {
	return actions.Snapshot{Version: 1, Instance: "demo", Context: "main", Revision: 1,
		Actions: []actions.Action{
			{ID: "pop", Label: "Pop", Hint: "enter", Enabled: true, Keys: []actions.Gesture{{Key: "enter", Modifiers: []string{}}}},
			{ID: "color", Label: "Color", Hint: "right", Enabled: false, Keys: []actions.Gesture{{Key: "right", Modifiers: []string{}}}},
			{ID: "quit", Label: "Quit", Hint: "q", Enabled: true, Keys: []actions.Gesture{{Key: "q", Modifiers: []string{}}}},
		},
		Short: []string{"pop", "color"}, Full: [][]string{{"pop", "color"}, {"quit"}}}
}

func report(t *testing.T) string {
	t.Helper()
	wire, err := actions.EncodeProgramStatus(testSnapshot(), actions.ProgramStatus{Summary: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

// write feeds s to w and fails the test on error.
func write(t *testing.T, w *Writer, s string) {
	t.Helper()
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
}

func TestWriterPassesThroughUntilHosting(t *testing.T) {
	var out bytes.Buffer
	w, _ := newWriter(&out)
	wire := report(t)
	in := "a" + actions.QueryProgramStatus + wire + "b"
	if n, err := w.Write([]byte(in)); err != nil || n != len(in) {
		t.Fatal(n, err)
	}
	if out.String() != in || len(w.drain()) != 0 {
		t.Fatalf("probing mode altered output: %q", out.String())
	}
	out.Reset()
	w.setMode(Passthrough)
	write(t, w, in)
	if out.String() != in || len(w.drain()) != 0 {
		t.Fatalf("passthrough mode altered output: %q", out.String())
	}
}

func TestWriterInterceptsWhileHosting(t *testing.T) {
	var out bytes.Buffer
	w, _ := newWriter(&out)
	w.setMode(Hosting)
	wire := report(t)
	clear, _ := actions.ClearProgramStatus("demo")
	in := "a" + actions.QueryProgramStatus + "b\x1b]52;c;AAAA\a" + wire + "c" + clear + "\x1b[31md\x1b[0m"
	if n, err := w.Write([]byte(in)); err != nil || n != len(in) {
		t.Fatal(n, err)
	}
	if got, want := out.String(), "ab\x1b]52;c;AAAA\ac\x1b[31md\x1b[0m"; got != want {
		t.Fatalf("output %q, want %q", got, want)
	}
	events := w.drain()
	if len(events) != 3 || !events[0].query || events[1].err != nil || events[1].report.Snapshot == nil || !events[2].report.Clear {
		t.Fatalf("events %+v", events)
	}
	select {
	case <-w.wake:
	default:
		t.Fatal("no wake")
	}
	if len(w.drain()) != 0 {
		t.Fatal("drain did not empty")
	}
}

func TestWriterReassemblesSplitSequences(t *testing.T) {
	var out bytes.Buffer
	w, _ := newWriter(&out)
	w.setMode(Hosting)
	wire := report(t)
	in := "x" + wire + "y\x1b"
	for i := range len(in) {
		if _, err := w.Write([]byte{in[i]}); err != nil {
			t.Fatal(err)
		}
	}
	write(t, w, "[0m")
	if got := out.String(); got != "xy\x1b[0m" {
		t.Fatalf("output %q", got)
	}
	if events := w.drain(); len(events) != 1 || events[0].report.Snapshot == nil {
		t.Fatalf("events %+v", events)
	}
	// An ESC that does not continue an OSC is released with the next write.
	out.Reset()
	write(t, w, "\x1b")
	write(t, w, "[2J")
	if out.String() != "\x1b[2J" {
		t.Fatalf("lone escape %q", out.String())
	}
}

func TestWriterReleasesUnterminatedOSC(t *testing.T) {
	var out bytes.Buffer
	w, _ := newWriter(&out)
	w.setMode(Hosting)
	// A stray ESC ends an unterminated OSC, which passes through.
	write(t, w, "\x1b]7501;state=idle\x1b[0m")
	if out.String() != "\x1b]7501;state=idle\x1b[0m" {
		t.Fatalf("stray escape %q", out.String())
	}
	out.Reset()
	long := "\x1b]" + strings.Repeat("z", maxCarry)
	write(t, w, long)
	if out.String() != long {
		t.Fatalf("oversized carry not released: %d bytes", out.Len())
	}
}

func TestWriterReportsMalformedReports(t *testing.T) {
	var out bytes.Buffer
	w, _ := newWriter(&out)
	w.setMode(Hosting)
	write(t, w, "\x1b]7501;state=idle:msg=!!!\a")
	if events := w.drain(); len(events) != 1 || events[0].err == nil {
		t.Fatalf("events %+v", events)
	}
	if out.Len() != 0 {
		t.Fatalf("malformed report leaked: %q", out.String())
	}
}
