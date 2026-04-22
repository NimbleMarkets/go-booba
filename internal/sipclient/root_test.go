package sipclient

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestExecute_MissingURL(t *testing.T) {
	cmd := newRootCmd()
	var stderr bytes.Buffer
	cmd.SetArgs([]string{})
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q; want it to mention 'url'", err.Error())
	}
}

func TestExecute_AcceptsWSURL(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetArgs([]string{"--dump-frames", "--dump-timeout", "1ms", "ws://127.0.0.1:1/ws"})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Expect a dial failure, NOT a "url" validation error.
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatalf("want dial error, got nil")
	}
	if strings.Contains(err.Error(), "url is required") || strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("url validation rejected a valid URL: %v", err)
	}
}
