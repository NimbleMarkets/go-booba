# Design: `@nimblemarkets/booba` npm Package

**Date:** 2026-04-15
**Status:** Approved

## Goal

Publish booba's TypeScript terminal wrapper as `@nimblemarkets/booba` on
GitHub Packages so consumers can `npm install` it instead of copying
pre-built JS files. The Go `serve` package continues to embed compiled
assets via `go:embed`.

## Constraints

- Single TypeScript source tree (`ts/`) serves both the npm package and
  the `go:embed` build.
- `ghostty-web` is a peer dependency — consumers provide it.
- Published to GitHub Packages (not public npm). Consumers add a one-line
  `.npmrc` for the `@nimblemarkets` scope.
- The existing `serve/static/booba/` embed path must keep working for
  the Go server.

## Architecture

### Import path change

`ts/booba.ts` currently imports ghostty-web via a relative path that
only works in the `serve/static/` layout:

```ts
// Before
import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';
```

This changes to a bare package import:

```ts
// After
import { init, Terminal, FitAddon } from 'ghostty-web';
```

No other TS files need changes — they only import from sibling `./` paths.

### Two tsconfigs

**`tsconfig.json`** — the primary config, used for the npm package build:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "lib": ["ES2020", "DOM"],
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "allowSyntheticDefaultImports": true,
    "esModuleInterop": true,
    "allowJs": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "outDir": "dist",
    "rootDir": "ts",
    "strict": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["ts/**/*"],
  "exclude": ["node_modules", "dist", "**/*.test.ts"]
}
```

- Outputs to `dist/`
- Bare `ghostty-web` import resolves from `node_modules`
- This is what `npm publish` ships

**`tsconfig.embed.json`** — extends the base, overrides for `go:embed`:

```json
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "./serve/static/booba",
    "paths": {
      "ghostty-web": ["./serve/static/ghostty-web/ghostty-web.js"]
    }
  }
}
```

- Outputs to `serve/static/booba/` for `go:embed`
- `paths` remaps the bare `ghostty-web` import to the relative location
  where `task build-serve-assets` places the ghostty-web distribution

### package.json

```json
{
  "name": "@nimblemarkets/booba",
  "version": "0.1.0",
  "type": "module",
  "description": "Terminal wrapper for BubbleTea programs using ghostty-web",
  "main": "dist/booba.js",
  "module": "dist/booba.js",
  "types": "dist/booba.d.ts",
  "exports": {
    ".": {
      "import": "./dist/booba.js",
      "types": "./dist/booba.d.ts"
    }
  },
  "files": ["dist"],
  "scripts": {
    "build": "tsc",
    "build:embed": "tsc -p tsconfig.embed.json"
  },
  "publishConfig": {
    "registry": "https://npm.pkg.github.com"
  },
  "peerDependencies": {
    "ghostty-web": "^0.4.0"
  },
  "devDependencies": {
    "ghostty-web": "^0.4.0-next.14.g6a1a50d",
    "typescript": "^5.9.3"
  }
}
```

- `ghostty-web` is a **peer dependency** (consumers install it alongside)
- Also in `devDependencies` so it's available for building
- `files: ["dist"]` ensures only compiled output is published
- Two build scripts: `build` for npm, `build:embed` for go:embed

### Taskfile changes

The `build-assets` task changes from `npx tsc` to `npx tsc -p tsconfig.embed.json`.

A new `build-npm` task runs `npx tsc` (base tsconfig, outputs to `dist/`).

### Release workflow

New `.github/workflows/release.yml`:

```yaml
name: Release npm package

on:
  push:
    tags: ['v[0-9]+.[0-9]+.[0-9]+']

permissions:
  contents: read
  packages: write

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6

      - uses: actions/setup-node@v6
        with:
          node-version: 20
          registry-url: https://npm.pkg.github.com

      - run: npm ci
      - run: npx tsc

      - run: npm publish
        env:
          NODE_AUTH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Triggers on version tags (e.g., `v0.1.0`). Builds TypeScript and
publishes to GitHub Packages.

### .gitignore update

Add `dist/` to `.gitignore` — npm build output should not be committed.

## Example repo changes

`go-booba-example` updates to consume the published package:

**`.npmrc`** (new file):
```
@nimblemarkets:registry=https://npm.pkg.github.com
```

**`package.json`** adds `@nimblemarkets/booba` as a dependency:
```json
{
  "dependencies": {
    "@nimblemarkets/booba": "^0.1.0",
    "ghostty-web": "^0.4.0-next.14.g6a1a50d"
  }
}
```

**`web/booba/`** is removed from the repo. The `pages.yml` workflow
copies from `node_modules` instead:

```yaml
- name: Copy runtime assets
  run: |
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
    mkdir -p web/ghostty-web
    cp node_modules/ghostty-web/dist/ghostty-web.js web/ghostty-web/
    cp node_modules/ghostty-web/dist/ghostty-vt.wasm web/ghostty-web/
    mkdir -p web/booba
    cp node_modules/@nimblemarkets/booba/dist/*.js web/booba/
```

**`web/index.html`** stays the same — imports from `./booba/booba.js`
which is populated at build time.

## Files changed (booba repo)

| File | Change |
|------|--------|
| `ts/booba.ts` | Change ghostty-web import to bare package name |
| `tsconfig.json` | Change `outDir` to `dist` |
| `tsconfig.embed.json` | New file — extends base, outputs to `serve/static/booba` with path remap |
| `package.json` | Add name, version, exports, peerDependencies, scripts, publishConfig |
| `Taskfile.yml` | `build-assets` uses `tsconfig.embed.json`; new `build-npm` task |
| `.github/workflows/release.yml` | New workflow for npm publishing on tags |
| `.gitignore` | Add `dist/` |

## Files changed (go-booba-example repo)

| File | Change |
|------|--------|
| `.npmrc` | New file — points `@nimblemarkets` scope at GitHub Packages |
| `package.json` | Add `@nimblemarkets/booba` dependency |
| `web/booba/` | Remove checked-in JS files |
| `.github/workflows/pages.yml` | Copy booba from `node_modules` |
| `.gitignore` | Add `web/booba/` |
