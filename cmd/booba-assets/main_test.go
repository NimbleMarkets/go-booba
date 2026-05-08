package main

import (
	"os"
	"path/filepath"
	"regexp"
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

	// Every __vite- reference in ghostty-web.js has a matching shipped file.
	// This catches regressions where Vite-generated filenames change but the
	// .gitignore whitelist wasn't updated or the files weren't committed.
	js, err := os.ReadFile(filepath.Join(out, "ghostty-web/ghostty-web.js"))
	if err != nil {
		t.Fatal(err)
	}

	re := regexp.MustCompile(`__vite-[A-Za-z0-9.\-]+`)
	for _, ref := range re.FindAllString(string(js), -1) {
		if _, err := os.Stat(filepath.Join(out, "ghostty-web", ref)); err != nil {
			t.Errorf("ghostty-web.js references %s but it isn't shipped (check .gitignore whitelist and make sure Vite chunks are committed)", ref)
		}
	}
}
