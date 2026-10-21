package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// progress receives the generator's progress lines and the stdout of child
// processes (go build, booba-assets). Tests replace it to keep output quiet.
var progress io.Writer = os.Stdout

// buildCommand returns the compiler invocation for a demo.
func buildCommand(d Demo, dst string) (string, []string) {
	return "go", []string{"build", "-o", dst, d.Source}
}

// validateSources checks Go source directories and browser demo source files.
// The generator runs from the repository root, so sources resolve as written.
func validateSources(m *Manifest) error {
	for _, d := range m.AllDemos() {
		info, err := os.Stat(d.Source)
		if err != nil {
			return fmt.Errorf("demo %q: source %s: %w", d.Name, d.Source, err)
		}
		if d.BrowserOnly {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("demo %q: source %s is not a file", d.Name, d.Source)
			}
		} else if !info.IsDir() {
			return fmt.Errorf("demo %q: source %s is not a directory", d.Name, d.Source)
		}
	}
	return nil
}

// compileAll builds GOOS=js GOARCH=wasm binaries for Go demos, in parallel.
// Output: <outDir>/demos/<demo.Name>/app.wasm
func compileAll(m *Manifest, outDir string) error {
	demos := m.AllDemos()
	errs := make([]error, len(demos))
	var wg sync.WaitGroup
	for i, d := range demos {
		if d.BrowserOnly {
			continue
		}
		wg.Add(1)
		go func(i int, d Demo) {
			defer wg.Done()
			errs[i] = compileOne(d, outDir)
		}(i, d)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return fmt.Errorf("demo %q: %w", demos[i].Name, err)
		}
	}
	return nil
}

func compileOne(d Demo, outDir string) error {
	dst := filepath.Join(outDir, "demos", d.Name, "app.wasm")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	name, args := buildCommand(d, dst)
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout = progress
	cmd.Stderr = os.Stderr
	_, _ = fmt.Fprintf(progress, "[wasm-build] compiling %s -> %s\n", d.Source, dst)
	return cmd.Run()
}

// copyAssets populates <outDir>/_assets/ with the runtime embedded in this
// repository (wasm_exec.js, booba/, ghostty-web/) by running the local
// booba-assets command. Its starter index.html is removed: the site renders
// its own pages.
func copyAssets(outDir string) error {
	assetsDir := filepath.Join(outDir, "_assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "./cmd/booba-assets", "--force", assetsDir)
	cmd.Stdout = progress
	cmd.Stderr = os.Stderr
	_, _ = fmt.Fprintf(progress, "[wasm-build] copying booba assets into %s\n", assetsDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("booba-assets: %w", err)
	}
	if err := os.Remove(filepath.Join(assetsDir, "index.html")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
