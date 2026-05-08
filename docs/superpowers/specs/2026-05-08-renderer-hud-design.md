# Renderer HUD: Treeshakeable API Design

**Date:** 2026-05-08  
**Status:** Design (awaiting approval)  
**Author:** Claude  
**Affected:** `ts/hud.ts` (new), `ts/booba.ts` (additions), `docs/TYPESCRIPT_API.md` (additions)

## Problem Statement

Every consumer that embeds `BoobaTerminal` and wants to show the active renderer + FPS counter must re-derive the same ~30 lines of JS that already live in `serve/static/index.html`. The reference implementation is correct but not reusable. Consumers like ntcharts have their own DOM structure (sidebars, headers) and need flexible placement, not a viewport-fixed badge.

## Solution Overview

Extract the renderer HUD into a treeshakeable, opt-in module (`ts/hud.ts`) that:
- Provides a standalone `installRendererHud(terminal, opts)` function that can mount the badge anywhere in the DOM
- Exports a utility function `parseRendererFromURL()` for respecting `?renderer=` query params
- Completes the missing `renderer` field in `BoobaTerminalOptions`
- Keeps HUD logic separate so consumers can copy and customize if needed

## Architecture

### Module: `ts/hud.ts` (new file)

Three exports:

1. **`parseRendererFromURL(): 'auto' | 'webgpu' | 'canvas2d'`**
   - Reads `?renderer=` from `window.location.search`
   - Validates against allowed set; rejects invalid values
   - Falls back to `window.__boobaDefaultRenderer` if present
   - Final fallback: `'auto'`

2. **`interface BoobaRendererHudOptions`**
   - `parent?: HTMLElement` — mount target (default: `document.body`)
   - `className?: string` — optional CSS class for consumers to restyle
   - `bindToggleHotkey?: boolean` — if true, bind Alt+Shift+R to toggle webgpu↔canvas2d (default: true)

3. **`installRendererHud(terminal: Terminal, opts?: BoobaRendererHudOptions): () => void`**
   - Must be called after `booba.init()` (so `terminal.renderer` exists)
   - Creates a `<div>` with inline styles, appends to `opts.parent ?? document.body`
   - Starts rAF loop counting frames, updating text `"<backend> <n> fps"` once per second
   - If `bindToggleHotkey` is true, attaches keydown listener for Alt+Shift+R
   - Returns uninstall function that stops loop and removes element

### Behavior (from reference implementation)

**Badge DOM:**
- Single `<div>` element with `id="booba-renderer-hud"` (for debugging)
- Inline styles: `position: fixed`, `bottom: 12px`, `right: 12px` (default viewport positioning)
- Font: monospace 11px
- Color: `#666` text on `rgba(0,0,0,0.4)` background
- Padding: `2px 6px`, border-radius: `3px`
- `pointer-events: none`, `z-index: 10`
- Optional `className` merged in (does not override inline styles)

**Renderer Display:**
- Reads `terminal.renderer?.backend` (values: `'canvas2d'`, `'webgpu'`, or undefined)
- Displays as `"<backend> <n> fps"` (e.g., `"webgpu 62 fps"`)
- Fallback to `"? <n> fps"` if renderer not ready

**FPS Counter:**
- rAF loop increments frame counter every frame
- Once per second (checked via `performance.now()` delta): write `<backend> <n> fps`, reset counter
- Loop continues until uninstall

**Alt+Shift+R Hotkey** (if `bindToggleHotkey: true`):
- Listens for `keydown` event with `altKey && shiftKey && (key === 'R' || key === 'r')`
- Prevents default
- Reads current backend from `terminal.renderer?.backend`
- Toggles: `'webgpu'` → `'canvas2d'`, anything else → `'webgpu'`
- Creates URL with `?renderer=<next>`, navigates `window.location.href = url.toString()`

**Cleanup:**
- Uninstall function: stops rAF loop, removes DOM element, detaches keydown listener, returns void

### Integration: `BoobaTerminalOptions` 

Add missing field to `ts/booba.ts`:

```ts
export interface BoobaTerminalOptions {
    // ... existing fields ...
    renderer?: 'auto' | 'webgpu' | 'canvas2d';
}
```

The compiled `booba.js` already passes this to the Terminal constructor; this just makes the TypeScript type correct.

### Reference Implementation Migration

Update `serve/static/index.html` to use the new helper:

```javascript
import { BoobaTerminal, installRendererHud, parseRendererFromURL } from './static/booba/booba.js';

const booba = new BoobaTerminal('terminal-container', {
    renderer: parseRendererFromURL(),
    // ... other options ...
});

await booba.init();
booba.connectAuto(urls.wsUrl, urls.wtUrl, urls.certHashUrl);
booba.focus();

// Install renderer HUD
installRendererHud(booba.terminal, { bindToggleHotkey: true });
```

Alternative: keep the inline version with a comment pointing to the new API. Either approach is acceptable; inline kept here for backwards-compatibility demo.

### Exports from `ts/booba.ts`

Re-export the HUD module for convenience:

```ts
export { installRendererHud, parseRendererFromURL, type BoobaRendererHudOptions } from './hud.js';
```

This allows consumers to do:
```ts
import { installRendererHud } from '@nimblemarkets/booba';
```

Or:
```ts
import { installRendererHud } from '@nimblemarkets/booba/hud.js';
```

## Testing

**File:** `ts/hud.test.ts` (vitest)

### Test Cases

1. **`parseRendererFromURL()` — valid param**
   - URL: `?renderer=webgpu` → returns `'webgpu'`
   - URL: `?renderer=canvas2d` → returns `'canvas2d'`
   - URL: `?renderer=auto` → returns `'auto'`

2. **`parseRendererFromURL()` — invalid param**
   - URL: `?renderer=invalid` → falls through to `window.__boobaDefaultRenderer ?? 'auto'`
   - If `window.__boobaDefaultRenderer = 'webgpu'` → returns `'webgpu'`
   - If no param and no `__boobaDefaultRenderer` → returns `'auto'`

3. **`parseRendererFromURL()` — fallback chain**
   - No param, no `__boobaDefaultRenderer` → `'auto'`
   - No param, `__boobaDefaultRenderer = 'canvas2d'` → `'canvas2d'`

4. **`installRendererHud()` — element creation**
   - Call with mock Terminal, verify `<div id="booba-renderer-hud">` is created
   - Verify it's appended to `opts.parent ?? document.body`
   - Verify inline styles are applied
   - Verify initial text is empty or placeholder

5. **`installRendererHud()` — uninstall function**
   - Call uninstall, verify element is removed from DOM
   - Verify rAF loop stops (no more text updates)

6. **`installRendererHud()` — Alt+Shift+R hotkey** (if `bindToggleHotkey: true`)
   - Mock `window.location` and `terminal.renderer`
   - Simulate Alt+Shift+R keydown, verify URL is constructed with next renderer
   - If hotkey disabled, verify no listener attached

### Smoke Test Notes

- Full FPS counting loop is hard to unit-test (depends on rAF timing); smoke test is sufficient
- Verify element updates happen (can mock `performance.now()` to simulate 1s tick)
- Don't test full browser navigation; mock `window.location.href` assignment

## Documentation

### `docs/TYPESCRIPT_API.md` — New Section

Add "Renderer HUD" section:

```markdown
## Renderer HUD (Optional Dev Feature)

Display active renderer backend and live FPS in a corner badge. Useful for debugging which renderer is active and monitoring performance.

### Quick Start

```ts
import { BoobaTerminal, installRendererHud, parseRendererFromURL } from '@nimblemarkets/booba';

const booba = new BoobaTerminal('container', {
  renderer: parseRendererFromURL(), // respects ?renderer= query param
});

await booba.init();
booba.connectAuto(wsUrl, wtUrl, certHashUrl);

// Show HUD in bottom-right corner (viewport-fixed)
installRendererHud(booba.terminal, { bindToggleHotkey: true });
```

### Options

- **`parent`** (HTMLElement): Where to mount the badge. Defaults to `document.body` with fixed positioning. Pass a container element for different placement.
- **`bindToggleHotkey`** (boolean): If true (default), Alt+Shift+R cycles `?renderer=` between `webgpu` and `canvas2d` and reloads.
- **`className`** (string): Optional CSS class name to apply (does not override inline styles; use for additional customization).

### Custom Placement

Mount the badge in your own DOM:

```ts
const header = document.getElementById('my-header');
installRendererHud(booba.terminal, { parent: header, bindToggleHotkey: false });
```

### Query Parameters

`parseRendererFromURL()` reads `?renderer=` from the URL:
- Valid values: `auto`, `webgpu`, `canvas2d`
- Invalid values fall back to `window.__boobaDefaultRenderer` (set by the server), then `auto`

Use this to respect renderer preference across page reloads.
```

## Out of Scope

- Server-rendered `<select>` for renderer choice; hotkey + query param is sufficient
- Separate CSS file; inline styles via JS (matches reference)
- Changing default renderer (`'auto'`)
- Modifying server-side `__boobaDefaultRenderer` injection in `serve/handlers.go`

## Open Decisions Made

1. **Module location:** `ts/hud.ts` (co-located with booba.ts, easy to find and copy)
2. **Positioning:** Default `position: fixed` in viewport, but flexible via `parent` option
3. **Styling:** Inline styles only; consumers can override via `className` + CSS if needed
4. **Hotkey:** On by default (`bindToggleHotkey: true`) but opt-out via options
5. **Export style:** Both direct (`import { installRendererHud }`) and module re-export available

## Success Criteria

- [ ] `BoobaTerminalOptions.renderer` is typed
- [ ] `ts/hud.ts` exports `installRendererHud`, `parseRendererFromURL`, `BoobaRendererHudOptions`
- [ ] HUD preserves all reference implementation behaviors (badge text, hotkey, URL parsing, styling)
- [ ] `serve/static/index.html` either migrates or has comment pointing to new API
- [ ] vitest covers `parseRendererFromURL` with all fallback cases
- [ ] Smoke test for `installRendererHud` (element creation, uninstall)
- [ ] Documentation added to `docs/TYPESCRIPT_API.md`
- [ ] All tests pass
- [ ] ntcharts can call one-liner: `installRendererHud(booba.terminal, { parent: containerEl })`
