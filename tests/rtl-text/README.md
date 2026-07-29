# rtl-text

Isolated reproduction for right-to-left text rendering in ghostty-web, covering
both BiDi ordering and Arabic cursive joining across Hebrew, Arabic, and Persian.

- [coder/ghostty-web#83](https://github.com/coder/ghostty-web/issues/83) — Hebrew renders reversed in the browser terminal
- [ghostty-org/ghostty#1442](https://github.com/ghostty-org/ghostty/issues/1442) — upstream BiDi support in the VT/screen model

## What it does

A static BubbleTea screen — no timers, no animation, no input beyond quit — that
writes RTL samples to the PTY in logical (Unicode) order:

```
Hebrew — ordering (BiDi class R)
 1 pure RTL        שלום עולם
 2 mixed LTR/RTL   hello שלום world
 3 RTL + digits    שלום 123 עולם
 4 RTL + punct     !שלום, עולם?
 5 styled runs     שלום עולם          (red / blue, separate SGR runs)
 6 boxed           ╭───────────╮
                   │ שלום עולם │      (lipgloss RoundedBorder)
                   ╰───────────╯
 7 logical order   U+05E9 U+05DC U+05D5 U+05DD U+0020 U+05E2 U+05D5 U+05DC U+05DD

Arabic / Persian — ordering (class AL) and cursive joining
 8 arabic          مرحبا بالعالم
 9 arabic mixed    hello مرحبا world
10 arabic styled   مرحبا بالعالم      (red / blue)
11 joining         عربي               (all four letters should join)
12 digits (AN)     مرحبا ٤٥٦ بالعالم
13 persian         سلام دنیا
14 digits (EN)     سلام ۴۵۶ دنیا
15 logical order   U+0639 U+0631 U+0628 U+064A
```

Rows 1–4 cover the plain cases. Rows 5–6 extend into ANSI style runs and
border/width calculation, which issue #83 names as things a renderer-side fix
also has to get right. Rows 7 and 15 spell out the codepoints the program
actually wrote: `U+05E9` (ש) comes first on the wire, which establishes that the
PTY received logical order and that any reversal on screen happened downstream
in the renderer.

Rows 8–15 are not redundant with the Hebrew ones:

- Hebrew is BiDi class **R**; Arabic and Persian are class **AL**, which resolves
  differently in several UBA rules that Hebrew alone never reaches.
- Arabic-Indic digits `٤٥٦` (U+0664–0666) are class **AN**, while Persian
  (Extended Arabic-Indic) `۴۵۶` (U+06F4–06F6) are class **EN**, same as ASCII.
  Same numeric value, different class, different code paths. 456 is chosen
  deliberately: the two encodings are visually identical at 1/2/3 but clearly
  distinct at 4/5/6, so rows 12 and 14 also catch font fallback substituting
  one range for the other.
- Arabic and Persian are **cursive**, which surfaces a second, independent
  defect — see below.

Because the screen is static, anything wrong on it is the renderer's doing.

## Running

Native terminal:

```sh
go run ./tests/rtl-text
```

Served to the browser through ghostty-web (the case under test):

```sh
go run ./tests/rtl-text --listen :8099
```

**The native run is not a correctness baseline.** Ghostty has no BiDi either — a
`grep -riE 'bidi|bidirectional' src/ include/` over its Zig tree finds two
incidental comments, one of which deliberately *disables* CoreText's BiDi. A
native terminal renders these samples mirrored too, so both screens can agree
and both be wrong. Diffing them proves nothing on its own.

Ground truth is a browser control: render the same string in two `<div>`s, one
plain (the browser applies the UBA correctly) and one with `unicode-bidi:
bidi-override; direction: ltr` (forces logical order, which is what a per-cell
renderer produces). Compare the terminal against those two. `tests/AGENTS.md`
has the recipe.

The native run is still worth doing — it confirms the harness itself emits the
right bytes, and it compares two independent renderers.

## What to look for

Correct rendering applies the Unicode Bidirectional Algorithm per line, so each
Hebrew run paints right-to-left: in row 1 the word שלום occupies the *rightmost*
cells of the run and עולם sits to its left.

The defect is that the renderer positions every cell independently, left to
right, which denies the text engine any run to apply the UBA to. Each glyph
lands in its logical cell, so ש appears leftmost and the phrase reads mirrored.
A CSS `direction: rtl` cannot help, for the same reason.

Two code paths do this, and issue #83 names only the first:

- `renderLine()` in `lib/renderer-canvas2d.ts` — canvas2d
- the shared `encodeCells()` in `lib/renderer-core.ts` — WebGL **and** WebGPU

The reproduction below ran on WebGPU, so all three backends are affected. Check
the HUD in the bottom-right for the active one before drawing conclusions.

The reliable check is the colour one — rows 5 and 10, described below — not
reading glyph shapes. Row 2 is the useful tell for a *partial* fix: the LTR
words must stay in place while only the Hebrew run reorders.

Note that your source editor and this README's renderer also apply BiDi, so the
samples above already display in visual order — compare against the running
program, not against this file.

## Two defects, not one

Confirmed 2026-07-29 against ghostty-web 0.3.0 (`nm-kitty-built`, submodule
`52ed6df`), WebGPU backend:

**Ordering.** Rows 5 and 10 both paint red left of blue, so RTL runs are laid out
in logical order. Affects Hebrew, Arabic, and Persian alike.

**Cursive joining.** Rows 8–14 render Arabic and Persian as *isolated* letter
forms — visibly disconnected, where correct output joins them into cursive
words. This is a shaping problem, not an ordering one, and fixing BiDi will not
fix it: `lib/renderer-core.ts:351` rasterizes one grapheme per atlas slot, so
each letter reaches `fillText` with no neighbours and the browser produces its
isolated form. Joining requires shaping across cell boundaries.

Keep these separate when reporting or reviewing. A renderer can pass every
Hebrew row here and still render Arabic wrongly, which is exactly why the Arabic
rows exist.
