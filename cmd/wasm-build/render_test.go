package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testManifest() *Manifest {
	return &Manifest{Groups: []Group{
		{Title: "Bubble Tea in the browser", Demos: []Demo{
			{Name: "views", Title: "Multiple views", Blurb: "Press <kbd>Enter</kbd>", Source: "./cmd/booba-view-example"},
		}},
		{Title: "Terminal Actions (experimental)", Demos: []Demo{
			{Name: "actions", Title: "Touch controls", Blurb: "Buttons", Source: "./cmd/booba-actions-example", Page: "actions"},
		}},
	}}
}

func TestRenderProducesAllPages(t *testing.T) {
	out := t.TempDir()
	if err := renderSite(testManifest(), "../../web/_templates", out); err != nil {
		t.Fatalf("renderSite: %v", err)
	}

	index := filepath.Join(out, "index.html")
	mustContain(t, index,
		"Multiple views", "Touch controls",
		`href="/go-booba/demos/views/"`, `href="/go-booba/style.css"`,
		"Press <kbd>Enter</kbd>",
		"Serving a program", "github.com/NimbleMarkets/go-booba-example")

	views := filepath.Join(out, "demos", "views", "index.html")
	mustContain(t, views,
		"Multiple views", "sidebar-link-active",
		"https://github.com/NimbleMarkets/go-booba/tree/main/cmd/booba-view-example",
		"go run ./cmd/booba-view-example",
		`src="/go-booba/_assets/wasm_exec.js"`, `_assets/booba/booba.js`,
		`fetch("./app.wasm")`, "connectWasm()",
		`getElementById("terminal")`, "installStatusBar", "demo-status",
		`id="restart"`, "compileStreaming", "WebAssembly.instantiate(module",
		`booba.input("\x03")`, "booba.disconnect()", "booba.reset()")
	mustNotContain(t, views, "demo-controls", "installActionBar", "instantiateStreaming")

	actions := filepath.Join(out, "demos", "actions", "index.html")
	mustContain(t, actions,
		"sidebar-link-active", `id="controls"`, "demo-controls",
		"installActionBar", "allowOSC7501Actions", "setActionKeyEncoder", "supportsProgramStatus")
	mustContain(t, actions, "installStatusBar", `id="restart"`)
	mustNotContain(t, actions, "booba.onProgramStatus = report")
}

func TestRenderFragmentScriptPrecedesMainScript(t *testing.T) {
	out := t.TempDir()
	if err := renderSite(testManifest(), "../../web/_templates", out); err != nil {
		t.Fatalf("renderSite: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(out, "demos", "actions", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	frag := strings.Index(page, "window.demoOptions = {") // only the fragment assigns it
	main := strings.Index(page, "connectWasm()")
	if frag < 0 || main < 0 || frag > main {
		t.Fatalf("fragment script (at %d) must come before the main module script (at %d)", frag, main)
	}
}

func TestValidatePagesRejectsMissingFragment(t *testing.T) {
	m := &Manifest{Groups: []Group{{Title: "A", Demos: []Demo{
		{Name: "x", Title: "X", Source: "./cmd/booba-view-example", Page: "nope"},
	}}}}
	err := validatePages(m, "../../web/_templates")
	if err == nil || !strings.Contains(err.Error(), `"x"`) || !strings.Contains(err.Error(), "nope.html.tmpl") {
		t.Fatalf("want error naming demo and fragment file, got %v", err)
	}
	if err := validatePages(testManifest(), "../../web/_templates"); err != nil {
		t.Fatalf("repo fragments should validate: %v", err)
	}
}

func mustContain(t *testing.T, path string, needles ...string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, n := range needles {
		if !strings.Contains(string(b), n) {
			t.Errorf("%s missing %q", path, n)
		}
	}
}

func mustNotContain(t *testing.T, path string, needles ...string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, n := range needles {
		if strings.Contains(string(b), n) {
			t.Errorf("%s unexpectedly contains %q", path, n)
		}
	}
}
