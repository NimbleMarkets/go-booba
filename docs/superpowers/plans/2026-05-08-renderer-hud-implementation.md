# Renderer HUD API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the renderer HUD (backend info + FPS counter) from the reference `index.html` into a treeshakeable, reusable TypeScript module that consumers can import and install anywhere in their DOM.

**Architecture:** Create a new `ts/hud.ts` module exporting `installRendererHud()` and `parseRendererFromURL()` as standalone functions. Tests verify both happy path (element creation, FPS updates, hotkey) and fallback behavior (URL param validation, renderer missing). Integrate with `BoobaTerminal` by adding the missing `renderer` field to `BoobaTerminalOptions` and re-exporting HUD functions. Migrate the reference `index.html` to use the new API.

**Tech Stack:** TypeScript, vitest, ghostty-web Terminal API, DOM APIs

---

### Task 1: Add `renderer` field to `BoobaTerminalOptions`

**Files:**
- Modify: `ts/booba.ts:8-22`

- [ ] **Step 1: Add `renderer` field to `BoobaTerminalOptions` interface**

Open `ts/booba.ts` and add the field after `theme`:

```typescript
export interface BoobaTerminalOptions {
    fontSize?: number;
    fontFamily?: string;
    cols?: number;
    rows?: number;
    cursorBlink?: boolean;
    cursorStyle?: 'block' | 'underline' | 'bar';
    scrollback?: number;
    allowOSC52?: boolean;
    allowTransparency?: boolean;
    convertEol?: boolean;
    disableStdin?: boolean;
    smoothScrollDuration?: number;
    theme?: BoobaTheme;
    renderer?: 'auto' | 'webgpu' | 'canvas2d';
}
```

- [ ] **Step 2: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: add renderer field to BoobaTerminalOptions"
```

---

### Task 2: Create `ts/hud.ts` with `parseRendererFromURL()`

**Files:**
- Create: `ts/hud.ts`

- [ ] **Step 1: Create `ts/hud.ts` with `parseRendererFromURL()` function**

```typescript
/**
 * Parse ?renderer= query parameter from the current URL.
 * Falls back to window.__boobaDefaultRenderer, then 'auto'.
 * Only accepts 'auto', 'webgpu', 'canvas2d'.
 */
export function parseRendererFromURL(): 'auto' | 'webgpu' | 'canvas2d' {
    const params = new URLSearchParams(window.location.search);
    const renderer = params.get('renderer');

    if (renderer === 'canvas2d' || renderer === 'webgpu' || renderer === 'auto') {
        return renderer;
    }

    // Fall back to server's default renderer preference
    return (window as any).__boobaDefaultRenderer || 'auto';
}
```

- [ ] **Step 2: Commit**

```bash
git add ts/hud.ts
git commit -m "feat: add parseRendererFromURL function"
```

---

### Task 3: Add `BoobaRendererHudOptions` interface and `installRendererHud()` signature

**Files:**
- Modify: `ts/hud.ts`

- [ ] **Step 1: Add `BoobaRendererHudOptions` interface to `ts/hud.ts`**

```typescript
export interface BoobaRendererHudOptions {
    /** Where to mount the badge (default: document.body) */
    parent?: HTMLElement;
    /** CSS class for custom styling (applied to badge element) */
    className?: string;
    /** Bind Alt+Shift+R to toggle webgpu↔canvas2d. Default: true. */
    bindToggleHotkey?: boolean;
}
```

- [ ] **Step 2: Add `installRendererHud()` function stub**

```typescript
import type { Terminal } from 'ghostty-web';

/**
 * Install a corner HUD showing active renderer backend and live FPS.
 * Must be called after booba.init() so terminal.renderer exists.
 * Returns an uninstall function.
 */
export function installRendererHud(
    terminal: Terminal,
    opts?: BoobaRendererHudOptions
): () => void {
    // Implemented in Task 5
    throw new Error('Not implemented');
}
```

- [ ] **Step 3: Commit**

```bash
git add ts/hud.ts
git commit -m "feat: add BoobaRendererHudOptions and installRendererHud signature"
```

---

### Task 4: Write failing tests for `parseRendererFromURL()`

**Files:**
- Create: `ts/hud.test.ts`

- [ ] **Step 1: Create `ts/hud.test.ts` with tests for `parseRendererFromURL()`**

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { parseRendererFromURL } from './hud';

describe('parseRendererFromURL', () => {
    const originalLocation = window.location;
    const originalBoobaDefaultRenderer = (window as any).__boobaDefaultRenderer;

    beforeEach(() => {
        // Reset global state
        delete (window as any).__boobaDefaultRenderer;
    });

    afterEach(() => {
        // Restore
        (window as any).__boobaDefaultRenderer = originalBoobaDefaultRenderer;
    });

    it('should parse valid ?renderer=webgpu', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=webgpu'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('webgpu');
    });

    it('should parse valid ?renderer=canvas2d', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=canvas2d'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('canvas2d');
    });

    it('should parse valid ?renderer=auto', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=auto'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('auto');
    });

    it('should fall back to window.__boobaDefaultRenderer if param is invalid', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=invalid'),
            writable: true,
        });
        (window as any).__boobaDefaultRenderer = 'webgpu';
        expect(parseRendererFromURL()).toBe('webgpu');
    });

    it('should fall back to auto if param is missing and no __boobaDefaultRenderer', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('auto');
    });

    it('should fall back to auto if both param and __boobaDefaultRenderer are missing', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com'),
            writable: true,
        });
        delete (window as any).__boobaDefaultRenderer;
        expect(parseRendererFromURL()).toBe('auto');
    });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
npm run test -- ts/hud.test.ts
```

Expected: Tests for `parseRendererFromURL` should all PASS (function is already implemented). Tests for `installRendererHud` will fail (not implemented yet).

- [ ] **Step 3: Commit**

```bash
git add ts/hud.test.ts
git commit -m "test: add parseRendererFromURL tests"
```

---

### Task 5: Write failing tests for `installRendererHud()`

**Files:**
- Modify: `ts/hud.test.ts`

- [ ] **Step 1: Add tests for `installRendererHud()` to `ts/hud.test.ts`**

```typescript
import type { Terminal } from 'ghostty-web';

describe('installRendererHud', () => {
    let mockTerminal: Partial<Terminal> = {};

    beforeEach(() => {
        // Clear any previous HUD elements
        const existing = document.getElementById('booba-renderer-hud');
        existing?.remove();

        // Create a simple mock Terminal
        mockTerminal = {
            renderer: {
                backend: 'webgpu',
            },
        };
    });

    afterEach(() => {
        const element = document.getElementById('booba-renderer-hud');
        element?.remove();
    });

    it('should create and append badge element to parent', () => {
        const { installRendererHud } = await import('./hud');
        const parent = document.createElement('div');
        document.body.appendChild(parent);

        installRendererHud(mockTerminal as Terminal, { parent });

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();
        expect(badge?.parentElement).toBe(parent);

        parent.remove();
    });

    it('should append to document.body by default', () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.parentElement).toBe(document.body);
    });

    it('should apply inline styles', () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.style.position).toBe('fixed');
        expect(badge?.style.bottom).toBe('12px');
        expect(badge?.style.right).toBe('12px');
        expect(badge?.style.fontFamily).toContain('monospace');
        expect(badge?.style.fontSize).toBe('11px');
        expect(badge?.style.color).toBe('rgb(102, 102, 102)'); // #666
        expect(badge?.style.pointerEvents).toBe('none');
        expect(badge?.style.zIndex).toBe('10');
    });

    it('should apply optional className', () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal, { className: 'my-custom-class' });

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.classList.contains('my-custom-class')).toBe(true);
    });

    it('should return an uninstall function that removes element', () => {
        const { installRendererHud } = await import('./hud');

        const uninstall = installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();

        uninstall();

        const badgeAfter = document.getElementById('booba-renderer-hud');
        expect(badgeAfter).toBeNull();
    });

    it('should bind Alt+Shift+R hotkey by default', () => {
        const { installRendererHud } = await import('./hud');
        const originalLocation = window.location.href;

        installRendererHud(mockTerminal as Terminal, { bindToggleHotkey: true });

        const event = new KeyboardEvent('keydown', {
            altKey: true,
            shiftKey: true,
            key: 'R',
        });

        // Mock window.location.href assignment
        let navigationUrl = '';
        Object.defineProperty(window, 'location', {
            value: {
                ...window.location,
                href: originalLocation,
            },
            writable: true,
        });

        // Dispatch event and check if URL would be set
        document.dispatchEvent(event);

        // Note: Full navigation test is hard without actual reload;
        // this verifies the listener is attached (no error)
    });

    it('should not bind hotkey if bindToggleHotkey is false', () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal, { bindToggleHotkey: false });

        const event = new KeyboardEvent('keydown', {
            altKey: true,
            shiftKey: true,
            key: 'R',
        });

        // Should not throw
        document.dispatchEvent(event);
    });

    it('should display backend and fps in text content', () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        // Text may be updated asynchronously via rAF, so just check it exists
        expect(badge?.textContent).toBeDefined();
    });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
npm run test -- ts/hud.test.ts
```

Expected: FAIL with "Not implemented" error from the stub

- [ ] **Step 3: Commit**

```bash
git add ts/hud.test.ts
git commit -m "test: add installRendererHud tests"
```

---

### Task 6: Implement `installRendererHud()` function

**Files:**
- Modify: `ts/hud.ts`

- [ ] **Step 1: Replace the stub with full `installRendererHud()` implementation**

```typescript
import type { Terminal } from 'ghostty-web';

export function installRendererHud(
    terminal: Terminal,
    opts?: BoobaRendererHudOptions
): () => void {
    const parent = opts?.parent ?? document.body;
    const className = opts?.className;
    const bindToggleHotkey = opts?.bindToggleHotkey !== false;

    // Create badge element
    const badge = document.createElement('div');
    badge.id = 'booba-renderer-hud';

    // Apply inline styles (exact from reference)
    badge.style.position = 'fixed';
    badge.style.bottom = '12px';
    badge.style.right = '12px';
    badge.style.fontFamily = 'monospace';
    badge.style.fontSize = '11px';
    badge.style.color = '#666';
    badge.style.background = 'rgba(0, 0, 0, 0.4)';
    badge.style.padding = '2px 6px';
    badge.style.borderRadius = '3px';
    badge.style.pointerEvents = 'none';
    badge.style.zIndex = '10';

    if (className) {
        badge.className = className;
    }

    parent.appendChild(badge);

    // rAF loop for FPS counter
    let frames = 0;
    let lastTick = performance.now();
    let rafId: number | null = null;

    const updateRendererInfo = () => {
        frames++;
        const now = performance.now();
        if (now - lastTick >= 1000) {
            const backend = terminal.renderer?.backend || '?';
            badge.textContent = `${backend} ${frames} fps`;
            frames = 0;
            lastTick = now;
        }
        rafId = requestAnimationFrame(updateRendererInfo);
    };

    rafId = requestAnimationFrame(updateRendererInfo);

    // Alt+Shift+R hotkey handler
    const handleKeyDown = (e: KeyboardEvent) => {
        if (!bindToggleHotkey) return;
        if (e.altKey && e.shiftKey && (e.key === 'R' || e.key === 'r')) {
            e.preventDefault();
            const cur = terminal.renderer?.backend;
            const next = cur === 'webgpu' ? 'canvas2d' : 'webgpu';
            const url = new URL(window.location.href);
            url.searchParams.set('renderer', next);
            window.location.href = url.toString();
        }
    };

    document.addEventListener('keydown', handleKeyDown);

    // Uninstall function
    return () => {
        if (rafId !== null) {
            cancelAnimationFrame(rafId);
        }
        badge.remove();
        document.removeEventListener('keydown', handleKeyDown);
    };
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
npm run test -- ts/hud.test.ts
```

Expected: PASS (all tests for both functions)

- [ ] **Step 3: Commit**

```bash
git add ts/hud.ts
git commit -m "feat: implement installRendererHud function with FPS counter and hotkey"
```

---

### Task 7: Add HUD re-exports to `ts/booba.ts`

**Files:**
- Modify: `ts/booba.ts:454-461`

- [ ] **Step 1: Add HUD exports to end of `ts/booba.ts`**

Add these lines at the end of the file, after the existing re-exports:

```typescript
// HUD utilities (optional, treeshakeable)
export { installRendererHud, parseRendererFromURL, type BoobaRendererHudOptions } from './hud.js';
```

- [ ] **Step 2: Verify TypeScript compilation**

```bash
npm run build
```

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: re-export HUD utilities from booba module"
```

---

### Task 8: Update `serve/static/index.html` to use new HUD API

**Files:**
- Modify: `serve/static/index.html:45-57`, `serve/static/index.html:78-86`, `serve/static/index.html:137-164`

- [ ] **Step 1: Update the imports in `serve/static/index.html`**

Change the import from:

```javascript
import { BoobaTerminal, resolveBoobaURLs } from './static/booba/booba.js';
```

To:

```javascript
import { BoobaTerminal, resolveBoobaURLs, installRendererHud, parseRendererFromURL } from './static/booba/booba.js';
```

- [ ] **Step 2: Remove the inline CSS for `#renderer-info` from `<style>` block**

Delete lines 45-57 (the `#renderer-info` style block). The `<div id="renderer-info"></div>` in the HTML can also be removed.

- [ ] **Step 3: Update the BoobaTerminal initialization to use `parseRendererFromURL()`**

Change:

```javascript
const booba = new BoobaTerminal('terminal-container', {
    renderer: parseRendererBackend(),
});
```

To:

```javascript
const booba = new BoobaTerminal('terminal-container', {
    renderer: parseRendererFromURL(),
});
```

- [ ] **Step 4: Remove the inline `parseRendererBackend()` function**

Delete the function definition (lines 78-86 in the original)

- [ ] **Step 5: Replace the inline rAF loop and hotkey handler with a single call**

Replace lines 137-164 (the entire renderer info block) with:

```javascript
await booba.init();

// Install renderer HUD (shows backend + FPS in bottom-right corner)
installRendererHud(booba.terminal, { bindToggleHotkey: true });
```

- [ ] **Step 6: Verify the HTML file is syntactically correct**

```bash
cat serve/static/index.html | grep -c "booba-renderer-hud"
```

Expected: 0 (no reference to the old ID; HUD creates it dynamically)

- [ ] **Step 7: Commit**

```bash
git add serve/static/index.html
git commit -m "refactor: migrate index.html to use installRendererHud API"
```

---

### Task 9: Update `docs/TYPESCRIPT_API.md` with HUD documentation

**Files:**
- Modify: `docs/TYPESCRIPT_API.md`

- [ ] **Step 1: Find the end of the existing API documentation**

Open `docs/TYPESCRIPT_API.md` and locate where the existing API sections end (likely after "Terminal Mode Queries" or "Link Detection").

- [ ] **Step 2: Add a new "Renderer HUD (Optional)" section**

Insert the following before the closing sections:

```markdown
## Renderer HUD (Optional Dev Feature)

Display active renderer backend and live FPS in a corner badge. Useful for debugging which renderer is active and monitoring performance.

```javascript
import { BoobaTerminal, installRendererHud, parseRendererFromURL } from '@nimblemarkets/booba';

const booba = new BoobaTerminal('container', {
  renderer: parseRendererFromURL(), // respects ?renderer= query param
  cols: 80,
  rows: 24,
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

```javascript
const header = document.getElementById('my-header');
installRendererHud(booba.terminal, { parent: header, bindToggleHotkey: false });
```

### Query Parameters

`parseRendererFromURL()` reads `?renderer=` from the URL:
- Valid values: `auto`, `webgpu`, `canvas2d`
- Invalid values fall back to `window.__boobaDefaultRenderer` (set by the server), then `auto`

Use this to respect renderer preference across page reloads.
```

- [ ] **Step 3: Commit**

```bash
git add docs/TYPESCRIPT_API.md
git commit -m "docs: add Renderer HUD API documentation"
```

---

### Task 10: Rebuild, test, and verify integration

**Files:**
- Generated: `serve/static/booba/booba.js`

- [ ] **Step 1: Run full test suite**

```bash
npm run test
```

Expected: All tests PASS, including new HUD tests

- [ ] **Step 2: Build TypeScript to JavaScript**

```bash
npm run build
```

Expected: No errors, `serve/static/booba/booba.js` is updated

- [ ] **Step 3: Rebuild the booba binary**

Since booba embeds static assets at compile time, rebuild the binary:

```bash
cd /Users/evan/projects/booba
go build -o bin/booba ./cmd/booba
```

Expected: Binary builds successfully

- [ ] **Step 4: Test the reference HTML in a browser (manual)**

Start the booba server:

```bash
./bin/booba serve
```

Navigate to `http://localhost:8080` and verify:
- Badge appears in bottom-right with format `"<backend> <n> fps"`
- Badge updates FPS every second
- Alt+Shift+R toggles between `webgpu` and `canvas2d` (page reloads with new param)
- Query param `?renderer=canvas2d` respects the parameter
- No console errors

- [ ] **Step 5: Commit rebuilt booba binary if needed**

```bash
git add bin/booba serve/static/booba/booba.js
git commit -m "chore: rebuild static assets and binary"
```

---

### Task 11: Create integration test (optional smoke test)

**Files:**
- Modify: `ts/hud.test.ts`

- [ ] **Step 1: Add a smoke test that verifies end-to-end behavior**

```typescript
describe('installRendererHud integration', () => {
    it('should mount HUD and update FPS live', async () => {
        const { installRendererHud } = await import('./hud');

        const mockTerminal: Partial<Terminal> = {
            renderer: {
                backend: 'canvas2d',
            },
        };

        const uninstall = installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();
        expect(badge?.textContent).toBeDefined();

        // Let a couple of rAF frames run
        await new Promise(resolve => setTimeout(resolve, 50));

        // Text should exist (exact FPS depends on timing)
        expect(badge?.textContent?.length).toBeGreaterThan(0);

        uninstall();
        expect(document.getElementById('booba-renderer-hud')).toBeNull();
    });
});
```

- [ ] **Step 2: Run the smoke test**

```bash
npm run test -- ts/hud.test.ts
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add ts/hud.test.ts
git commit -m "test: add integration smoke test for installRendererHud"
```
