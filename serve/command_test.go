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

func TestRunCommandReceivesWinchOnResize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess, err := newPtySession(ctx, WindowSize{Width: 80, Height: 24})
	if err != nil {
		t.Fatalf("newPtySession() error = %v", err)
	}
	defer sess.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runCommand(ctx, sess, "/bin/sh", "-lc", `
trap 'printf "winch:%s\n" "$(stty size)"' WINCH
printf "ready\n"
while :; do sleep 1; done
`)
	}()

	buf := make([]byte, 256)
	deadline := time.After(5 * time.Second)
	sawReady := false

	for {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("runCommand() error = %v", err)
			}
			if !sawReady {
				t.Fatal("command exited before resize test completed")
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for WINCH output")
		default:
		}

		n, readErr := sess.OutputReader().Read(buf)
		if n > 0 {
			out := string(buf[:n])
			if !sawReady && strings.Contains(out, "ready") {
				sawReady = true
				sess.Resize(100, 40)
			}
			if sawReady && strings.Contains(out, "winch:40 100") {
				cancel()
				if err := <-errCh; err != nil {
					t.Fatalf("runCommand() error = %v", err)
				}
				return
			}
		}
		if readErr != nil && readErr != io.EOF {
			t.Fatalf("PTY read error = %v", readErr)
		}
	}
}
