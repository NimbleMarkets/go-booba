//go:build !js

package serve

import "testing"

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
