# Development Guide

## BubbleTea Integration

booba uses a patched version of BubbleTea with WebAssembly support. The patched fork is at [neomantra/bubbletea:nm-wasm](https://github.com/neomantra/bubbletea/tree/nm-wasm) and is pinned by pseudo-version via a `replace` directive in `go.mod`:

```
replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 v2.0.0-20260928192001-1b36865b418a
```

This means:
- Contributors need no special setup — a plain clone builds; Go fetches the fork like any other module
- **Consumers of booba must copy this replace directive into their own `go.mod`** to build for WASM — Go replace directives do not propagate to downstream modules (see the README's browser-embedding section for the consumer-facing instructions and troubleshooting)

The patches add `js && wasm` (and `wasip1`) build-tagged implementations of signal handling, TTY initialization, and termios that aren't in upstream BubbleTea yet. Once merged upstream, the replace directive can be removed.

To bump the pin to the fork branch tip:

```sh
go mod edit -replace charm.land/bubbletea/v2=github.com/neomantra/bubbletea/v2@nm-wasm
go mod tidy
```

then commit `go.mod` and `go.sum` — `go mod tidy` resolves the branch name back to a pinned pseudo-version.

## Building from Source

The pre-commit hook and `task go-lint` select the Go version declared in
`go.mod`, matching CI. This prevents a newer system Go from causing the
installed golangci-lint binary to panic while analyzing the standard library.
The hook also uses that toolchain's `gofmt`. Go downloads the selected toolchain
automatically if it is not already installed.

booba vendors [`ghostty-web`](https://github.com/NimbleMarkets/ghostty-web) (NimbleMarkets fork) as a git submodule at `third_party/ghostty-web`. Clone with submodules:

```sh
git clone --recurse-submodules https://github.com/NimbleMarkets/go-booba.git
# or after a regular clone:
git submodule update --init --recursive
```

The submodule is pinned to the `nm-kitty-built` branch, which ships the prebuilt `dist/` (wasm + JS + .d.ts) committed alongside the source by ghostty-web's CI. `task build` copies only the browser runtime `.js` and `.wasm` files into `serve/static/ghostty-web/`, runs the booba TypeScript embed, and produces `bin/booba` — no `bun` or `zig` toolchain needed locally. The copy script prunes obsolete chunks and non-runtime artifacts. Upstream declarations remain available through the installed npm package.

The embedded `serve/static/booba/*.js` and `serve/static/ghostty-web/*` files are committed so `go install github.com/NimbleMarkets/go-booba/cmd/booba` works without a JS toolchain. They're refreshed by bumping the `third_party/ghostty-web` submodule pointer to a new `nm-kitty-built` tip and re-running `task build-serve-assets` locally before committing.

The `serve/static` package (`serve/static/embed.go`) `go:embed`s these same files, so both the `serve` package and the `booba-assets` scaffolding tool ship them inside their binaries. Explicit `*.js` and `*.wasm` patterns include underscore-prefixed Vite chunks while excluding CJS bundles, declarations, and maps left over from older builds. The embed build emits JavaScript only; `npm run build` still emits declarations and source maps into `dist/` for npm consumers.

### Updating ghostty-web

```sh
git -C third_party/ghostty-web fetch origin nm-kitty-built
git -C third_party/ghostty-web checkout --detach origin/nm-kitty-built
npm install --package-lock-only --ignore-scripts
task build-serve-assets
task build-npm
task test
```

Commit the submodule pointer, lockfile, and embedded assets together. The lockfile
refresh picks up changes to the fork's npm dependencies. CI rebuilds the assets
and checks that the committed copies match, and `task test` runs both Go and
TypeScript tests.

The project's `.npmrc` uses `install-links=true` to install the prebuilt
ghostty-web package without its development dependencies. Booba declares its
own test tools, including Happy DOM. Run `npm ci` after changing the submodule
checkout to refresh the installed copy; Task tracks the upstream distribution
and does this automatically.

Build tasks preserve an initialized submodule checkout so testing an unstaged
dependency update does not reset it. After switching booba branches, run
`git submodule update --init --recursive` to synchronize the pinned dependencies.

## Building WASM Applications with booba

To compile a BubbleTea application to WebAssembly for use in the browser:

```sh
GOOS=js GOARCH=wasm go build -o app.wasm ./cmd/myapp/
```

The patched BubbleTea (via the `replace` directive — see [BubbleTea Integration](#bubbletea-integration)) provides all necessary WASM stubs (signal handling, TTY initialization) automatically. No custom build tool is needed, but builds outside this repository must carry the replace directive in their own `go.mod`.

`app.wasm` needs supporting files alongside it to run in a browser. The `booba-assets` tool scaffolds a complete `web/` directory (`wasm_exec.js`, `booba/`, `ghostty-web/`, starter `index.html`):

```sh
go run github.com/NimbleMarkets/go-booba/cmd/booba-assets web/
GOOS=js GOARCH=wasm go build -o web/app.wasm ./cmd/myapp/
```

See [ADAPTER_USAGE.md](./ADAPTER_USAGE.md) for the full WASM embedding workflow.

**Requirements:**
- Go 1.25+ (matching the `go` directive in `go.mod`; WebAssembly support is built-in)
- In-repo builds: nothing else — the `replace` directive in `go.mod` pulls the patched bubbletea fork automatically
- Out-of-repo (consumer) builds: copy the replace directive into your module's `go.mod` (see the README)

**Runtime:**
The generated `.wasm` file needs a host environment that provides:
- A way to accept input (keyboard, resize events)
- A way to render output (terminal emulator frontend)

The booba library provides:
- `booba.Run()` — automatically picks native or WASM runtime based on build target
- `booba.NewProgram()` — for more control, returns a `booba.Program` that wraps `*tea.Program`
- `wasm` subpackage — low-level browser bridge for custom implementations

See the example at `cmd/booba-view-example/` for a full working application that builds for both native and WASM targets.

## Renderer Configuration

The booba frontend supports multiple rendering backends:
- **WebGPU** — Modern hardware-accelerated rendering (requires compatible browser)
- **Canvas2D** — Fallback canvas-based rendering
- **Auto** — Automatically selects WebGPU if available, falls back to Canvas2D

Configure the default renderer via the `--renderer` CLI flag:

```sh
booba serve --renderer webgpu    # Force WebGPU
booba serve --renderer canvas2d  # Force Canvas2D
booba serve --renderer auto      # Auto-select (default)
```

Clients can override the server's default via the `?renderer=` query parameter:
- `http://localhost:8080/?renderer=canvas2d` — Force Canvas2D
- `http://localhost:8080/?renderer=webgpu` — Force WebGPU
- `http://localhost:8080/?renderer=auto` — Auto-select

In the browser, click the renderer badge in the bottom-right corner to toggle between WebGPU and Canvas2D.

## Command Documentation

The `booba` CLI is built on [spf13/cobra](https://github.com/spf13/cobra) and ships generated documentation alongside the binary:

- **Man pages** — `docs/man/booba.1` and one file per subcommand. Install with `cp docs/man/*.1 /usr/local/share/man/man1/`.
- **Markdown** — `docs/markdown/booba.md` and subcommand files, suitable for wikis or docs sites.
- **Shell completions** — `completions/booba.{bash,zsh,fish}`. Source the appropriate file from your shell rc, or install to your system's completion directory.

Regenerate everything after changing commands or flags:

```sh
task docs:build
```

The hidden `booba docs` subcommand drives this: `booba docs man -o <dir>`, `booba docs markdown -o <dir>`, and `booba completion <shell>`.

## Release CI Environment Variables

Releases are handled by [GoReleaser](https://goreleaser.com) via `.github/workflows/release.yml`. The following secrets are optional; when absent, macOS notarization is skipped and the release proceeds with unsigned binaries.

| Secret | Purpose |
|--------|---------|
| `GITHUB_TOKEN` | Automatically provided by GitHub Actions. Used to create the GitHub Release and upload artifacts. |
| `MACOS_SIGN_P12` | Base64-encoded Apple Developer ID Application certificate (`.p12`). Enables macOS code signing. |
| `MACOS_SIGN_PASSWORD` | Password for the `MACOS_SIGN_P12` certificate. |
| `MACOS_NOTARY_ISSUER_ID` | Apple App Store Connect Team / Notary issuer ID. |
| `MACOS_NOTARY_KEY_ID` | App Store Connect API key ID for notarization. |
| `MACOS_NOTARY_KEY` | App Store Connect API private key (PKCS8 `.p8` content). |

To set up macOS signing:
1. Export your Developer ID Application certificate from Keychain as `.p12`.
2. Base64-encode it: `base64 -i cert.p12 | pbcopy`
3. Paste the result into a repository secret named `MACOS_SIGN_P12`.
4. Add the certificate password as `MACOS_SIGN_PASSWORD`.
5. Create an App Store Connect API key with **Developer** role and copy the Issuer ID, Key ID, and private key content into the corresponding secrets.

The `etc/entitlements.plist` file relaxes the macOS hardened runtime for Go binaries (JIT, unsigned executable memory, and library validation are allowed). Adjust it if you introduce features that require additional entitlements.
