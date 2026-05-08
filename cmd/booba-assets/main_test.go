package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunCopiesViteChunks(t *testing.T) {
	out := t.TempDir()
	if err := run(out, false); err != nil {
		t.Fatal(err)
	}

	// Existing assets present
	for _, name := range []string{"ghostty-web/ghostty-web.js", "ghostty-web/ghostty-vt.wasm"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	// Every __vite- reference in ghostty-web.js has a matching shipped file
	js, err := os.ReadFile(filepath.Join(out, "ghostty-web/ghostty-web.js"))
	if err != nil {
		t.Fatal(err)
	}

	re := regexp.MustCompile(`__vite-[A-Za-z0-9.\-]+`)
	for _, ref := range re.FindAllString(string(js), -1) {
		if _, err := os.Stat(filepath.Join(out, "ghostty-web", ref)); err != nil {
			t.Errorf("ghostty-web.js references %s but it isn't shipped", ref)
		}
	}

	// Verify referenced Vite chunks are git-tracked (not gitignored).
	// This catches packaging gaps where .gitignore excludes Vite chunks without
	// whitelisting them, causing go get to skip them.
	gitOut, err := exec.Command("git", "ls-files", "serve/static/ghostty-web/").Output()
	if err != nil {
		t.Fatalf("git ls-files failed: %v", err)
	}
	tracked := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(gitOut)), "\n") {
		if line != "" {
			tracked[filepath.Base(line)] = true
		}
	}
	for _, ref := range re.FindAllString(string(js), -1) {
		if !tracked[ref] {
			t.Errorf("ghostty-web.js references %s but it isn't git-tracked (check .gitignore whitelist)", ref)
		}
	}
}
