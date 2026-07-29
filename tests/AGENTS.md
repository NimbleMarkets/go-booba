# tests/ — agent guide

## What lives here

Manual reproduction harnesses, not automated tests. Each subdirectory is a
standalone `main` package that renders a known-tricky case so a human (or an
agent driving a browser) can look at it. They assert nothing; `go test ./...`
runs zero tests here.

Use this tree when the thing under test is *visual* — glyph placement, colour,
cell geometry — and a Go assertion can't see the defect because the defect is
in the browser renderer.

| Harness | Covers |
|---|---|
| `rtl-text/` | RTL ordering (BiDi) and Arabic cursive joining — Hebrew, Arabic, Persian (coder/ghostty-web#83) |

## Package requirements

Every harness must:

- Build for **both** native and `js/wasm`. `go build ./...` covers native;
  check wasm explicitly with `GOOS=js GOARCH=wasm go build -o /dev/null ./tests/<name>`.
  This is why each one carries a `web.go` (`//go:build !js`) and a `web_js.go`
  (`//go:build js`) stub — the `serve` package needs Unix PTYs and can't compile
  for wasm.
- Pass `golangci-lint run --enable=errcheck ./tests/...`, which the pre-commit
  hook runs. `./...` globs this tree.
- Follow `cmd/booba-view-example/` for the `--listen` wiring. Copy it; don't
  invent a new pattern.

Render a **static** screen where possible — no timers, no animation. Then
anything wrong on screen is the renderer's doing, not a race in the harness.

## Running one

```sh
go run ./tests/<name>                 # native terminal baseline
go run ./tests/<name> --listen :8099  # ghostty-web in the browser
```

`:8080` is often already taken; pick a high port. Serve assets are embedded at
compile time, so `go run` always picks up the current `serve/static/` — but a
prebuilt `bin/booba` will not. See the static-embed note in project memory.

## Designing a harness that can actually be verified

The hard part is not rendering the case, it's being *sure* what you're looking
at. Screenshot evidence is only as good as the reader's ability to judge it, and
for scripts you can't read, that's near zero.

**Build in a discriminator that doesn't require reading the text.** The
`rtl-text` harness paints שלום red and עולם blue. Logical order is red then
blue, so correct RTL rendering puts red on the *right*. Red-on-the-left proves
logical-order painting with no Hebrew literacy at all — and colour survives
JPEG compression and small glyph sizes, which shape does not.

**Print the codepoints you actually wrote.** A row spelling out `U+05E9 U+05DC
…` establishes what went to the PTY, separating "the program emitted it wrong"
from "the renderer painted it wrong". Without that the bug report is arguable.

**Pick sample data whose glyphs actually differ where the test does.** Two
codepoint ranges you're contrasting may render identically — Persian
(U+06F0–06F9) and Arabic-Indic (U+0660–0669) digits are indistinguishable at
1/2/3 and obviously different at 4/5/6. Sampling the wrong digits makes the row
unable to show what it exists to show, and invites a false "font fallback" bug
report. Before blaming a renderer for glyphs that look wrong, check what the
codepoints are supposed to look like.

**Render a ground-truth control in the same browser.** For `rtl-text` that's
two `<div>`s with the same string: one plain (browser applies the Unicode BiDi
Algorithm correctly) and one with `unicode-bidi: bidi-override; direction: ltr`
(forces logical order — what a per-cell canvas renderer produces). Screenshot
both at ~50px and compare against the terminal. Same browser, same font, so
font-fallback differences can't confound it. Serve it from the scratchpad with
`python3 -m http.server`; a `data:` URL won't navigate.

## Inspecting canvas output in the browser

The terminal is a `<canvas>`, so DOM/accessibility reads tell you nothing about
what was painted. Screenshots are the only evidence.

- **`zoom` crops, it does not magnify.** Asking for a small region returns that
  region at native resolution — useless for glyph detail.
- **CSS-upscale the canvas instead.** This magnifies already-painted pixels, so
  glyph *order* is preserved exactly:

  ```js
  document.querySelectorAll('#terminal-container canvas').forEach(c => {
    c.style.transformOrigin = '0 0';
    c.style.transform = 'scale(6) translate(-186px, -102px)'; // pan to region
    c.style.imageRendering = 'pixelated';
  });
  ```

  Apply it to **every** canvas in the container — there are two layers (text and
  overlay) and scaling only one desynchronises them.
- **The HUD in the bottom-right names the active backend** (`webgpu 60 fps`).
  Record it. `lib/renderer-canvas2d.ts`, `renderer-webgl.ts`, and
  `renderer-webgpu.ts` are separate implementations and can disagree; a defect
  confirmed on one is not confirmed on the others.
- The `BoobaTerminal` instance is not exposed on `window`, so you can't reach
  terminal options from the console. Manipulate the canvas element directly.

## Reporting a finding

State the branch, the ghostty-web submodule SHA and version, and the renderer
backend. Without all three a screenshot isn't reproducible:

```sh
git rev-parse --abbrev-ref HEAD
git -C third_party/ghostty-web log -1 --format='%h %ci %s'
grep -m1 '"version"' third_party/ghostty-web/package.json
```
