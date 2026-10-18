package termhost

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/NimbleMarkets/go-booba/actions"
)

// maxCarry bounds the bytes held back while waiting for an OSC terminator. An
// OSC 7501 report is at most about 3 KB on the wire; anything longer passes
// through untouched rather than stalling the renderer.
const maxCarry = 8192

// Writer sits between Bubble Tea's renderer and the terminal. While Hosting it
// consumes OSC 7501 capability queries and reports and queues them for the
// Model; in every other mode it forwards each byte unchanged. Sequences may
// span writes, so an incomplete OSC is held until its terminator arrives.
type Writer struct {
	out     io.Writer
	mode    atomic.Int32
	carry   []byte
	mu      sync.Mutex
	pending []event
	wake    chan struct{}
}

type event struct {
	query  bool
	report actions.Report
	err    error
}

type fileLike interface {
	io.ReadWriteCloser
	Fd() uintptr
}

// fileWriter keeps Bubble Tea's terminal detection working when the wrapped
// output is a tty: the program looks for Fd on its output.
type fileWriter struct {
	*Writer
	file fileLike
}

func (f *fileWriter) Fd() uintptr                { return f.file.Fd() }
func (f *fileWriter) Read(p []byte) (int, error) { return f.file.Read(p) }
func (f *fileWriter) Close() error               { return f.file.Close() }

func newWriter(out io.Writer) (*Writer, io.Writer) {
	w := &Writer{out: out, wake: make(chan struct{}, 1)}
	if f, ok := out.(fileLike); ok {
		return w, &fileWriter{w, f}
	}
	return w, w
}

func (w *Writer) setMode(m Mode) { w.mode.Store(int32(m)) }
func (w *Writer) hosting() bool  { return Mode(w.mode.Load()) == Hosting }

// Write forwards p, intercepting complete OSC 7501 sequences while Hosting.
// Held-back bytes count as written.
func (w *Writer) Write(p []byte) (int, error) {
	data := p
	if len(w.carry) > 0 {
		data = append(w.carry, p...)
		w.carry = nil
	}
	for len(data) > 0 {
		i := bytes.Index(data, []byte("\x1b]"))
		if i < 0 {
			if data[len(data)-1] == 0x1b {
				// A trailing ESC may start an OSC in the next write.
				w.carry = []byte{0x1b}
				data = data[:len(data)-1]
			}
			return len(p), w.flush(data)
		}
		if err := w.flush(data[:i]); err != nil {
			return 0, err
		}
		data = data[i:]
		end := oscEnd(data)
		if end < 0 {
			if len(data) > maxCarry {
				return len(p), w.flush(data)
			}
			w.carry = append([]byte(nil), data...)
			return len(p), nil
		}
		seq := data[:end]
		data = data[end:]
		if !w.intercept(seq) {
			if err := w.flush(seq); err != nil {
				return 0, err
			}
		}
	}
	return len(p), nil
}

func (w *Writer) flush(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	_, err := w.out.Write(b)
	return err
}

// oscEnd returns the length of the OSC starting at data[0], including its BEL
// or ST terminator, -1 when more bytes are needed, or the index of a stray ESC
// that ends an unterminated sequence.
func oscEnd(data []byte) int {
	for j := 2; j < len(data); j++ {
		switch data[j] {
		case 0x07:
			return j + 1
		case 0x1b:
			if j+1 >= len(data) {
				return -1
			}
			if data[j+1] == '\\' {
				return j + 2
			}
			return j
		}
	}
	return -1
}

func (w *Writer) intercept(seq []byte) bool {
	if !w.hosting() || !bytes.HasPrefix(seq, []byte("\x1b]7501;")) {
		return false
	}
	if !bytes.HasSuffix(seq, []byte("\a")) && !bytes.HasSuffix(seq, []byte("\x1b\\")) {
		return false // unterminated: not ours to interpret
	}
	s := string(seq)
	if actions.IsProgramStatusQuery(s) {
		w.push(event{query: true})
		return true
	}
	if !strings.HasPrefix(s, "\x1b]7501;state=") && !strings.HasPrefix(s, "\x1b]7501;id=") {
		return false
	}
	r, err := actions.DecodeProgramStatus(s)
	w.push(event{report: r, err: err})
	return true
}

func (w *Writer) push(e event) {
	w.mu.Lock()
	w.pending = append(w.pending, e)
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *Writer) drain() []event {
	w.mu.Lock()
	defer w.mu.Unlock()
	events := w.pending
	w.pending = nil
	return events
}
