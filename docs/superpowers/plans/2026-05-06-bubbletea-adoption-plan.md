# Bubbletea Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the booba-wasm-build monkeypatch tool with a proper local dependency on the nm-wasm bubbletea fork, simplifying the WASM build pipeline.

**Architecture:** Add a local `replace` directive in `go.mod` pointing to `../bubbletea`, then remove the custom build tool and update build configs to use standard `go build` with WASM environment variables.

**Tech Stack:** Go modules, Taskfile, goreleaser

---

### Task 1: Add Replace Directive to go.mod

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Open go.mod and add the replace directive**

At the end of the `go.mod` file (after the `require` and `indirect` sections), add:

```
replace charm.land/bubbletea/v2 => ../bubbletea
```

The file should look like:
```go
module github.com/NimbleMarkets/go-booba

go 1.25.0

require (
	charm.land/bubbletea/v2 v2.0.6
	charm.land/lipgloss/v2 v2.0.3
	// ... other requires
)

require (
	// ... indirect dependencies
)

replace charm.land/bubbletea/v2 => ../bubbletea
```

- [ ] **Step 2: Run go mod tidy to verify the replace works**

```bash
go mod tidy
```

Expected: Command completes without error. No changes to go.mod or go.sum (the replace is local).

- [ ] **Step 3: Verify bubbletea is loadable**

```bash
go list -m charm.land/bubbletea/v2
```

Expected: Output shows `charm.land/bubbletea/v2 v2.0.6 => ../bubbletea`

- [ ] **Step 4: Commit**

```bash
git add go.mod
git commit -m "feat(deps): add local replace for patched bubbletea (nm-wasm)"
```

---

### Task 2: Remove booba-wasm-build Command Directory

**Files:**
- Delete: `cmd/booba-wasm-build/` (entire directory)

- [ ] **Step 1: List the directory to confirm what we're removing**

```bash
ls -la cmd/booba-wasm-build/
```

Expected: Shows `main.go`, `_stubs/` subdirectory, and other files.

- [ ] **Step 2: Remove the entire directory**

```bash
rm -rf cmd/booba-wasm-build/
```

- [ ] **Step 3: Verify it's gone**

```bash
ls cmd/booba-wasm-build/ 2>&1
```

Expected: `ls: cannot access 'cmd/booba-wasm-build/': No such file or directory`

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore: remove booba-wasm-build tool (replaced by patched bubbletea dependency)"
```

---

### Task 3: Update Taskfile.yml - Remove Build Task

**Files:**
- Modify: `Taskfile.yml` (around lines containing `build-cmd-booba-wasm-build`)

- [ ] **Step 1: Find and view the build-cmd-booba-wasm-build task**

```bash
grep -n "build-cmd-booba-wasm-build" Taskfile.yml
```

Expected: Shows 2-3 line numbers where this task appears.

- [ ] **Step 2: Open Taskfile.yml and find the task definition**

Search for `build-cmd-booba-wasm-build:` task. It should look like:

```yaml
  build-cmd-booba-wasm-build:
    desc: 'Build booba-wasm-build command'
    generates:
      - bin/booba-wasm-build
    sources:
      - cmd/booba-wasm-build/**/*
    cmds:
      - go build -o bin/booba-wasm-build ./cmd/booba-wasm-build/
```

Remove this entire task definition (typically 7-8 lines).

- [ ] **Step 3: Remove dependency from other tasks**

Find the `build-wasm-example` or similar task that has `deps: [go-tidy, build-cmd-booba-wasm-build]` and remove `build-cmd-booba-wasm-build` from the deps list. It should now be just `deps: [go-tidy]`.

Also find any task that references `bin/booba-wasm-build` as a source and remove those lines.

- [ ] **Step 4: Run task list to verify structure**

```bash
task --list | grep -i wasm
```

Expected: Shows wasm-related tasks but no reference to `build-cmd-booba-wasm-build`.

- [ ] **Step 5: Commit**

```bash
git add Taskfile.yml
git commit -m "chore(task): remove booba-wasm-build build task"
```

---

### Task 4: Update Taskfile.yml - Simplify WASM Build Commands

**Files:**
- Modify: `Taskfile.yml` (WASM-related task cmds, likely `build-wasm-example` or similar)

- [ ] **Step 1: Find WASM build tasks**

```bash
grep -A 5 "build.*wasm\|wasm.*build" Taskfile.yml | head -30
```

Expected: Shows tasks that invoke `bin/booba-wasm-build`.

- [ ] **Step 2: Update the example build task**

Find the task that builds the wasm example (likely `build-wasm-example` or `booba-view-example.wasm`). Change:

```yaml
# OLD:
cmds:
  - bin/booba-wasm-build -o bin/booba-view-example.wasm ./cmd/booba-view-example/
```

To:

```yaml
# NEW:
cmds:
  - GOOS=js GOARCH=wasm go build -o bin/booba-view-example.wasm ./cmd/booba-view-example/
```

- [ ] **Step 3: Remove any artifact cleanup tasks that reference bin/booba-wasm-build**

Search for tasks that have `rm -f bin/booba-wasm-build`. Remove those lines or the entire `cmds:` section if that's the only command.

- [ ] **Step 4: Verify Taskfile syntax**

```bash
task --list 2>&1 | head -20
```

Expected: No errors, task list displays successfully.

- [ ] **Step 5: Commit**

```bash
git add Taskfile.yml
git commit -m "chore(task): simplify wasm build to use go build directly"
```

---

### Task 5: Update .goreleaser.yml - Remove Binary Build Config

**Files:**
- Modify: `.goreleaser.yml` (builds section)

- [ ] **Step 1: Locate the booba-wasm-build entry**

```bash
grep -n "booba-wasm-build" .goreleaser.yml
```

Expected: Shows line numbers where this appears (likely 3-4 times for the build entry).

- [ ] **Step 2: View the builds section**

Open `.goreleaser.yml` and find the build entry that looks like:

```yaml
builds:
  - id: booba-wasm-build
    main: ./cmd/booba-wasm-build
    binary: booba-wasm-build
    goos: [linux, darwin]
    goarch: [amd64, arm64]
    # ... other config
```

Remove the entire `- id: booba-wasm-build` block (typically 6-10 lines).

- [ ] **Step 3: Find and remove from archives section**

Search for the archives section. If there's an entry like:

```yaml
- booba-wasm-build
```

under some archive/build filter, remove it.

- [ ] **Step 4: Verify YAML syntax**

```bash
cat .goreleaser.yml | head -50
```

Expected: Valid YAML structure (check indentation visually).

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yml
git commit -m "chore(release): remove booba-wasm-build from goreleaser config"
```

---

### Task 6: Test WASM Build Manually

**Files:**
- No files modified (manual test)

- [ ] **Step 1: Clean any stale build artifacts**

```bash
rm -f bin/booba-view-example.wasm
```

- [ ] **Step 2: Build the WASM example**

```bash
GOOS=js GOARCH=wasm go build -o bin/booba-view-example.wasm ./cmd/booba-view-example/
```

Expected: Completes in 10-30 seconds without error. File is created.

- [ ] **Step 3: Verify the output file exists and has reasonable size**

```bash
ls -lh bin/booba-view-example.wasm && file bin/booba-view-example.wasm
```

Expected: File is >500KB, `file` output shows WebAssembly.

- [ ] **Step 4: Verify no lingering references to booba-wasm-build in source**

```bash
grep -r "booba-wasm-build" --include="*.go" --include="*.yml" --include="*.yaml" . 2>/dev/null | grep -v ".git"
```

Expected: No results or only in comments/docs that are okay to keep.

- [ ] **Step 5: Commit** (if any cleanup files modified)

```bash
git status
```

Expected: No uncommitted changes (test artifacts are .gitignored).

---

### Task 7: Verify Full Build and Test Suite

**Files:**
- No files modified (verification only)

- [ ] **Step 1: Run go vet on the entire project**

```bash
go vet ./...
```

Expected: No errors.

- [ ] **Step 2: Run go test**

```bash
go test ./... -v
```

Expected: All tests pass (or match existing baseline if there are known failures).

- [ ] **Step 3: Verify booba binary still builds**

```bash
GOOS=js GOARCH=wasm go build -o /tmp/test.wasm ./cmd/booba-view-example/
```

Expected: Builds successfully.

- [ ] **Step 4: Check git log to verify commits are clean**

```bash
git log --oneline -7
```

Expected: Shows 5-6 commits for this feature with clear messages.

- [ ] **Step 5: Final status check**

```bash
git status
```

Expected: `On branch nm-bubbletea-wasm` with no uncommitted changes.
