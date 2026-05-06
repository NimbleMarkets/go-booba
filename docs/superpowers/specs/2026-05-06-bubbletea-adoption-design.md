---
name: Adopt Patched Bubbletea + Remove booba-wasm-build
description: Replace monkeypatch tool with proper nm-wasm fork dependency to simplify WASM build process
type: design
date: 2026-05-06
---

# Adopt Patched Bubbletea + Remove booba-wasm-build

## Goal

Replace the fragile `booba-wasm-build` monkeypatch tool with a proper dependency on the nm-wasm bubbletea fork, eliminating the need for runtime injection of WebAssembly stubs and simplifying the overall build pipeline.

## Context

Currently, booba works around the lack of WASM support in upstream bubbletea v2 by:
1. Copying bubbletea from the module cache at build time
2. Injecting WASM-specific stub files into the copy
3. Using a temporary `go.mod` replace directive to build against the patched version

This approach is brittle, adds build complexity, and requires a custom CLI tool (`booba-wasm-build`) that must be maintained and distributed.

The user has worked with bubbletea maintainers who are open to merging WASM support patches. A fork with these patches now exists at `~/projects/bubbletea` on the `nm-wasm` branch, containing:
- `signals_wasm.go` — signal handling for WASM
- `termios_wasm.go` — terminal I/O stubs for WASM
- `tty_wasm.go` — TTY initialization for WASM
- Modifications to `termios_other.go`

## Solution

### 1. Dependency Resolution

Add a local `replace` directive to `go.mod`:
```
replace charm.land/bubbletea/v2 => ../bubbletea
```

This tells Go to use the local checkout of bubbletea (the nm-wasm branch) instead of fetching from charm.land. The path is relative, so it works as long as both repos are checked out at the expected locations.

### 2. WASM Build Simplification

Replace the current WASM build process (which invokes `booba-wasm-build`) with standard Go tooling:
```bash
GOOS=js GOARCH=wasm go build -o <output> <package>
```

No injection, no temp directories, no custom tool. The patches are already in the bubbletea dependency.

### 3. Cleanup

**Delete:**
- `cmd/booba-wasm-build/` — entire directory (including `_stubs/` subdirectory)

**Update:**
- `Taskfile.yml` — remove `build-cmd-booba-wasm-build` task; update WASM-related tasks to use plain `go build`
- `.goreleaser.yml` — remove the booba-wasm-build build configuration and binary entry

## Files Affected

| File | Change |
|------|--------|
| `go.mod` | Add `replace charm.land/bubbletea/v2 => ../bubbletea` |
| `Taskfile.yml` | Remove `build-cmd-booba-wasm-build` task; update wasm build tasks |
| `.goreleaser.yml` | Remove booba-wasm-build build entry |
| `cmd/booba-wasm-build/` | Delete entire directory |

## Prerequisites

- Both `booba` and `bubbletea` repos must be checked out locally
- Relative path: `../bubbletea` must point to the nm-wasm branch
- Developers and CI must have this layout before running WASM builds

## Migration Path

**Phase 1 (now):** Local replace directive (`../bubbletea`)
- Developers must have both repos checked out
- Suitable for active development and testing

**Phase 2 (when patches merge):** Public git dependency
- If/when bubbletea maintainers merge WASM support, update to:
  ```
  require charm.land/bubbletea/v2 vX.Y.Z
  ```
- No replace directive needed
- Works in all environments

## Benefits

- **Simpler build pipeline** — No custom tool, no temp directories, no file injection
- **Clearer ownership** — WASM support is part of the bubbletea dependency, not a booba-specific workaround
- **Easier debugging** — Issues can be traced directly to bubbletea WASM stubs, not a build-time transformation
- **Ready for upstream** — When patches merge, a single line change transitions to the official fork
- **Reduced binary distribution** — No need to build/distribute `booba-wasm-build` via goreleaser

## Testing

- [ ] Local WASM builds work: `GOOS=js GOARCH=wasm go build -o test.wasm ./cmd/booba-view-example/`
- [ ] Taskfile WASM tasks complete successfully
- [ ] Example WASM application runs in the browser
- [ ] CI/CD pipeline handles the local replace directive correctly (may need adjustment)

## Open Questions / Assumptions

- CI/CD environment handling: The replace directive assumes local checkout. CI may need a setup step or conditional logic.
- Long-term maintenance: Assumes patches will be merged upstream relatively quickly; if not, the nm-wasm branch must be kept in sync with upstream.
