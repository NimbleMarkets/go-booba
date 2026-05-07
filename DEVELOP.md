# Development Guide

## BubbleTea Integration

booba uses a patched version of BubbleTea with WebAssembly support. The patched fork is at [neomantra/bubbletea:nm-wasm](https://github.com/neomantra/bubbletea/tree/nm-wasm) and is referenced locally via a `replace` directive in `go.mod`:

```
replace charm.land/bubbletea/v2 => ../bubbletea
```

This means:
- Clone both `booba` and `bubbletea` (nm-wasm branch) in sibling directories
- The replace directive automatically uses the local bubbletea for builds
- No separate build tool or monkeypatching needed

The patches include WASM-specific signal handling and TTY initialization that aren't in upstream BubbleTea yet. Once merged upstream, the replace directive can be removed.

## Building from Source

booba vendors [`ghostty-web`](https://github.com/NimbleMarkets/ghostty-web) (NimbleMarkets fork) as a git submodule at `third_party/ghostty-web`. Clone with submodules:

```sh
git clone --recurse-submodules https://github.com/NimbleMarkets/go-booba.git
# or after a regular clone:
git submodule update --init --recursive
```

The submodule is pinned to the `nm-kitty-built` branch, which ships the prebuilt `dist/` (wasm + JS + .d.ts) committed alongside the source by ghostty-web's CI. `task build` then just copies `third_party/ghostty-web/dist/*` into `serve/static/` for `go:embed`, runs the booba TypeScript embed, and produces `bin/booba` — no `bun` or `zig` toolchain needed locally.

The embedded `serve/static/booba/*.js` and `serve/static/ghostty-web/*` files are committed so `go install github.com/NimbleMarkets/go-booba/cmd/booba` works without a JS toolchain. They're refreshed by bumping the `third_party/ghostty-web` submodule pointer to a new `nm-kitty-built` tip and re-running `task build-serve-assets` locally before committing.

## Building WASM Applications with booba

To compile a BubbleTea application to WebAssembly for use in the browser:

```sh
GOOS=js GOARCH=wasm go build -o app.wasm ./cmd/myapp/
```

The patched BubbleTea (via the `replace` directive) provides all necessary WASM stubs (signal handling, TTY initialization) automatically. No custom build tool or additional configuration is needed.

**Requirements:**
- Go 1.21+ (WebAssembly support is stable and built-in)
- Both `booba` and `bubbletea` (nm-wasm branch) cloned locally in sibling directories
- The replace directive in booba's `go.mod` points to the local bubbletea

**Runtime:**
The generated `.wasm` file needs a host environment that provides:
- A way to accept input (keyboard, resize events)
- A way to render output (terminal emulator frontend)

The booba library provides:
- `booba.Run()` — automatically picks native or WASM runtime based on build target
- `booba.NewProgram()` — for more control, returns a `booba.Program` that wraps `*tea.Program`
- `wasm` subpackage — low-level browser bridge for custom implementations

See the example at `cmd/booba-view-example/` for a full working application that builds for both native and WASM targets.

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
