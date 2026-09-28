//go:build !js

package serve

import (
	"io/fs"
	"path"
	"regexp"
	"testing"

	assets "github.com/NimbleMarkets/go-booba/serve/static"
)

func TestEmbeddedAssetsContainOnlyRuntimeFiles(t *testing.T) {
	serverAssets, err := fs.Sub(staticFiles, "static")
	if err != nil {
		t.Fatal(err)
	}
	for name, files := range map[string]fs.FS{"server": serverAssets, "scaffolder": assets.FS} {
		t.Run(name, func(t *testing.T) {
			err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() && name != "index.html" && path.Ext(name) != ".js" && path.Ext(name) != ".wasm" {
					t.Errorf("non-runtime asset embedded: %s", name)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

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

// TestEmbeddedViteChunksResolve guards against accidentally excluding Vite's
// underscore-prefixed chunks. The browser fetches these at runtime, so every
// "__vite-" reference in ghostty-web.js must be embedded.
func TestEmbeddedViteChunksResolve(t *testing.T) {
	js, err := staticFiles.ReadFile("static/ghostty-web/ghostty-web.js")
	if err != nil {
		t.Fatalf("read embedded ghostty-web.js: %v", err)
	}

	re := regexp.MustCompile(`__vite-[A-Za-z0-9.\-]+`)
	for _, ref := range re.FindAllString(string(js), -1) {
		if _, err := staticFiles.ReadFile("static/ghostty-web/" + ref); err != nil {
			t.Errorf("ghostty-web.js references %s but it is not embedded "+
				"(check the go:embed patterns in server.go): %v", ref, err)
		}
	}
}
