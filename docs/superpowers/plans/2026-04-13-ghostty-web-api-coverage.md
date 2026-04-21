# ghostty-web Full API Coverage Implementation Plan

**Status:** Shipped. All 11 tasks landed in `ts/booba.ts`, `ts/types.ts`, and the example page at `serve/static/index.html`. Build output now lives at `serve/static/booba/` (the asset layout was reorganized after this plan was written; original references to `assets/index.html` and `assets/booba/` below have been updated).

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the full ghostty-web Terminal API surface through BoobaTerminal, adding selection, clipboard, scrollback, paste, focus, links, title, bell, cursor config, and expanded terminal options — so consumers of booba get access to everything ghostty-web offers without reaching past the abstraction.

**Architecture:** BoobaTerminal remains the primary public class wrapping ghostty-web's Terminal. We expand `BoobaTerminalOptions` to cover all `ITerminalOptions` fields, add event forwarding for all Terminal events, add methods for selection/clipboard/scroll/focus/paste, and re-export relevant types. The adapter protocol (WebSocket `0x01`/`0x02` messages) is unchanged — mouse events and paste already flow as escape sequences through `onData`/`0x01`. No Go server changes needed.

**Tech Stack:** TypeScript (ES2020), ghostty-web 0.4.0-next.14, compiled via `npx tsc` to `serve/static/booba/`

---

## File Structure

| File | Responsibility | Action |
|------|---------------|--------|
| `ts/booba.ts` | Main BoobaTerminal class — options, events, methods | Modify |
| `ts/adapter.ts` | Adapter interface and implementations | No changes |
| `ts/types.ts` | Re-exported types from ghostty-web | Create |
| `serve/static/index.html` | Example page — update to demo new features | Modify |

---

### Task 1: Expand BoobaTerminalOptions and Terminal construction

**Files:**
- Modify: `ts/booba.ts:1-45`

Currently `BoobaTerminalOptions` only has `fontSize`, `cols`, `rows`, `theme`, and a catch-all index signature. We need to explicitly surface all `ITerminalOptions` fields so consumers get type safety.

- [x] **Step 1: Update BoobaTerminalOptions interface**

Replace the current `BoobaTerminalOptions` and constructor in `ts/booba.ts`:

```typescript
// @ts-ignore - Import will resolve at runtime in browser
import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';
import { BoobaAdapter, BoobaConnectionState, BoobaWebSocketAdapter, BoobaWasmAdapter } from './adapter.js';

export interface BoobaTerminalOptions {
    fontSize?: number;
    fontFamily?: string;
    cols?: number;
    rows?: number;
    cursorBlink?: boolean;
    cursorStyle?: 'block' | 'underline' | 'bar';
    scrollback?: number;
    allowTransparency?: boolean;
    convertEol?: boolean;
    disableStdin?: boolean;
    smoothScrollDuration?: number;
    theme?: {
        foreground?: string;
        background?: string;
        cursor?: string;
        cursorAccent?: string;
        selectionBackground?: string;
        selectionForeground?: string;
        black?: string;
        red?: string;
        green?: string;
        yellow?: string;
        blue?: string;
        magenta?: string;
        cyan?: string;
        white?: string;
        brightBlack?: string;
        brightRed?: string;
        brightGreen?: string;
        brightYellow?: string;
        brightBlue?: string;
        brightMagenta?: string;
        brightCyan?: string;
        brightWhite?: string;
    };
}
```

- [x] **Step 2: Update constructor defaults**

Update the constructor to only set the minimal defaults (fontSize, cols, rows, theme background/foreground) and pass everything else through:

```typescript
constructor(containerId: string, options: BoobaTerminalOptions = {}) {
    this.container = document.getElementById(containerId);
    this.options = {
        fontSize: 14,
        cols: 80,
        rows: 24,
        theme: {
            background: '#1e1e1e',
            foreground: '#d4d4d4',
        },
        ...options
    };
    this.term = null;
    this.adapter = null;
    this.onStatusChange = null;
    this.fitAddon = null;
}
```

(This is the same logic, just confirming the spread covers new fields.)

- [x] **Step 3: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 4: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: expand BoobaTerminalOptions to cover full ghostty-web ITerminalOptions"
```

---

### Task 2: Add selection and clipboard methods

**Files:**
- Modify: `ts/booba.ts` (add methods to BoobaTerminal class)

ghostty-web's Terminal exposes `getSelection()`, `hasSelection()`, `clearSelection()`, `copySelection()`, `selectAll()`, `select()`, `selectLines()`, `getSelectionPosition()`. We forward all of these.

- [x] **Step 1: Add selection methods to BoobaTerminal**

Add these methods to the `BoobaTerminal` class, after the `disconnect()` method:

```typescript
// --- Selection & Clipboard ---

/** Get the currently selected text */
getSelection(): string {
    return this.term?.getSelection() ?? '';
}

/** Check if there's an active selection */
hasSelection(): boolean {
    return this.term?.hasSelection() ?? false;
}

/** Clear the current selection */
clearSelection(): void {
    this.term?.clearSelection();
}

/** Copy the current selection to clipboard. Returns true if text was copied. */
copySelection(): boolean {
    return this.term?.copySelection() ?? false;
}

/** Select all text in the terminal */
selectAll(): void {
    this.term?.selectAll();
}

/** Select text at a specific position */
select(column: number, row: number, length: number): void {
    this.term?.select(column, row, length);
}

/** Select entire lines from start to end (inclusive) */
selectLines(start: number, end: number): void {
    this.term?.selectLines(start, end);
}

/** Get the selection position as a buffer range, or undefined if no selection */
getSelectionPosition(): { start: { x: number; y: number }; end: { x: number; y: number } } | undefined {
    return this.term?.getSelectionPosition();
}
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: add selection and clipboard methods to BoobaTerminal"
```

---

### Task 3: Add scrollback and viewport methods

**Files:**
- Modify: `ts/booba.ts` (add methods to BoobaTerminal class)

ghostty-web's Terminal has `scrollLines()`, `scrollPages()`, `scrollToTop()`, `scrollToBottom()`, `scrollToLine()`, `getViewportY()`. We forward all of these.

- [x] **Step 1: Add scroll methods to BoobaTerminal**

Add these methods after the selection methods:

```typescript
// --- Scrollback & Viewport ---

/** Scroll by a number of lines (positive = down/towards bottom, negative = up/towards history) */
scrollLines(amount: number): void {
    this.term?.scrollLines(amount);
}

/** Scroll by a number of pages */
scrollPages(amount: number): void {
    this.term?.scrollPages(amount);
}

/** Scroll to the top of the scrollback buffer */
scrollToTop(): void {
    this.term?.scrollToTop();
}

/** Scroll to the bottom (current output) */
scrollToBottom(): void {
    this.term?.scrollToBottom();
}

/** Scroll to a specific line in the buffer */
scrollToLine(line: number): void {
    this.term?.scrollToLine(line);
}

/** Get the current viewport Y position (lines scrolled back from bottom) */
getViewportY(): number {
    return this.term?.getViewportY() ?? 0;
}
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: add scrollback and viewport methods to BoobaTerminal"
```

---

### Task 4: Add paste, focus, clear, and reset methods

**Files:**
- Modify: `ts/booba.ts` (add methods to BoobaTerminal class)

- [x] **Step 1: Add terminal control methods**

Add after the scroll methods:

```typescript
// --- Terminal Control ---

/** Paste text into the terminal (uses bracketed paste if the program supports it) */
paste(data: string): void {
    this.term?.paste(data);
}

/** Input data as if typed by the user */
input(data: string): void {
    this.term?.input(data, true);
}

/** Focus the terminal */
focus(): void {
    this.term?.focus();
}

/** Remove focus from the terminal */
blur(): void {
    this.term?.blur();
}

/** Clear the terminal screen */
clear(): void {
    this.term?.clear();
}

/** Reset the terminal state */
reset(): void {
    this.term?.reset();
}

/** Write data to the terminal display */
write(data: string | Uint8Array, callback?: () => void): void {
    this.term?.write(data, callback);
}

/** Write data with a trailing newline */
writeln(data: string | Uint8Array, callback?: () => void): void {
    this.term?.writeln(data, callback);
}
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: add paste, focus, clear, reset, write methods to BoobaTerminal"
```

---

### Task 5: Add terminal mode query methods

**Files:**
- Modify: `ts/booba.ts` (add methods to BoobaTerminal class)

These let consumers check what the connected BubbleTea program has enabled.

- [x] **Step 1: Add mode query methods**

Add after the terminal control methods:

```typescript
// --- Terminal Mode Queries ---

/** Check if the program has enabled mouse tracking */
hasMouseTracking(): boolean {
    return this.term?.hasMouseTracking() ?? false;
}

/** Check if the program has enabled bracketed paste mode */
hasBracketedPaste(): boolean {
    return this.term?.hasBracketedPaste() ?? false;
}

/** Check if the program has enabled focus event reporting */
hasFocusEvents(): boolean {
    return this.term?.hasFocusEvents() ?? false;
}

/** Query an arbitrary terminal mode by number */
getMode(mode: number, isAnsi?: boolean): boolean {
    return this.term?.getMode(mode, isAnsi) ?? false;
}
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: add terminal mode query methods to BoobaTerminal"
```

---

### Task 6: Add event forwarding for all Terminal events

**Files:**
- Modify: `ts/booba.ts` (add event properties and wire them up in `init()`)

ghostty-web's Terminal fires: `onData`, `onResize`, `onBell`, `onSelectionChange`, `onKey`, `onTitleChange`, `onScroll`, `onRender`, `onCursorMove`. Currently booba only uses `onData` and `onResize` internally (in `init()` and `_setupAdapter()`). We need to expose all events to consumers.

- [x] **Step 1: Add event callback properties to BoobaTerminal**

Add these properties to the class, alongside the existing `onStatusChange`:

```typescript
// --- Event Callbacks ---
onStatusChange: ((state: string, message: string) => void) | null;
onBell: (() => void) | null;
onSelectionChange: (() => void) | null;
onKey: ((event: { key: string; domEvent: KeyboardEvent }) => void) | null;
onTitleChange: ((title: string) => void) | null;
onScroll: ((viewportY: number) => void) | null;
onRender: ((event: { start: number; end: number }) => void) | null;
onCursorMove: (() => void) | null;
```

- [x] **Step 2: Initialize new callbacks to null in constructor**

In the constructor, after `this.fitAddon = null;`:

```typescript
this.onBell = null;
this.onSelectionChange = null;
this.onKey = null;
this.onTitleChange = null;
this.onScroll = null;
this.onRender = null;
this.onCursorMove = null;
```

- [x] **Step 3: Wire up event forwarding in init()**

In the `init()` method, after the existing `this.term.onData(...)` block, add:

```typescript
// Forward terminal events to consumer callbacks
this.term.onBell(() => {
    this.onBell?.();
});

this.term.onSelectionChange(() => {
    this.onSelectionChange?.();
});

this.term.onKey((event: { key: string; domEvent: KeyboardEvent }) => {
    this.onKey?.(event);
});

this.term.onTitleChange((title: string) => {
    this.onTitleChange?.(title);
});

this.term.onScroll((viewportY: number) => {
    this.onScroll?.(viewportY);
});

this.term.onRender((event: { start: number; end: number }) => {
    this.onRender?.(event);
});

this.term.onCursorMove(() => {
    this.onCursorMove?.();
});
```

- [x] **Step 4: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 5: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: forward all ghostty-web Terminal events through BoobaTerminal"
```

---

### Task 7: Add link provider registration and custom key/wheel handlers

**Files:**
- Modify: `ts/booba.ts` (add methods to BoobaTerminal class)

- [x] **Step 1: Add link and handler methods**

Add after the mode query methods:

```typescript
// --- Link Detection ---

/**
 * Register a link provider for detecting clickable links in terminal output.
 * Multiple providers can be registered (e.g., one for OSC 8, one for URL regex).
 */
registerLinkProvider(provider: { provideLinks(y: number, callback: (links: any[] | undefined) => void): void; dispose?(): void }): void {
    this.term?.registerLinkProvider(provider);
}

// --- Custom Event Handlers ---

/**
 * Attach a custom keyboard event handler.
 * Return true from the handler to prevent default terminal handling.
 */
attachCustomKeyEventHandler(handler: (event: KeyboardEvent) => boolean): void {
    this.term?.attachCustomKeyEventHandler(handler);
}

/**
 * Attach a custom wheel event handler.
 * Return true from the handler to prevent default scroll handling.
 */
attachCustomWheelEventHandler(handler?: (event: WheelEvent) => boolean): void {
    this.term?.attachCustomWheelEventHandler(handler);
}
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: add link provider registration and custom event handlers"
```

---

### Task 8: Add dispose method and direct terminal/buffer access

**Files:**
- Modify: `ts/booba.ts` (add methods to BoobaTerminal class)

Consumers may need the underlying Terminal for advanced use cases. Expose it as a read-only getter alongside a proper dispose lifecycle method.

- [x] **Step 1: Add dispose and terminal access**

Add to the BoobaTerminal class:

```typescript
// --- Lifecycle ---

/** Dispose the terminal and clean up all resources */
dispose(): void {
    this.disconnect();
    this.term?.dispose();
    this.term = null;
    this.fitAddon = null;
}

// --- Advanced Access ---

/**
 * Get the underlying ghostty-web Terminal instance for advanced use cases.
 * Returns null if init() hasn't been called yet.
 */
get terminal(): any {
    return this.term;
}

/** Get the current terminal dimensions */
get cols(): number {
    return this.term?.cols ?? 0;
}

/** Get the current terminal dimensions */
get rows(): number {
    return this.term?.rows ?? 0;
}
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 3: Commit**

```bash
git add ts/booba.ts
git commit -m "feat: add dispose lifecycle and direct terminal access"
```

---

### Task 9: Create types.ts with re-exported types

**Files:**
- Create: `ts/types.ts`
- Modify: `ts/booba.ts` (add re-export)

Provide type definitions that consumers can import without reaching into ghostty-web directly.

- [x] **Step 1: Create ts/types.ts**

```typescript
/**
 * Booba type definitions
 * 
 * Re-exports of ghostty-web types that are part of booba's public API,
 * plus booba-specific types.
 */

/** Terminal theme colors */
export interface BoobaTheme {
    foreground?: string;
    background?: string;
    cursor?: string;
    cursorAccent?: string;
    selectionBackground?: string;
    selectionForeground?: string;
    black?: string;
    red?: string;
    green?: string;
    yellow?: string;
    blue?: string;
    magenta?: string;
    cyan?: string;
    white?: string;
    brightBlack?: string;
    brightRed?: string;
    brightGreen?: string;
    brightYellow?: string;
    brightBlue?: string;
    brightMagenta?: string;
    brightCyan?: string;
    brightWhite?: string;
}

/** Buffer range for selection positions */
export interface BoobaBufferRange {
    start: { x: number; y: number };
    end: { x: number; y: number };
}

/** Keyboard event from the terminal */
export interface BoobaKeyEvent {
    key: string;
    domEvent: KeyboardEvent;
}

/** Render event range */
export interface BoobaRenderEvent {
    start: number;
    end: number;
}

/** Link provider interface for registering custom link detection */
export interface BoobaLinkProvider {
    provideLinks(y: number, callback: (links: BoobaLink[] | undefined) => void): void;
    dispose?(): void;
}

/** A detected link in the terminal */
export interface BoobaLink {
    text: string;
    range: BoobaBufferRange;
    activate(event: MouseEvent): void;
    hover?(isHovered: boolean): void;
    dispose?(): void;
}
```

- [x] **Step 2: Re-export types from booba.ts**

Add to the bottom of `ts/booba.ts`, alongside the existing re-exports:

```typescript
export type { BoobaTheme, BoobaBufferRange, BoobaKeyEvent, BoobaRenderEvent, BoobaLinkProvider, BoobaLink } from './types.js';
```

- [x] **Step 3: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors

- [x] **Step 4: Commit**

```bash
git add ts/types.ts ts/booba.ts
git commit -m "feat: add booba type definitions for public API surface"
```

---

### Task 10: Update example page to demonstrate new features

**Files:**
- Modify: `serve/static/index.html`

Add title bar updating from `onTitleChange`, selection copy button, and scroll indicator to demonstrate the new API surface.

- [x] **Step 1: Update the title bar to respond to onTitleChange**

In `serve/static/index.html`, after `booba.onStatusChange = ...`, add:

```javascript
booba.onTitleChange = (title) => {
    document.querySelector('.title').textContent = title || 'booba-view-example';
};

booba.onBell = () => {
    // Brief visual flash on bell
    const container = document.getElementById('terminal-container');
    container.style.outline = '2px solid #ffbd2e';
    setTimeout(() => { container.style.outline = 'none'; }, 150);
};
```

- [x] **Step 2: Focus terminal after connection**

After `booba.connectWebSocket(wsUrl);`, add:

```javascript
// Focus terminal for immediate keyboard input
booba.focus();
```

- [x] **Step 3: Verify the HTML is valid**

Open in browser or visually inspect. No build step needed for HTML.

- [x] **Step 4: Commit**

```bash
git add serve/static/index.html
git commit -m "feat: update example page to demonstrate title, bell, and focus"
```

---

### Task 11: Build and verify

**Files:**
- All modified files

- [x] **Step 1: Full TypeScript build**

Run: `npx tsc`
Expected: Clean build, no errors. Output files in `serve/static/booba/`.

- [x] **Step 2: Verify output files exist**

Run: `ls -la serve/static/booba/booba.js serve/static/booba/booba.d.ts serve/static/booba/types.js serve/static/booba/types.d.ts`
Expected: All four files present with recent timestamps.

- [x] **Step 3: Verify the declaration file exports all new methods**

Run: `grep -E '(getSelection|scrollLines|paste|focus|hasMouseTracking|onBell|onTitleChange|registerLinkProvider|dispose|terminal)' serve/static/booba/booba.d.ts`
Expected: All new methods/properties appear in the declaration output.

- [x] **Step 4: Commit build output**

```bash
git add serve/static/booba/
git commit -m "build: compile TypeScript with full ghostty-web API coverage"
```

---

## Summary of API coverage after implementation

| ghostty-web Terminal API | Booba coverage |
|--------------------------|----------------|
| `write()`, `writeln()` | `write()`, `writeln()` |
| `paste()`, `input()` | `paste()`, `input()` |
| `resize()` | Handled internally by FitAddon |
| `clear()`, `reset()` | `clear()`, `reset()` |
| `focus()`, `blur()` | `focus()`, `blur()` |
| `open()`, `dispose()` | `init()`, `dispose()` |
| `loadAddon()` | Used internally (FitAddon) |
| `getSelection()` et al. | Full selection API forwarded |
| `scrollLines()` et al. | Full scroll API forwarded |
| `registerLinkProvider()` | `registerLinkProvider()` |
| `attachCustomKeyEventHandler()` | `attachCustomKeyEventHandler()` |
| `attachCustomWheelEventHandler()` | `attachCustomWheelEventHandler()` |
| `hasMouseTracking()` et al. | Full mode query API forwarded |
| `getMode()` | `getMode()` |
| `onData` | Used internally by adapter |
| `onResize` | Used internally by adapter |
| `onBell` | `onBell` callback |
| `onSelectionChange` | `onSelectionChange` callback |
| `onKey` | `onKey` callback |
| `onTitleChange` | `onTitleChange` callback |
| `onScroll` | `onScroll` callback |
| `onRender` | `onRender` callback |
| `onCursorMove` | `onCursorMove` callback |
| `buffer`, `unicode`, `options` | Accessible via `terminal` getter |
| `cols`, `rows` | Direct getters |
