package sipclient

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// SOLTracker tracks whether the next byte to be emitted is at the start of a
// line. A line break is either CR (\r) or LF (\n). The tracker is updated as
// bytes are observed (typically the bytes being forwarded to the server).
type SOLTracker struct {
	atStart bool
}

// NewSOLTracker returns a tracker initialized to AtStart=true, since a fresh
// connection begins on a new line.
func NewSOLTracker() *SOLTracker {
	return &SOLTracker{atStart: true}
}

// AtStart reports whether the next observed byte will be at start-of-line.
func (t *SOLTracker) AtStart() bool { return t.atStart }

// Observe updates the tracker with the given bytes. If the slice is empty, the
// state is unchanged.
func (t *SOLTracker) Observe(b []byte) {
	if len(b) == 0 {
		return
	}
	last := b[len(b)-1]
	t.atStart = last == '\r' || last == '\n'
}

// EscapeAction is returned by RunEscapePrompt to tell the caller what to do
// next.
type EscapeAction int

const (
	ActionContinue   EscapeAction = iota // resume the session
	ActionDisconnect                     // close and exit cleanly
)

// PromptInfo is the context shown by the "status" command.
type PromptInfo struct {
	URL           string
	Started       time.Time
	LastFrameTime time.Time
}

// RunEscapePrompt reads commands line-by-line from r, writing prompts and
// output to w, and returns when the user chooses to continue or disconnect.
// EOF on the input is treated as a disconnect (so closing stdin cleanly
// severs the session rather than hanging).
func RunEscapePrompt(r io.Reader, w io.Writer, info PromptInfo) (EscapeAction, error) {
	sc := bufio.NewScanner(r)
	sc.Split(scanLinesCROrLF)
	for {
		if _, err := fmt.Fprint(w, "booba-sip-client> "); err != nil {
			return ActionDisconnect, err
		}
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return ActionDisconnect, err
			}
			// EOF with no line read
			_, _ = fmt.Fprintln(w)
			return ActionDisconnect, nil
		}
		line := strings.TrimSpace(sc.Text())
		switch strings.ToLower(line) {
		case "", "continue", "c":
			return ActionContinue, nil
		case "quit", "exit", "q":
			return ActionDisconnect, nil
		case "status":
			printStatus(w, info)
		case "help", "h", "?":
			printHelp(w)
		default:
			_, _ = fmt.Fprintf(w, "unknown command %q\n", line)
			printHelp(w)
		}
	}
}

// scanLinesCROrLF is a bufio.SplitFunc that treats either '\r' or '\n' as
// a line terminator. This is necessary because RunEscapePrompt runs while
// the tty is in raw mode (where Enter produces '\r', not '\n'), but is
// also used in tests that feed '\n'-terminated input.
func scanLinesCROrLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\r' || b == '\n' {
			// Consume an optional trailing LF after CR for \r\n terminators.
			adv := i + 1
			if b == '\r' && adv < len(data) && data[adv] == '\n' {
				adv++
			}
			return adv, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	// Need more data.
	return 0, nil, nil
}

func printHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, "commands:")
	_, _ = fmt.Fprintln(w, "  quit | exit | q      disconnect and exit")
	_, _ = fmt.Fprintln(w, "  continue | c | <cr>  resume the session")
	_, _ = fmt.Fprintln(w, "  status               show connection info")
	_, _ = fmt.Fprintln(w, "  help | h | ?         show this help")
}

func printStatus(w io.Writer, info PromptInfo) {
	if info.URL != "" {
		_, _ = fmt.Fprintf(w, "url:       %s\n", info.URL)
	}
	if !info.Started.IsZero() {
		_, _ = fmt.Fprintf(w, "connected: %s ago\n", time.Since(info.Started).Truncate(time.Second))
	}
	if !info.LastFrameTime.IsZero() {
		_, _ = fmt.Fprintf(w, "last frame: %s ago\n", time.Since(info.LastFrameTime).Truncate(time.Millisecond))
	}
}
