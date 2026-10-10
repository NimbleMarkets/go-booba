package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCommand(t *testing.T) {
	d := Demo{Name: "views", Source: "./cmd/booba-view-example"}
	name, args := buildCommand(d, "web/demos/views/app.wasm")
	if name != "go" || strings.Join(args, " ") != "build -o web/demos/views/app.wasm ./cmd/booba-view-example" {
		t.Errorf("got %s %v", name, args)
	}
}

func TestValidateSourcesRejectsMissingDirectory(t *testing.T) {
	m := &Manifest{Groups: []Group{{Title: "A", Demos: []Demo{
		{Name: "views", Source: "./cmd/booba-view-example"},
		{Name: "ghost", Source: "./cmd/does-not-exist"},
	}}}}
	// Tests run inside cmd/wasm-build; sources are relative to the repo root.
	chdirRepoRoot(t)
	err := validateSources(m)
	if err == nil || !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "./cmd/does-not-exist") {
		t.Fatalf("want error naming the demo and its source, got %v", err)
	}
}

func TestValidateSourcesAcceptsRepoManifest(t *testing.T) {
	chdirRepoRoot(t)
	m, err := loadManifestFile("web/demos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSources(m); err != nil {
		t.Fatal(err)
	}
}

// TestCompileAndAssetsEndToEnd builds the real demos for js/wasm and runs
// booba-assets. It is the only test that invokes the Go toolchain.
func TestCompileAndAssetsEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles WASM; skipped in -short mode")
	}
	progress = io.Discard
	t.Cleanup(func() { progress = os.Stdout })
	chdirRepoRoot(t)
	m, err := loadManifestFile("web/demos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := compileAll(m, out); err != nil {
		t.Fatalf("compileAll: %v", err)
	}
	for _, d := range m.AllDemos() {
		wasm := filepath.Join(out, "demos", d.Name, "app.wasm")
		head := make([]byte, 4)
		f, err := os.Open(wasm)
		if err != nil {
			t.Fatalf("%s: %v", wasm, err)
		}
		_, _ = f.Read(head)
		_ = f.Close()
		if string(head) != "\x00asm" {
			t.Errorf("%s: not a wasm binary (magic %q)", wasm, head)
		}
	}
	if err := copyAssets(out); err != nil {
		t.Fatalf("copyAssets: %v", err)
	}
	for _, p := range []string{"wasm_exec.js", "booba/booba.js", "ghostty-web/ghostty-web.js", "ghostty-web/ghostty-vt.wasm"} {
		if _, err := os.Stat(filepath.Join(out, "_assets", p)); err != nil {
			t.Errorf("missing asset %s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "_assets", "index.html")); !os.IsNotExist(err) {
		t.Errorf("booba-assets starter index.html must be removed from _assets, stat err = %v", err)
	}
}

func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(wd, "..", "..")
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}
