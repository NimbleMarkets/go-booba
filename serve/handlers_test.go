//go:build !js

package serve

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/NimbleMarkets/go-booba/sip"
)

// SE-9: unknown message types must be logged when Debug is enabled so
// forward-compatibility silence doesn't hide protocol regressions during
// development. When Debug is off, the default branch remains silent.
const unknownMsgType byte = 0xFE

func TestProcessMessageLogsUnknownTypeWhenDebug(t *testing.T) {
	buf := captureLog(t)
	sess := &writeTrackingSession{Session: &resizeTestSession{}}
	processMessage(context.Background(), nil, sess, sip.OptionsMessage{}, unknownMsgType, []byte("x"), true, Config{}, func(WindowSize) {})

	if !strings.Contains(buf.String(), "unknown") {
		t.Errorf("log output = %q; want it to mention 'unknown'", buf.String())
	}
	if !strings.Contains(buf.String(), "0xfe") && !strings.Contains(buf.String(), "254") {
		t.Errorf("log output = %q; want it to include the unknown message type byte (0xfe / 254)", buf.String())
	}
}

func TestProcessMessageSilentOnUnknownTypeWhenDebugDisabled(t *testing.T) {
	buf := captureLog(t)
	sess := &writeTrackingSession{Session: &resizeTestSession{}}
	processMessage(context.Background(), nil, sess, sip.OptionsMessage{}, unknownMsgType, []byte("x"), false, Config{}, func(WindowSize) {})

	if buf.Len() != 0 {
		t.Errorf("log output = %q; want empty when debug=false", buf.String())
	}
}

func TestProcessWTMessageLogsUnknownTypeWhenDebug(t *testing.T) {
	buf := captureLog(t)
	sess := &writeTrackingSession{Session: &resizeTestSession{}}
	processWTMessage(context.Background(), nil, sess, sip.OptionsMessage{}, unknownMsgType, []byte("x"), true, Config{}, func(WindowSize) {})

	if !strings.Contains(buf.String(), "unknown") {
		t.Errorf("log output = %q; want it to mention 'unknown'", buf.String())
	}
}

// captureLog redirects the stdlib log package to a buffer for the duration
// of the test and restores it on cleanup.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prev)
		log.SetFlags(prevFlags)
	})
	return &buf
}
