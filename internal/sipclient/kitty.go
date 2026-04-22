package sipclient

import (
	"fmt"
	"io"
	"strconv"
	"time"
)

// QueryKittyFlags writes the "CSI ? u" query to w and reads the response
// (CSI ? <n> u) from r, with a short timeout. Returns the flags the local
// terminal reports supporting, and whether the terminal responded at all.
//
// The read runs in a goroutine so the timeout is enforced from the caller's
// thread without depending on r supporting a deadline.
func QueryKittyFlags(r io.Reader, w io.Writer, timeout time.Duration) (int, bool) {
	if _, err := fmt.Fprint(w, "\x1b[?u"); err != nil {
		return 0, false
	}
	type result struct {
		flags int
		ok    bool
	}
	resCh := make(chan result, 1)
	go func() {
		buf := make([]byte, 32)
		var accum []byte
		for {
			n, err := r.Read(buf)
			if n > 0 {
				accum = append(accum, buf[:n]...)
				if f, ok := parseKittyResponse(accum); ok {
					resCh <- result{f, true}
					return
				}
			}
			if err != nil {
				resCh <- result{0, false}
				return
			}
		}
	}()
	select {
	case res := <-resCh:
		return res.flags, res.ok
	case <-time.After(timeout):
		return 0, false
	}
}

// parseKittyResponse scans b for the first CSI ? <n> u sequence and returns n.
// It tolerates stray bytes before the ESC (e.g., a user pressed a key while
// the query was in flight).
func parseKittyResponse(b []byte) (int, bool) {
	for i := 0; i < len(b)-3; i++ {
		if b[i] != 0x1b || b[i+1] != '[' || b[i+2] != '?' {
			continue
		}
		j := i + 3
		start := j
		for j < len(b) && b[j] >= '0' && b[j] <= '9' {
			j++
		}
		if j == start || j >= len(b) || b[j] != 'u' {
			continue
		}
		n, err := strconv.Atoi(string(b[start:j]))
		if err != nil {
			continue
		}
		return n, true
	}
	return 0, false
}

// PushKittyFlags emits the "CSI > <flags> u" sequence to push flags onto the
// terminal's Kitty stack. Pair with PopKittyFlags on shutdown.
func PushKittyFlags(w io.Writer, flags int) error {
	_, err := fmt.Fprintf(w, "\x1b[>%du", flags)
	return err
}

// PopKittyFlags emits "CSI < u" to pop the most recently pushed flags.
func PopKittyFlags(w io.Writer) error {
	_, err := fmt.Fprint(w, "\x1b[<u")
	return err
}
