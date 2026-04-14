// @ts-ignore - Import will resolve at runtime in browser
import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';
import { BoobaAdapter, BoobaConnectionState, BoobaWasmAdapter } from './adapter.js';
import { BoobaProtocolAdapter, type WebSocketAdapterCallbacks } from './websocket_adapter.js';
import { BoobaAutoAdapter } from './auto_adapter.js';
import { OSC52Scanner } from './clipboard.js';
import type { BoobaTheme, BoobaBufferRange, BoobaLinkProvider } from './types.js';

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
    theme?: BoobaTheme;
}

export class BoobaTerminal {
    container: HTMLElement | null = null;
    options: BoobaTerminalOptions;
    term: any = null;
    adapter: BoobaAdapter | null = null;
    fitAddon: FitAddon | null = null;
    private _resizeHandler: (() => void) | null = null;
    private osc52Scanner: OSC52Scanner = new OSC52Scanner();

    // --- Event Callbacks ---
    onStatusChange: ((state: string, message: string) => void) | null = null;
    onBell: (() => void) | null = null;
    onSelectionChange: (() => void) | null = null;
    onKey: ((event: { key: string; domEvent: KeyboardEvent }) => void) | null = null;
    onTitleChange: ((title: string) => void) | null = null;
    onScroll: ((viewportY: number) => void) | null = null;
    onRender: ((event: { start: number; end: number }) => void) | null = null;
    onCursorMove: (() => void) | null = null;

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
    }

    async init() {
        await init();
        this.term = new Terminal(this.options);

        this.fitAddon = new FitAddon();
        this.term.loadAddon(this.fitAddon);

        this.term.open(this.container);
        this.fitAddon.fit();
        this.fitAddon.observeResize();

        // Handle window resize as fallback
        this._resizeHandler = () => { this.fitAddon?.fit(); };
        window.addEventListener('resize', this._resizeHandler);

        // Listen for resize events from the terminal (triggered by fit addon)
        this.term.onResize((size: { cols: number; rows: number }) => {
            this.adapter?.boobaResize(size.cols, size.rows);
        });

        console.log('Terminal opened. Size:', this.term.cols, 'x', this.term.rows);

        // Send user input through adapter
        this.term.onData((data: string) => {
            this.adapter?.boobaWrite(data);
        });

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
    }

    /**
     * Connect to a BubbleTea backend via WebSocket
     * @param url WebSocket URL (e.g., 'ws://localhost:8080/ws')
     */
    connectWebSocket(url: string) {
        const callbacks: WebSocketAdapterCallbacks = {
            onTitle: (title) => { this.onTitleChange?.(title); },
            onOptions: (_opts) => { /* store readOnly state if needed */ },
            onClose: (reason) => {
                this.term?.write(`\r\n${reason}\r\n`);
            },
        };
        this.adapter = new BoobaProtocolAdapter(url, callbacks);
        this._setupAdapter();
    }

    /**
     * Connect with auto-detection: tries WebTransport first, falls back to WebSocket.
     * @param wsUrl WebSocket URL (e.g., 'ws://localhost:8080/ws')
     * @param wtUrl WebTransport URL (e.g., 'https://localhost:8081/wt'), or null to skip
     * @param certHashUrl URL to fetch cert hash (e.g., 'http://localhost:8080/cert-hash'), or null
     */
    connectAuto(wsUrl: string, wtUrl: string | null = null, certHashUrl: string | null = null) {
        const callbacks = {
            onTitle: (title: string) => { this.onTitleChange?.(title); },
            onOptions: (_opts: any) => {},
            onClose: (reason: string) => { this.term?.write(`\r\n${reason}\r\n`); },
        };
        this.adapter = new BoobaAutoAdapter(wsUrl, wtUrl, certHashUrl, callbacks);
        this._setupAdapter();
    }

    /**
     * Connect to a BubbleTea program running in WASM
     * @param pollMs Polling interval in milliseconds (default: 16ms / ~60fps)
     */
    connectWasm(pollMs: number = 16) {
        this.adapter = new BoobaWasmAdapter(pollMs);
        this._setupAdapter();
    }

    /**
     * Use a custom adapter implementation
     * @param adapter Custom BubbleTeaAdapter
     */
    connectAdapter(adapter: BoobaAdapter) {
        this.adapter = adapter;
        this._setupAdapter();
    }

    private _setupAdapter() {
        if (!this.adapter) return;
        this.adapter.connect(
            (data: string | Uint8Array) => {
                if (data instanceof Uint8Array) {
                    this.osc52Scanner.scan(data);
                }
                this.term.write(data);
            },
            (state: BoobaConnectionState, message: string) => {
                this._updateStatus(state, message);
                if (state === 'connected' && this.term) {
                    this.adapter?.boobaResize(this.term.cols, this.term.rows);
                }
                if (state === 'disconnected') {
                    this.term.write('\r\nConnection closed.\r\n');
                }
            }
        );
    }

    disconnect() {
        this.adapter?.disconnect();
        this.adapter = null;
    }

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
    getSelectionPosition(): BoobaBufferRange | undefined {
        return this.term?.getSelectionPosition();
    }

    // --- Scrollback & Viewport ---

    /** Scroll by a number of lines (positive = down, negative = up into history) */
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

    // --- Link Detection ---

    /**
     * Register a link provider for detecting clickable links in terminal output.
     * Returns a disposable to unregister the provider.
     */
    registerLinkProvider(provider: BoobaLinkProvider): { dispose(): void } | undefined {
        return this.term?.registerLinkProvider(provider);
    }

    // --- Custom Event Handlers ---

    /** Attach a custom keyboard event handler. Return true to prevent default handling. */
    attachCustomKeyEventHandler(handler: (event: KeyboardEvent) => boolean): void {
        this.term?.attachCustomKeyEventHandler(handler);
    }

    /** Attach a custom wheel event handler. Return true to prevent default scroll handling. */
    attachCustomWheelEventHandler(handler?: (event: WheelEvent) => boolean): void {
        this.term?.attachCustomWheelEventHandler(handler);
    }

    // --- Lifecycle ---

    /** Dispose the terminal and clean up all resources */
    dispose(): void {
        this.disconnect();
        if (this._resizeHandler) {
            window.removeEventListener('resize', this._resizeHandler);
            this._resizeHandler = null;
        }
        this.term?.dispose();
        this.term = null;
        this.fitAddon = null;
    }

    // --- Advanced Access ---

    /** Get the underlying ghostty-web Terminal instance for advanced use cases */
    get terminal(): any {
        return this.term;
    }

    /** Get the current number of columns */
    get cols(): number {
        return this.term?.cols ?? 0;
    }

    /** Get the current number of rows */
    get rows(): number {
        return this.term?.rows ?? 0;
    }

    private _updateStatus(state: string, message: string) {
        if (this.onStatusChange) {
            this.onStatusChange(state, message);
        }
    }
}

// Re-export adapter types for convenience
export { BoobaAdapter, BoobaWasmAdapter, BoobaConnectionState };
export { BoobaProtocolAdapter } from './websocket_adapter.js';
export { BoobaAutoAdapter } from './auto_adapter.js';
export { BoobaWebTransportAdapter } from './webtransport_adapter.js';
export { OSC52Scanner } from './clipboard.js';
export type { BoobaTheme, BoobaBufferRange, BoobaKeyEvent, BoobaRenderEvent, BoobaLinkProvider, BoobaLink } from './types.js';
