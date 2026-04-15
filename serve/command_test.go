//go:build !js && !windows

package serve

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunCommandStartsAttachedProcess(t *testing.T) {
	sess, err := newPtySession(context.Background(), WindowSize{Width: 80, Height: 24})
	if err != nil {
		t.Fatalf("newPtySession() error = %v", err)
	}
	defer sess.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runCommand(context.Background(), sess, "/bin/sh", "-lc", "printf hello-from-booba")
	}()

	buf := make([]byte, 256)
	deadline := time.After(3 * time.Second)

	for {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("runCommand() error = %v", err)
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for command output")
		default:
		}

		n, readErr := sess.OutputReader().Read(buf)
		if n > 0 && strings.Contains(string(buf[:n]), "hello-from-booba") {
			if err := <-errCh; err != nil {
				t.Fatalf("runCommand() error = %v", err)
			}
			return
		}
		if readErr != nil && readErr != io.EOF {
			t.Fatalf("PTY read error = %v", readErr)
		}
	}
}
