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
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed template/index.html
var templateFS embed.FS

const boobaModule = "github.com/NimbleMarkets/go-booba"

func main() {
	force := flag.Bool("force", false, "overwrite an existing index.html")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--force] <output-dir>\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Populates <output-dir> with wasm_exec.js, booba/, ghostty-web/,")
		fmt.Fprintln(os.Stderr, "and a starter index.html for hosting a BubbleTea WASM program.")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *force); err != nil {
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
	for _, name := range []string{"ghostty-web.js", "ghostty-vt.wasm"} {
		src := filepath.Join(ghSrc, name)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("ghostty-web asset missing at %s: %w", src, err)
		}
		if err := copyFile(src, filepath.Join(ghDst, name)); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}
	fmt.Printf("  ghostty-web/          → %s\n", ghDst)

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
	out, err := os.Create(dst)
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
