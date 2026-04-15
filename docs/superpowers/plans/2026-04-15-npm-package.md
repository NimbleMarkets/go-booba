# `@nimblemarkets/booba` npm Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish booba's TypeScript terminal wrapper as `@nimblemarkets/booba` on GitHub Packages, with dual build output for both npm consumers and Go `serve` embedding.

**Architecture:** Single TS source tree (`ts/`) compiled by two tsconfigs — `tsconfig.json` outputs to `dist/` for npm, `tsconfig.embed.json` outputs to `serve/static/booba/` for `go:embed`. The ghostty-web import changes from a relative path to a bare package name, with `paths` remapping it for the embed build.

**Tech Stack:** TypeScript, npm (GitHub Packages registry), GitHub Actions

**Spec:** `docs/superpowers/specs/2026-04-15-npm-package-design.md`

---

### Task 1: Change ghostty-web import to bare package name

**Files:**
- Modify: `ts/booba.ts:1`

- [ ] **Step 1: Update the import**

In `ts/booba.ts`, change line 1 from:

```ts
// @ts-ignore - Import will resolve at runtime in browser
import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';
```

to:

```ts
import { init, Terminal, FitAddon } from 'ghostty-web';
```

- [ ] **Step 2: Verify the npm build compiles**

Run: `npx tsc --noEmit`

Expected: Clean exit, no errors. The bare `ghostty-web` import resolves from `node_modules/ghostty-web` which has an `index.d.ts`.

- [ ] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "refactor: change ghostty-web import to bare package name"
```

---

### Task 2: Create dual tsconfig setup

**Files:**
- Modify: `tsconfig.json`
- Create: `tsconfig.embed.json`

- [ ] **Step 1: Update tsconfig.json to output to dist/**

Replace the contents of `tsconfig.json` with:

```json
{
    "compilerOptions": {
        "target": "ES2020",
        "module": "ESNext",
        "lib": [
            "ES2020",
            "DOM"
        ],
        "moduleResolution": "bundler",
        "resolveJsonModule": true,
        "allowSyntheticDefaultImports": true,
        "esModuleInterop": true,
        "allowJs": true,
        "declarationMap": true,
        "declaration": true,
        "sourceMap": true,
        "outDir": "dist",
        "rootDir": "ts",
        "strict": true,
        "skipLibCheck": true,
        "forceConsistentCasingInFileNames": true
    },
    "include": [
        "ts/**/*"
    ],
    "exclude": [
        "node_modules",
        "dist",
        "**/*.test.ts"
    ]
}
```

- [ ] **Step 2: Create tsconfig.embed.json**

Create `tsconfig.embed.json`:

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

- [ ] **Step 3: Verify the npm build**

Run: `npx tsc`

Expected: Clean exit. Files appear in `dist/` (booba.js, booba.d.ts, adapter.js, etc.).

- [ ] **Step 4: Verify the embed build**

Run: `npx tsc -p tsconfig.embed.json`

Expected: Clean exit. Files appear in `serve/static/booba/` as before.

- [ ] **Step 5: Verify the embed output resolves ghostty-web correctly**

Run: `head -1 serve/static/booba/booba.js`

Expected: The import should be rewritten to the relative path `../ghostty-web/ghostty-web.js` (TypeScript `paths` rewrites the import in the output). If it still says `ghostty-web`, we'll need to verify the paths config.

Note: TypeScript `paths` only affects type resolution, not emitted JS. The emitted JS will still contain `from 'ghostty-web'`. This is fine for the embed build because the `serve/static/` layout serves files via HTTP, and `index.html` uses an import map or the browser resolves it. However, if the current embed build relies on the relative path in the JS output, we need to verify. Check `serve/static/index.html` — it imports `./static/booba/booba.js` which then imports `ghostty-web`. Since these are served by the Go HTTP server (not a bundler), the browser needs the bare import to resolve. The `index.html` uses `<script type="module">` so the browser will try to resolve `ghostty-web` as a URL, which will fail.

**Resolution:** We need to keep the relative path in the embed output JS. Since `paths` doesn't rewrite emitted JS, we'll use a small post-build sed to fix the embed output:

Update `tsconfig.embed.json` to NOT use paths (remove the paths field):

```json
{
    "extends": "./tsconfig.json",
    "compilerOptions": {
        "outDir": "./serve/static/booba"
    }
}
```

Then the Taskfile embed build step (Task 4) will run sed after tsc to rewrite the import.

- [ ] **Step 6: Verify Go build still works**

Run: `go build ./...`

Expected: Clean exit. The Go embed picks up `serve/static/booba/*.js`.

- [ ] **Step 7: Commit**

```bash
git add tsconfig.json tsconfig.embed.json
git commit -m "build: add dual tsconfig for npm and go:embed builds"
```

---

### Task 3: Update package.json for npm publishing

**Files:**
- Modify: `package.json`

- [ ] **Step 1: Update package.json**

Replace `package.json` with:

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
  "files": [
    "dist"
  ],
  "scripts": {
    "build": "tsc",
    "build:embed": "tsc -p tsconfig.embed.json && sed -i'' -e \"s|from 'ghostty-web'|from '../ghostty-web/ghostty-web.js'|\" serve/static/booba/booba.js"
  },
  "publishConfig": {
    "registry": "https://npm.pkg.github.com"
  },
  "repository": {
    "type": "git",
    "url": "https://github.com/NimbleMarkets/go-booba.git"
  },
  "license": "MIT",
  "peerDependencies": {
    "ghostty-web": "^0.4.0"
  },
  "devDependencies": {
    "ghostty-web": "^0.4.0-next.14.g6a1a50d",
    "typescript": "^5.9.3"
  }
}
```

- [ ] **Step 2: Regenerate package-lock.json**

Run: `npm install`

Expected: Clean install, `package-lock.json` updated with the new package name.

- [ ] **Step 3: Verify npm build**

Run: `npm run build`

Expected: Files in `dist/`.

- [ ] **Step 4: Verify embed build**

Run: `npm run build:embed`

Expected: Files in `serve/static/booba/`. Check the import was rewritten:

Run: `grep "ghostty-web" serve/static/booba/booba.js`

Expected: `from '../ghostty-web/ghostty-web.js'` (the relative path, not bare `ghostty-web`).

- [ ] **Step 5: Verify npm pack contents**

Run: `npm pack --dry-run`

Expected: Lists only files under `dist/` plus package.json/README. No `ts/` source, no `serve/`, no `node_modules/`.

- [ ] **Step 6: Commit**

```bash
git add package.json package-lock.json
git commit -m "build: configure package.json for @nimblemarkets/booba npm package"
```

---

### Task 4: Update Taskfile and .gitignore

**Files:**
- Modify: `Taskfile.yml`
- Modify: `.gitignore`

- [ ] **Step 1: Update Taskfile.yml build-assets task**

In `Taskfile.yml`, change the `build-assets` task's command from:

```yaml
    cmds:
      - npx tsc
```

to:

```yaml
    cmds:
      - npm run build:embed
```

- [ ] **Step 2: Add build-npm task to Taskfile.yml**

Add after the `build-assets` task:

```yaml
  build-npm:
    desc: 'Build npm package to dist/'
    deps: [npm-deps]
    cmds:
      - npm run build
    sources:
      - ts/*.ts
    generates:
      - dist/*.js
```

- [ ] **Step 3: Add clean-npm to clean-assets**

In the `clean-assets` task, add a line:

```yaml
  clean-assets:
    desc: 'Cleans compiled assets'
    cmds:
      - rm -rf serve/static/booba
      - rm -rf serve/static/ghostty-web
      - rm -rf dist
```

- [ ] **Step 4: Add dist/ to .gitignore**

Add `dist/` to `.gitignore`:

```
# booba .gitignore

.task
.superpowers/
.DS_Store
node_modules/
bin/
dist/
/booba-view-example

serve/static/booba/
serve/static/ghostty-web/
```

- [ ] **Step 5: Verify full build chain**

Run: `task build`

Expected: Clean build — npm deps installed, embed TS compiled, ghostty-web copied, Go binaries built.

- [ ] **Step 6: Verify Go tests still pass**

Run: `task test`

Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add Taskfile.yml .gitignore
git commit -m "build: update Taskfile for dual tsconfig and add dist/ to .gitignore"
```

---

### Task 5: Add release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create the release workflow**

Create `.github/workflows/release.yml`:

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
      - run: npm run build

      - run: npm publish
        env:
          NODE_AUTH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add release workflow for npm publishing to GitHub Packages"
```

---

### Task 6: Update example repo to consume npm package

**Files (in `~/projects/go-booba-example/`):**
- Create: `.npmrc`
- Modify: `package.json`
- Modify: `.github/workflows/pages.yml`
- Modify: `.gitignore`
- Delete: `web/booba/` directory

- [ ] **Step 1: Create .npmrc**

Create `/Users/evan/projects/go-booba-example/.npmrc`:

```
@nimblemarkets:registry=https://npm.pkg.github.com
```

- [ ] **Step 2: Update package.json**

Replace `/Users/evan/projects/go-booba-example/package.json` with:

```json
{
  "private": true,
  "dependencies": {
    "@nimblemarkets/booba": "^0.1.0",
    "ghostty-web": "^0.4.0-next.14.g6a1a50d"
  }
}
```

- [ ] **Step 3: Update .gitignore**

Replace `/Users/evan/projects/go-booba-example/.gitignore` with:

```
node_modules/
web/app.wasm
web/wasm_exec.js
web/ghostty-web/
web/booba/
```

- [ ] **Step 4: Remove checked-in web/booba/ files**

Run: `rm -rf /Users/evan/projects/go-booba-example/web/booba/`

- [ ] **Step 5: Update pages.yml workflow to copy booba from node_modules**

In `.github/workflows/pages.yml`, update the "Copy runtime assets" step to:

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

- [ ] **Step 6: Update docs/GUIDE_GITHUB.md in booba repo**

In `/Users/evan/projects/booba/docs/GUIDE_GITHUB.md`, update the "Copy runtime assets" workflow step to match the new pattern (copy from `node_modules/@nimblemarkets/booba/dist/` instead of building from source), and update the package.json example to include `@nimblemarkets/booba` as a dependency with an `.npmrc` note.

- [ ] **Step 7: Commit both repos**

In booba repo:
```bash
git -C /Users/evan/projects/booba add docs/GUIDE_GITHUB.md
git -C /Users/evan/projects/booba commit -m "docs: update guide to use @nimblemarkets/booba npm package"
```

Note: The example repo commits happen separately when that repo is ready to push. The npm package must be published first (tag `v0.1.0` in booba repo) before the example repo's `npm ci` will work.

---

### Task 7: Publish initial release

This task is manual — it triggers the release workflow.

- [ ] **Step 1: Verify everything builds locally**

Run in booba repo:

```bash
npm run build          # npm package -> dist/
npm run build:embed    # go:embed -> serve/static/booba/
go build ./...         # Go binaries
go test ./...          # Go tests
```

Expected: All succeed.

- [ ] **Step 2: Tag and push**

```bash
git tag v0.1.0
git push origin main --tags
```

Expected: The `release.yml` workflow triggers and publishes `@nimblemarkets/booba@0.1.0` to GitHub Packages.

- [ ] **Step 3: Verify the package is published**

Run: `npm view @nimblemarkets/booba --registry=https://npm.pkg.github.com`

Expected: Shows version `0.1.0` with the correct metadata.

- [ ] **Step 4: Test install in example repo**

Run in go-booba-example:

```bash
npm install
```

Expected: `@nimblemarkets/booba` and `ghostty-web` install into `node_modules/`.

Verify: `ls node_modules/@nimblemarkets/booba/dist/booba.js` exists.
