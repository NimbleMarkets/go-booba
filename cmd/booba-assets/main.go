// booba-assets sets up a web/ directory with everything needed to host
// a BubbleTea program compiled to WebAssembly.
//
// It writes:
//   - wasm_exec.js  — copied from GOROOT (Go's WASM runtime shim)
//   - booba/*.js    — the terminal wrapper (embedded in this binary)
//   - ghostty-web/* — the ghostty-web terminal emulator (embedded)
//   - index.html    — a starter template (embedded), unless one exists
//
// Every asset except wasm_exec.js is embedded at compile time via the
// go-booba serve/static package, so the tool needs no module context,
// network access, or particular working directory — it runs anywhere.
// wasm_exec.js is copied from the active Go toolchain because it must
// match the compiler that builds the user's app.wasm.
//
// Usage:
//
//	booba-assets [--force] <output-dir>
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/pflag"

	"github.com/NimbleMarkets/go-booba/serve/static"
)

//go:embed template/index.html
var templateFS embed.FS

func main() {
	force := pflag.BoolP("force", "f", false, "overwrite an existing index.html")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--force] <output-dir>\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Populates <output-dir> with wasm_exec.js, booba/, ghostty-web/,")
		fmt.Fprintln(os.Stderr, "and a starter index.html for hosting a BubbleTea WASM program.")
		pflag.PrintDefaults()
	}
	pflag.Parse()

	if pflag.NArg() != 1 {
		pflag.Usage()
		os.Exit(2)
	}
	if err := run(pflag.Arg(0), *force); err != nil {
		fmt.Fprintf(os.Stderr, "booba-assets: %v\n", err)
		os.Exit(1)
	}
}

func run(outDir string, force bool) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// wasm_exec.js — copied from the active Go toolchain so it matches
	// the compiler that builds the user's app.wasm.
	if err := copyWasmExec(outDir); err != nil {
		return err
	}

	// booba/*.js (terminal wrapper) — embedded
	boobaDst := filepath.Join(outDir, "booba")
	n, err := copyEmbedded("booba", boobaDst, isJS)
	if err != nil {
		return fmt.Errorf("write booba assets: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no embedded booba/*.js assets found — booba-assets build is corrupt")
	}
	fmt.Printf("  booba/ (%d files)      → %s\n", n, boobaDst)

	// ghostty-web (terminal emulator) — embedded
	ghDst := filepath.Join(outDir, "ghostty-web")
	n, err = copyEmbedded("ghostty-web", ghDst, isBrowserAsset)
	if err != nil {
		return fmt.Errorf("write ghostty-web assets: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no embedded ghostty-web assets found — booba-assets build is corrupt")
	}
	// Sanity check: ensure all __vite- references are resolved.
	if err := checkViteReferences(filepath.Join(ghDst, "ghostty-web.js"), ghDst); err != nil {
		return fmt.Errorf("ghostty-web.js contains unresolved Vite references: %w", err)
	}
	fmt.Printf("  ghostty-web/ (%d files) → %s\n", n, ghDst)

	// index.html (only if missing or --force)
	htmlDst := filepath.Join(outDir, "index.html")
	if _, err := os.Stat(htmlDst); err == nil && !force {
		fmt.Printf("  index.html exists, skipped (use --force to overwrite)\n")
	} else {
		html, err := templateFS.ReadFile("template/index.html")
		if err != nil {
			return err
		}
		if err := os.WriteFile(htmlDst, html, 0o644); err != nil {
			return fmt.Errorf("write index.html: %w", err)
		}
		fmt.Printf("  index.html            → %s\n", htmlDst)
	}

	return nil
}

// copyWasmExec copies wasm_exec.js from the active Go toolchain's GOROOT
// into outDir. It must come from the toolchain (not an embedded copy) so
// it matches the compiler that builds the user's app.wasm.
func copyWasmExec(outDir string) error {
	goroot, err := goEnv("GOROOT")
	if err != nil {
		return fmt.Errorf("locate GOROOT: %w", err)
	}
	src := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read wasm_exec.js (expected at %s): %w", src, err)
	}
	dst := filepath.Join(outDir, "wasm_exec.js")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write wasm_exec.js: %w", err)
	}
	fmt.Printf("  wasm_exec.js          → %s\n", dst)
	return nil
}

// copyEmbedded writes every file in the embedded directory subdir for
// which keep(name) reports true into dstDir, returning the count written.
func copyEmbedded(subdir, dstDir string, keep func(name string) bool) (int, error) {
	entries, err := fs.ReadDir(static.FS, subdir)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !keep(e.Name()) {
			continue
		}
		data, err := static.FS.ReadFile(path.Join(subdir, e.Name()))
		if err != nil {
			return count, err
		}
		if err := os.WriteFile(filepath.Join(dstDir, e.Name()), data, 0o644); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func isJS(name string) bool { return strings.HasSuffix(name, ".js") }

// isBrowserAsset keeps the runtime .js and .wasm files, skipping type
// definitions, source maps, and the UMD bundle.
func isBrowserAsset(name string) bool {
	if strings.HasSuffix(name, ".d.ts") ||
		strings.HasSuffix(name, ".d.ts.map") ||
		strings.HasSuffix(name, ".js.map") ||
		strings.HasSuffix(name, ".cjs") {
		return false
	}
	return strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".wasm")
}

func goEnv(name string) (string, error) {
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// checkViteReferences verifies that all __vite- references in jsFile are
// resolved (i.e., the referenced files exist in dstDir). This catches
// regressions where Vite-generated filenames change after a rebuild.
func checkViteReferences(jsFile, dstDir string) error {
	content, err := os.ReadFile(jsFile)
	if err != nil {
		return err
	}

	viteRe := regexp.MustCompile(`__vite-[A-Za-z0-9.\-]+`)
	for _, filename := range viteRe.FindAllString(string(content), -1) {
		if _, err := os.Stat(filepath.Join(dstDir, filename)); err != nil {
			return fmt.Errorf("referenced file not found: %s", filename)
		}
	}
	return nil
}
