# Prompt: implement BiDi rendering in ghostty-web

Paste the section below to an agent working in a `ghostty-web` checkout.

---

## Task

Implement Unicode bidirectional (BiDi) text rendering in ghostty-web. RTL scripts
currently render mirrored. Tracking issue: coder/ghostty-web#83; upstream context:
ghostty-org/ghostty#1442.

## Findings — read these before planning; they are verified, not assumptions

Source audited at `ghostty-web@bfbe0e5` (branch `nm-webgpu`) and
`ghostty@94b93a091` (branch `nm-webgpu`). Defect reproduced on the WebGPU backend
via a Go TUI over WebTransport, 2026-07-29.

### The fix belongs entirely in ghostty-web. Ghostty needs no changes.

This is the answer to the question issue #83 puts to maintainers, and it is the
single most important finding here.

Ghostty core has no BiDi and needs none for this. `grep -riE 'bidi|bidirectional'
src/ include/` across the Zig tree returns **two** incidental comments — one in
`src/font/shaper/coretext.zig:192` that deliberately *disables* CoreText's BiDi,
one unrelated in `src/terminal/Screen.zig:8667`.

More to the point, ghostty-web already has everything the algorithm needs.
`GhosttyTerminal.getViewport()` (`lib/ghostty.ts:1137`) batch-materializes the
whole viewport as a JS `GhosttyCell[]`, codepoints and all. BiDi is a pure
function of a row's codepoint sequence plus a paragraph direction. **The VT API
data was never the blocker** — so do not wait on ghostty#1442 or the #162 Ghostty
1.3 upgrade, and do not add wasm exports for this.

The VT headers carry no directionality state whatsoever — no paragraph
direction, no character path, no BiDi mode.

**Scope caveat.** All of the above holds for *implicit* BiDi, where the terminal
auto-detects paragraph direction per row. That is what #83 asks for. If
ghostty-web later wants *explicit*, application-controlled direction — ECMA-48
`SCP` (Select Character Path), `SDS`/`SRS`, or `BDSM` from the terminal-wg BiDi
draft — those are escape sequences, and parsing them into per-row state is
squarely libghostty-vt's job. Decide up front which of the two you are building;
only the second one pulls Ghostty in.

### There are exactly two places that map cell index to x-position

Not three. The renderers share more than the issue thread suggests:

1. **`lib/renderer-canvas2d.ts:687`** — `renderLine()`. Two passes
   (`for (let x = 0; x < line.length; x++)` at :703 backgrounds, :711 text),
   both positioning by `x * metrics.width`. Per-cell `fillText` at :857.
2. **`lib/renderer-core.ts:414`** — `encodeCells()`, the shared GPU instance
   builder. Fetches the viewport at :489, then `for (let x = 0; x < dims.cols;
   x++)` at :516 writing to `arr[(y * dims.cols + x) * CELL_U32S]`.

Both GPU backends delegate to it — `renderer-webgl.ts:31` and
`renderer-webgpu.ts:30` import it as `coreEncodeCells` (webgpu wraps it at :902).
So fixing `encodeCells` fixes WebGL and WebGPU together. Two sites, two backends'
worth of coverage.

Glyph rasterization itself is fine: `lib/renderer-core.ts:351` rasterizes one
grapheme per atlas slot. The atlas is order-agnostic — only placement is wrong.

### Copy is already correct. The two pixel→column conversions are what break.

`SelectionManager.getSelection()` (`lib/selection-manager.ts:124`) walks logical
columns `colStart..colEnd` and concatenates, so clipboard text is in logical
order — correct, and it must **stay** that way.

The hazard is at the other end. Two separate places convert a pixel x to a
column, and **both** produce a raw visual column that stops being the logical one
the moment glyphs reorder:

1. `lib/selection-manager.ts:854` — `Math.floor(x / metrics.width)`, used for
   selection and hit testing. Selects the wrong characters.
2. `lib/input-handler.ts:738` — `pixelToCell()`,
   `Math.floor(x / dims.width) + 1`, which feeds **mouse reporting to the
   application** in 1-based terminal coordinates. This one is easy to miss
   because nothing on screen looks wrong: the TUI simply receives the wrong
   column and reacts to the wrong thing under the user's cursor.

Together with the two paint paths above, that makes **four** consumers of the
visual↔logical mapping, not two.

And a contiguous *visual* selection over mixed LTR/RTL text does not map to a
contiguous *logical* range — it maps to several disjoint ones. `getSelection()`'s
single `colStart..colEnd` span per row cannot express that. Reworking selection
to a set of logical ranges is the largest piece of this task, and it is easy to
miss because the screen looks right while the clipboard is wrong.

### Ordering is broken for every RTL script, not just Hebrew

Confirmed on the same run: the harness paints its first word red and its second
blue in logical order, and **both** the Hebrew row and the Arabic row render red
to the *left* of blue. Class R and class AL are equally affected, as is Persian.
Do not treat this as a Hebrew bug.

### Arabic has a second, independent defect: no cursive joining

Confirmed on the same run. Arabic and Persian render as **isolated** letter
forms — visibly disconnected — where correct output joins them cursively.

This is shaping, not ordering, and **a perfect BiDi fix will not fix it**.
`lib/renderer-core.ts:351` rasterizes one grapheme per atlas slot, so each letter
reaches `fillText` alone and the browser emits its isolated form. Joining needs
shaping across cell boundaries, which the per-cell atlas cannot express as it
stands. Ghostty proper solves this with a real shaper over grapheme runs.

Treat it as a separate work item and decide its scope explicitly before starting
— it is plausibly larger than the BiDi work. Do not let "Hebrew looks right now"
stand in for "RTL works": Hebrew is non-cursive and will pass while Arabic is
still visibly broken.

### Character classes the implementation must distinguish

Relevant if you hand-roll the UBA rather than adopting a library — these are the
distinctions a naive implementation collapses, and each has a harness row:

- **R vs AL.** Hebrew is class R (strong RTL); Arabic and Persian are class AL
  (Arabic Letter). Several UBA rules resolve AL differently from R, so passing
  the Hebrew rows proves nothing about the Arabic ones.
- **AN vs EN.** Arabic-Indic digits `٤٥٦` (U+0664–0666) are class **AN**, while
  Persian / Extended Arabic-Indic digits `۴۵۶` (U+06F4–06F6) are class **EN**,
  the same class as ASCII digits. They look related and resolve differently —
  notably in how surrounding neutrals and separators are handled.
- **Neutrals next to each.** Punctuation and spaces resolve against the
  surrounding strong types, which is why the harness carries a punctuation row
  and mixed LTR/RTL rows rather than pure-RTL samples alone.

### No existing BiDi scaffolding

`grep -riE '\bbidi\b|\brtl\b' lib/*.ts` returns only unrelated hits
(`compositionstart`, `autoScrollDirection`). Greenfield.

### A CSS fix is impossible

`direction: rtl` cannot work: one `fillText` per cell means no run ever reaches
the text engine intact for it to apply the UBA to.

## Suggested approach

Compute a per-row logical↔visual permutation and route everything through it.
Never reverse strings.

1. New module producing, for a row of `GhosttyCell[]`, a `visualToLogical` index
   map plus its inverse. Implementing the UBA from scratch is a large job —
   evaluate a vetted implementation such as `bidi-js` first, against
   ghostty-web's dependency and bundle-size budget.
2. `renderLine()` iterates visual positions, resolving each to a logical cell
   through the map. Keep the two-pass background/text split.
3. `encodeCells()` does the same when writing instance data — the map applies at
   the `arr[...]` index, so the GPU side needs no shader change.
4. Apply the inverse map at **both** pixel→column conversions —
   `selection-manager.ts:854` and `input-handler.ts:738` — and widen the
   selection model to hold multiple logical ranges per row.
5. Cache the map per row, invalidated on row change. This runs every frame.
6. Cursor placement moves to visual coordinates too.

Settle these explicitly, ideally with maintainers on #83, since they set the API
shape:

- **Paragraph direction** — auto-detect per row from the first strong character,
  or a terminal-wide mode? Most emulators default to LTR paragraph direction with
  implicit per-row UBA.
- **Scrollback and reflow** — rows re-wrap; confirm the map is derived at render
  time and never cached across a reflow.
- **Prior art** — mlterm and Konsole implement terminal BiDi, and terminal-wg has
  a draft spec covering DEC/ECMA-48 BiDi modes. Follow existing conventions
  rather than inventing semantics.

## Verifying

Reproduction harness: `tests/rtl-text/` in the booba repo (see its README).
`go run ./tests/rtl-text --listen :8099`, then open it. Fifteen rows in two
sections — Hebrew (class R) for ordering, Arabic and Persian (class AL) for
ordering plus cursive joining — including mixed LTR/RTL, both digit classes
(AN and EN), punctuation, ANSI styled runs, a bordered box, and codepoint dumps
proving the PTY received logical order.

**The reliable ordering check needs no Hebrew or Arabic literacy.** Rows 5 and 10
paint the first word red and the second blue, in logical order. Correct rendering
puts the red word to the **right** of the blue one. Red-on-the-left means
logical-order painting. Use this instead of reading glyph shapes off a
screenshot.

**The joining check is row 11** — `عربي`, four letters that should all connect.
Disconnected letters mean shaping is unfixed, independent of ordering.

**Do not check your work against Ghostty running natively.** It has no BiDi
either (see the first finding), so it renders these samples mirrored too. Both
terminals will agree and both will be wrong — a comparison that looks like a
pass. The same applies to most terminal emulators; mlterm and Konsole are the
common exceptions.

For ground truth, render the same string in two `<div>`s in the same browser at
~50px — one plain (correct UBA), one with `unicode-bidi: bidi-override;
direction: ltr` (the current defect). Compare the terminal against those two.
Serve the page over `http://` — a `data:` URL will not navigate.

Inspecting the output is harder than it sounds, since the terminal is a
`<canvas>` and DOM reads tell you nothing about what was painted. Screenshot
tooling that offers a "zoom" usually crops at native resolution rather than
magnifying. CSS-upscale the canvas instead, which magnifies already-painted
pixels so glyph order and joining are preserved exactly:

```js
document.querySelectorAll('canvas').forEach(c => {
  c.style.transformOrigin = '0 0';
  c.style.transform = 'scale(6) translate(-186px, -102px)'; // pan to region
  c.style.imageRendering = 'pixelated';
});
```

Apply it to **every** canvas in the container — there are two layers (text and
overlay), and scaling only one desynchronises them.

Cover in unit tests, following the existing `renderer-*.test.ts` and
`selection-manager.test.ts` patterns:

- copy after a visual selection over mixed text yields logical-order text
- hit testing maps a visual position to the right logical cell
- mouse reporting sends the logical column to the application, not the visual one
  (regress this and clicks land on the wrong widget with nothing looking amiss)
- a visually contiguous selection spanning an LTR/RTL boundary
- wide/spacer cells (`cell.width === 0`) interacting with reordering

Confirm on canvas2d **and** a GPU backend — the HUD bottom-right names the active
one. `renderer-factory.ts` picks the backend.

## Constraints

- `origin` is `coder/ghostty-web`, `nm` is the NimbleMarkets fork. Per fork
  convention `nm-kitty-meow` is the pristine, PR-able series against upstream,
  while the current working branch is `nm-webgpu` (what the vendored
  `nm-kitty-built` artifacts are built from). Confirm which branch this work
  should land on before starting.
- Build with `nix develop --command bun run build`. Homebrew zig and the
  ziglang.org tarball both fail on this project.
- Do not reverse strings and do not special-case Hebrew. The Unicode algorithm
  covers Arabic, Hebrew, and mixed content; a shortcut that fixes the demo will
  fail on real text.
