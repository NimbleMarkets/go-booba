//go:build !js

package serve

import (
	"regexp"
	"testing"
)

func TestEmbeddedStaticAssetsPresent(t *testing.T) {
	required := []string{
		"static/index.html",
		"static/booba/booba.js",
		"static/ghostty-web/ghostty-web.js",
		"static/ghostty-web/ghostty-vt.wasm",
	}

	for _, name := range required {
		if _, err := staticFiles.ReadFile(name); err != nil {
			t.Fatalf("embedded asset %q missing: %v", name, err)
		}
	}
}

// TestEmbeddedViteChunksResolve guards against the go:embed underscore
// exclusion: Vite emits lazily-imported chunks named "__vite-*.js", and a
// plain "static/*" embed pattern silently drops them (go:embed skips
// "_"-prefixed files without the all: prefix). The browser does fetch
// these at runtime, so a missing one is a console 404. Every "__vite-"
// reference in ghostty-web.js must be embedded.
func TestEmbeddedViteChunksResolve(t *testing.T) {
	js, err := staticFiles.ReadFile("static/ghostty-web/ghostty-web.js")
	if err != nil {
		t.Fatalf("read embedded ghostty-web.js: %v", err)
	}

	re := regexp.MustCompile(`__vite-[A-Za-z0-9.\-]+`)
	for _, ref := range re.FindAllString(string(js), -1) {
		if _, err := staticFiles.ReadFile("static/ghostty-web/" + ref); err != nil {
			t.Errorf("ghostty-web.js references %s but it is not embedded "+
				"(check the go:embed all: prefix in server.go): %v", ref, err)
		}
	}
}
