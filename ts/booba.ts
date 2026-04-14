// @ts-ignore - Import will resolve at runtime in browser
import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';
import { BoobaAdapter, BoobaConnectionState, BoobaWebSocketAdapter, BoobaWasmAdapter } from './adapter.js';
import type { BoobaTheme, BoobaLinkProvider } from './types.js';

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
    container: HTMLElement | null;
    options: BoobaTerminalOptions;
    term: any; // Using any for now as we don't have full types for Terminal
    adapter: BoobaAdapter | null;
    onStatusChange: ((state: string, message: string) => void) | null;
    fitAddon: FitAddon | null;
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
        this.term = null;
        this.adapter = null;
        this.onStatusChange = null;
        this.fitAddon = null;
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
        window.addEventListener('resize', () => {
            this.fitAddon?.fit();
        });

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
        this.adapter = new BoobaWebSocketAdapter(url);
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
                // Write data from BubbleTea to terminal
                this.term.write(data);
            },
            (state: BoobaConnectionState, message: string) => {
                // Update connection status
                this._updateStatus(state, message);

                // Send initial size when connected
                if (state === 'connected' && this.term) {
                    this.adapter?.boobaResize(this.term.cols, this.term.rows);
                }

                // Show disconnect message in terminal
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

    getSelection(): string {
        return this.term?.getSelection() ?? '';
    }

    hasSelection(): boolean {
        return this.term?.hasSelection() ?? false;
    }

    clearSelection(): void {
        this.term?.clearSelection();
    }

    copySelection(): boolean {
        return this.term?.copySelection() ?? false;
    }

    selectAll(): void {
        this.term?.selectAll();
    }

    select(column: number, row: number, length: number): void {
        this.term?.select(column, row, length);
    }

    selectLines(start: number, end: number): void {
        this.term?.selectLines(start, end);
    }

    getSelectionPosition(): { start: { x: number; y: number }; end: { x: number; y: number } } | undefined {
        return this.term?.getSelectionPosition();
    }

    // --- Scrollback & Viewport ---

    scrollLines(amount: number): void {
        this.term?.scrollLines(amount);
    }

    scrollPages(amount: number): void {
        this.term?.scrollPages(amount);
    }

    scrollToTop(): void {
        this.term?.scrollToTop();
    }

    scrollToBottom(): void {
        this.term?.scrollToBottom();
    }

    scrollToLine(line: number): void {
        this.term?.scrollToLine(line);
    }

    getViewportY(): number {
        return this.term?.getViewportY() ?? 0;
    }

    // --- Terminal Control ---

    paste(data: string): void {
        this.term?.paste(data);
    }

    input(data: string): void {
        this.term?.input(data, true);
    }

    focus(): void {
        this.term?.focus();
    }

    blur(): void {
        this.term?.blur();
    }

    clear(): void {
        this.term?.clear();
    }

    reset(): void {
        this.term?.reset();
    }

    write(data: string | Uint8Array, callback?: () => void): void {
        this.term?.write(data, callback);
    }

    writeln(data: string | Uint8Array, callback?: () => void): void {
        this.term?.writeln(data, callback);
    }

    // --- Terminal Mode Queries ---

    hasMouseTracking(): boolean {
        return this.term?.hasMouseTracking() ?? false;
    }

    hasBracketedPaste(): boolean {
        return this.term?.hasBracketedPaste() ?? false;
    }

    hasFocusEvents(): boolean {
        return this.term?.hasFocusEvents() ?? false;
    }

    getMode(mode: number, isAnsi?: boolean): boolean {
        return this.term?.getMode(mode, isAnsi) ?? false;
    }

    // --- Link Detection ---

    registerLinkProvider(provider: BoobaLinkProvider): { dispose(): void } | undefined {
        return this.term?.registerLinkProvider(provider);
    }

    // --- Custom Event Handlers ---

    attachCustomKeyEventHandler(handler: (event: KeyboardEvent) => boolean): void {
        this.term?.attachCustomKeyEventHandler(handler);
    }

    attachCustomWheelEventHandler(handler?: (event: WheelEvent) => boolean): void {
        this.term?.attachCustomWheelEventHandler(handler);
    }

    // --- Lifecycle ---

    dispose(): void {
        this.disconnect();
        this.term?.dispose();
        this.term = null;
        this.fitAddon = null;
    }

    // --- Advanced Access ---

    get terminal(): any {
        return this.term;
    }

    get cols(): number {
        return this.term?.cols ?? 0;
    }

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
export { BoobaAdapter, BoobaWebSocketAdapter, BoobaWasmAdapter, BoobaConnectionState };
export type { BoobaTheme, BoobaBufferRange, BoobaKeyEvent, BoobaRenderEvent, BoobaLinkProvider, BoobaLink } from './types.js';
