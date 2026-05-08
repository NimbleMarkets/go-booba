// booba-assets sets up a web/ directory with everything needed to host
// a BubbleTea program compiled to WebAssembly.
//
// It copies:
//   - wasm_exec.js from GOROOT (Go WASM runtime support)
//   - booba/*.js from the go-booba module (terminal wrapper)
//   - ghostty-web/ghostty-web.js and ghostty-vt.wasm (terminal emulator)
//   - index.html (an embedded starter template, unless one already exists)
//
// Usage:
//
//	go run github.com/NimbleMarkets/go-booba/cmd/booba-assets [--force] <output-dir>
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
)

//go:embed template/index.html
var templateFS embed.FS

const boobaModule = "github.com/NimbleMarkets/go-booba"

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

	boobaDir, err := findModuleDir(boobaModule)
	if err != nil {
		return fmt.Errorf("locate %s (run 'go mod download' first?): %w", boobaModule, err)
	}

	goroot, err := goEnv("GOROOT")
	if err != nil {
		return fmt.Errorf("locate GOROOT: %w", err)
	}

	// wasm_exec.js (Go runtime)
	wasmExec := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
	if _, err := os.Stat(wasmExec); err != nil {
		return fmt.Errorf("wasm_exec.js not found at %s: %w", wasmExec, err)
	}
	if err := copyFile(wasmExec, filepath.Join(outDir, "wasm_exec.js")); err != nil {
		return fmt.Errorf("copy wasm_exec.js: %w", err)
	}
	fmt.Printf("  wasm_exec.js          → %s\n", filepath.Join(outDir, "wasm_exec.js"))

	// booba/*.js (terminal wrapper)
	boobaSrc := filepath.Join(boobaDir, "serve", "static", "booba")
	boobaDst := filepath.Join(outDir, "booba")
	if err := os.MkdirAll(boobaDst, 0o755); err != nil {
		return err
	}
	n, err := copyJSFiles(boobaSrc, boobaDst)
	if err != nil {
		return fmt.Errorf("copy booba assets: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no .js files found in %s — module may be incomplete", boobaSrc)
	}
	fmt.Printf("  booba/ (%d files)      → %s\n", n, boobaDst)

	// ghostty-web (terminal emulator)
	ghSrc := filepath.Join(boobaDir, "serve", "static", "ghostty-web")
	ghDst := filepath.Join(outDir, "ghostty-web")
	if err := os.MkdirAll(ghDst, 0o755); err != nil {
		return err
	}
	n, err = copyBrowserAssets(ghSrc, ghDst)
	if err != nil {
		return fmt.Errorf("copy ghostty-web assets: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no browser assets found in %s", ghSrc)
	}
	// Sanity check: ensure all __vite- references are resolved
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

func findModuleDir(path string) (string, error) {
	out, err := exec.Command("go", "list", "-m", "-json", path).Output()
	if err != nil {
		return "", err
	}
	var info struct {
		Dir string
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", err
	}
	if info.Dir == "" {
		return "", fmt.Errorf("module %s not in module cache", path)
	}
	return info.Dir, nil
}

func goEnv(name string) (string, error) {
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	// Normalize to 0o644: assets come from the Go module cache, which can
	// hold files with 0o755 or other perms we don't want to propagate into
	// the user's web directory.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func copyJSFiles(srcDir, dstDir string) (int, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		if err := copyFile(src, dst); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// copyBrowserAssets copies all .js and .wasm files from srcDir to dstDir,
// skipping type definitions, source maps, and UMD bundles.
func copyBrowserAssets(srcDir, dstDir string) (int, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip type definitions, source maps, and UMD bundles
		if strings.HasSuffix(name, ".d.ts") ||
			strings.HasSuffix(name, ".d.ts.map") ||
			strings.HasSuffix(name, ".js.map") ||
			strings.HasSuffix(name, ".cjs") {
			continue
		}
		// Include only .js and .wasm files
		if !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".wasm") {
			continue
		}
		src := filepath.Join(srcDir, name)
		dst := filepath.Join(dstDir, name)
		if err := copyFile(src, dst); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// checkViteReferences verifies that all __vite- references in jsFile are resolved
// (i.e., the referenced files exist in dstDir). This catches regressions where
// Vite-generated filenames change after a rebuild.
func checkViteReferences(jsFile, dstDir string) error {
	content, err := os.ReadFile(jsFile)
	if err != nil {
		return err
	}
	jsText := string(content)

	// Find all __vite- references
	viteRe := regexp.MustCompile(`__vite-[A-Za-z0-9.\-]+`)
	matches := viteRe.FindAllString(jsText, -1)

	for _, filename := range matches {
		checkPath := filepath.Join(dstDir, filename)
		if _, err := os.Stat(checkPath); err != nil {
			return fmt.Errorf("referenced file not found: %s", filename)
		}
	}
	return nil
}
