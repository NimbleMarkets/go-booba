# Development Guide

## Building from Source

booba vendors [`ghostty-web`](https://github.com/NimbleMarkets/ghostty-web) (NimbleMarkets fork) as a git submodule at `third_party/ghostty-web`. Clone with submodules:

```sh
git clone --recurse-submodules https://github.com/NimbleMarkets/go-booba.git
# or after a regular clone:
git submodule update --init --recursive
```

Then `task build` builds everything: the wasm + JS inside the submodule, the booba TypeScript embed, copies the artifacts into `serve/static/` for `go:embed`, and produces `bin/booba`. The build needs `bun` and `zig 0.15.2`; locally `task` will use `nix develop` from `third_party/ghostty-web/flake.nix` if `nix` is on PATH (recommended), otherwise it expects both to be available directly. See `Taskfile.yml:build-ghostty-web`.

The embedded `serve/static/booba/*.js` and `serve/static/ghostty-web/*` files are committed so `go install github.com/NimbleMarkets/go-booba/cmd/booba` works without a JS toolchain. CI rebuilds them on every push and force-commits the result back to the branch (`.github/workflows/rebuild-static.yml`), so the bytes in HEAD are always traceable to a CI run plus the submodule SHA. To verify locally: clone with submodules, run `task build`, and inspect any diff in `serve/static/` — bytes may differ from the CI-built ones due to build-environment determinism, but the *source* should be identical.

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
