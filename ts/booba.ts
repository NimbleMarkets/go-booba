import { init, Terminal, FitAddon } from 'ghostty-web';
import { BoobaAdapter, BoobaConnectionState, BoobaWasmAdapter } from './adapter.js';
import { BoobaProtocolAdapter, type WebSocketAdapterCallbacks } from './websocket_adapter.js';
import { BoobaAutoAdapter } from './auto_adapter.js';
import { OSC52Scanner } from './clipboard.js';
import { encodeKey as encodeLegacyKey } from './key_encoder.js';
import { ActionController, type ActionGesture, type ActionKeyEncoder } from './actions.js';
import type { ProgramStatusReport, ProgramStatusSource } from './program_status.js';
import { ActionStatusBridge } from './action_status.js';
import { ProgramStatusStore } from './program_status_store.js';
import { TerminalSequenceObserver } from './terminal_sequences.js';
import type { BoobaTheme, BoobaBufferRange, BoobaLinkProvider } from './types.js';
import { applyThemeProperties } from './theme_properties.js';
import { TouchInput } from './touch_input.js';

import type { RendererBackend } from 'ghostty-web';

export interface BoobaTerminalOptions {
    fontSize?: number;
    fontFamily?: string;
    cols?: number;
    rows?: number;
    cursorBlink?: boolean;
    cursorStyle?: 'block' | 'underline' | 'bar';
    scrollback?: number;
    allowOSC52?: boolean;
    /** Experimental, disabled by default. Interpret the private booba-actions msg convention; its schema may change. */
    allowOSC7501Actions?: boolean;
    allowTransparency?: boolean;
    convertEol?: boolean;
    disableStdin?: boolean;
    smoothScrollDuration?: number;
    theme?: BoobaTheme;
    renderer?: RendererBackend;
}

export class BoobaTerminal {
    container: HTMLElement | null = null;
    options: BoobaTerminalOptions;
    term: Terminal | null = null;
    adapter: BoobaAdapter | null = null;
    fitAddon: FitAddon | null = null;
    private _resizeHandler: (() => void) | null = null;
    private _dprCleanup: (() => void) | null = null;
    private touchInput: TouchInput | null = null;
    private osc52Scanner: OSC52Scanner;
    /** Direct publication API until the action OSC envelope is defined. */
    readonly actions: ActionController;
    private actionKeyEncoder: ActionKeyEncoder | null = null;
    private actionReadOnly = false;
    private actionStatus: ActionStatusBridge;
    /** Program Status Protocol (OSC 7501) records, with the protocol's lifetime rules applied. */
    readonly programStatus: ProgramStatusStore;
    private sequenceObserver: TerminalSequenceObserver;
    private terminalWriteDepth = 0;
    private lifecycleBoundaries: { reason: 'reset' | 'screen' | 'prompt'; end: number }[] = [];
    onActionError: ((error: Error) => void) | null = null;

    /** Install a mode-aware encoder; no gestures are enabled without one. */
    setActionKeyEncoder(encoder: ActionKeyEncoder | null): void {
        this.actionKeyEncoder = encoder;
        this.actions.clear();
    }

    /** Encode a legacy VT/xterm gesture using current cursor, backarrow and newline modes.
     * Opt-in only: ghostty-web cannot report Kitty/modifyOtherKeys state. Hosts using
     * those protocols must supply their own ActionKeyEncoder instead.
     */
    encodeKey(gesture: ActionGesture): string | null {
        if (!this.term) return null;
        return encodeLegacyKey(gesture, {
            applicationCursor: this.getMode(1),
            backarrow: this.getMode(67),
            newline: this.getMode(20, true),
        });
    }

    private programStatusSubscription: { dispose(): void } | null = null;
    private programStatusCallback: ((report: ProgramStatusReport) => void) | null = null;

    /** Available when the loaded ghostty-web runtime provides OSC 7501 events. */
    get supportsProgramStatus(): boolean {
        return typeof (this.term as ProgramStatusSource | null)?.onProgramStatus === 'function';
    }

    get onProgramStatus(): ((report: ProgramStatusReport) => void) | null {
        return this.programStatusCallback;
    }

    set onProgramStatus(callback: ((report: ProgramStatusReport) => void) | null) {
        this.programStatusCallback = callback;
        this.syncProgramStatus();
    }

    private get consumesProgramStatus(): boolean {
        return !!(this.options.allowOSC7501Actions || this.programStatusCallback || this.programStatus.listenerCount > 0);
    }

    private syncProgramStatus(): void {
        this.programStatusSubscription?.dispose();
        this.programStatusSubscription = null;
        const source = this.term as ProgramStatusSource | null;
        if (this.consumesProgramStatus && source?.onProgramStatus) {
            this.programStatusSubscription = source.onProgramStatus(report => {
                const display = this.options.allowOSC7501Actions ? this.actionStatus.receive(report) : report;
                this.programStatus.apply(display);
                this.programStatusCallback?.(display);
            });
        }
    }

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
            fontSize: 15,
            cols: 80,
            rows: 24,
            allowOSC52: false,
            convertEol: true,
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
            },
            ...options
        };
        this.osc52Scanner = new OSC52Scanner(this.options.allowOSC52 ?? false);
        this.actions = new ActionController({
            canSendKey: gesture => !!(this.term && this.adapter && !this.actionReadOnly &&
                !this.options.disableStdin && !this.term.options.disableStdin &&
                this.actionKeyEncoder?.canEncode(gesture)),
            sendKey: gesture => {
                const encoded = this.actionKeyEncoder?.encode(gesture);
                if (!encoded || !this.term || !this.adapter || this.actionReadOnly ||
                    this.options.disableStdin || this.term.options.disableStdin) return false;
                this.term.input(encoded, true);
                return true;
            },
        });
        this.actionStatus = new ActionStatusBridge(this.actions, error => this.onActionError?.(error));
        this.programStatus = new ProgramStatusStore(Date.now, () => this.syncProgramStatus());
        this.sequenceObserver = new TerminalSequenceObserver({
            osc: () => {},
            lifecycle: (reason, end) => this.lifecycleBoundaries.push({ reason, end }),
        });
    }

    // Split only at lifecycle boundaries so native onProgramStatus events and
    // clears retain wire order even when multiple sequences share one write.
    private writeLiveOutput(data: string | Uint8Array): void {
        if (!this.consumesProgramStatus) { this.writeTerminal(data); return; }
        this.lifecycleBoundaries = [];
        this.sequenceObserver.scan(data);
        let start = 0;
        for (const { reason, end } of this.lifecycleBoundaries) {
            this.writeTerminal(data.slice(start, end));
            this.actionStatus.clear();
            // Records belong to the terminal, not the screen: a full reset drops
            // them all, a shell prompt ends in-flight work, a screen switch is nothing.
            if (reason === 'reset') this.programStatus.clear();
            else if (reason === 'prompt') this.programStatus.prompt();
            start = end;
        }
        if (start < data.length) this.writeTerminal(data.slice(start));
    }

    /**
     * Write to the terminal. Replies the terminal generates while parsing
     * (query responses) arrive on onData synchronously inside this call, which
     * is how forwardInput tells them apart from user input.
     */
    private writeTerminal(data: string | Uint8Array, callback?: () => void): void {
        this.terminalWriteDepth++;
        try { this.term?.write(data, callback); } finally { this.terminalWriteDepth--; }
    }

    /**
     * Send user input to the program. The user has now seen any finished or
     * failed status, which the protocol retires on the next interaction.
     * Terminal-generated replies arrive here synchronously inside a write and
     * are not user input, so they never dismiss.
     * @internal exposed for tests; terminal input normally arrives here via onData.
     */
    forwardInput(data: string | Uint8Array): void {
        if (this.terminalWriteDepth === 0 && this.programStatus.records.some(r => r.state === 'done' || r.state === 'error')) {
            this.programStatus.dismissSettled();
        }
        this.adapter?.boobaWrite(data);
    }

    async init() {
        if (!this.container) {
            throw new Error('BoobaTerminal.init: container element is null');
        }
        await init();
        const term = new Terminal(this.options);
        this.term = term;
        this.syncProgramStatus();

        this.fitAddon = new FitAddon();
        term.loadAddon(this.fitAddon);

        // CRITICAL: term.open() is async and must be awaited. Without the await,
        // callers that call init() then immediately connectWasm() will race the
        // renderer's async initialization. If WebGPU adapter is slow (cold cache),
        // term.write() will call assertOpen() before isOpen flips true, causing
        // "Terminal must be opened" error. See ghostty-web.js:8055-8067 for details.
        await term.open(this.container);
        applyThemeProperties(this.container, this.options.theme);
        this.fitAddon.fit();
        this.fitAddon.observeResize();

        // Handle window resize as fallback
        this._resizeHandler = () => { this.fitAddon?.fit(); };
        window.addEventListener('resize', this._resizeHandler);

        // Watch for devicePixelRatio changes (browser zoom, display switch)
        this._dprCleanup = this._watchDevicePixelRatio();

        // Listen for resize events from the terminal (triggered by fit addon)
        term.onResize((size) => {
            const px = this._pixelDims(size.cols, size.rows);
            this.adapter?.boobaResize(size.cols, size.rows, px.widthPx, px.heightPx);
        });

        console.log('Terminal opened. Size:', term.cols, 'x', term.rows);

        // Send user input through adapter
        term.onData((data) => this.forwardInput(data));
        this.touchInput = new TouchInput(this.container, {
            hasMouseTracking: () => this.hasMouseTracking() && !this.options.disableStdin && !term.options.disableStdin && !this.actionReadOnly,
            getCellSize: () => term.renderer?.getMetrics(),
            getSize: () => ({ cols: term.cols, rows: term.rows }),
            sendInput: data => this.forwardInput(data),
        });

        term.onBell(() => {
            this.onBell?.();
        });

        term.onSelectionChange(() => {
            this.onSelectionChange?.();
        });

        term.onKey((event) => {
            this.onKey?.(event);
        });

        term.onTitleChange((title) => {
            this.onTitleChange?.(title);
        });

        term.onScroll((viewportY) => {
            this.onScroll?.(viewportY);
        });

        term.onRender((event) => {
            this.onRender?.(event);
        });

        term.onCursorMove(() => {
            this.onCursorMove?.();
        });
    }

    /**
     * Connect to a BubbleTea backend via WebSocket
     * @param url WebSocket URL (e.g., 'ws://localhost:8080/ws')
     */
    connectWebSocket(url: string) {
        this.disconnect();
        this.actionReadOnly = true;
        const callbacks: WebSocketAdapterCallbacks = {
            onTitle: (title) => { if (this.adapter === adapter) this.onTitleChange?.(title); },
            onOptions: (opts) => {
                if (this.adapter !== adapter) return;
                this.actionReadOnly = opts.readOnly !== false;
                this.actions.setReadOnly(this.actionReadOnly);
            },
            onClose: (reason) => {
                if (this.adapter !== adapter) return;
                this.term?.write(`\r\n${reason}\r\n`);
            },
        };
        const adapter = new BoobaProtocolAdapter(url, callbacks);
        this.adapter = adapter;
        this._setupAdapter();
    }

    /**
     * Connect with auto-detection: tries WebTransport first, falls back to WebSocket.
     * @param wsUrl WebSocket URL (e.g., 'ws://localhost:8080/ws')
     * @param wtUrl WebTransport URL (e.g., 'https://localhost:8080/wt'), or null to skip
     * @param certHashUrl URL to fetch cert hash (e.g., 'https://localhost:8080/cert-hash'), or null
     */
    connectAuto(wsUrl: string, wtUrl: string | null = null, certHashUrl: string | null = null) {
        this.disconnect();
        this.actionReadOnly = true;
        const callbacks = {
            onTitle: (title: string) => { if (this.adapter === adapter) this.onTitleChange?.(title); },
            onOptions: (opts: { readOnly: boolean }) => {
                if (this.adapter !== adapter) return;
                this.actionReadOnly = opts.readOnly !== false;
                this.actions.setReadOnly(this.actionReadOnly);
            },
            onClose: (reason: string) => { if (this.adapter !== adapter) return; this.term?.write(`\r\n${reason}\r\n`); },
        };
        const adapter = new BoobaAutoAdapter(wsUrl, wtUrl, certHashUrl, callbacks);
        this.adapter = adapter;
        this._setupAdapter();
    }

    /**
     * Connect to a BubbleTea program running in WASM
     * @param pollMs Polling interval in milliseconds (default: 16ms / ~60fps)
     */
    connectWasm(pollMs: number = 16) {
        this.disconnect();
        this.actionReadOnly = false;
        this.adapter = new BoobaWasmAdapter(pollMs);
        this._setupAdapter();
    }

    /**
     * Use a custom adapter implementation
     * @param adapter Custom BubbleTeaAdapter
     */
    connectAdapter(adapter: BoobaAdapter) {
        this.disconnect();
        this.actionReadOnly = false;
        this.adapter = adapter;
        this._setupAdapter();
    }

    private _setupAdapter() {
        if (!this.adapter) return;
        const adapter = this.adapter;
        this.actions.setConnected(false);
        this.actions.setReadOnly(this.actionReadOnly);
        adapter.connect(
            (data: string | Uint8Array) => {
                if (this.adapter !== adapter) return;
                this.osc52Scanner.scan(data);
                this.writeLiveOutput(data);
            },
            (state: BoobaConnectionState, message: string) => {
                if (this.adapter !== adapter) return;
                this.sequenceObserver.reset();
                this.actionStatus.clear();
                if (state === 'connected') this.programStatus.clear();
                else if (state === 'disconnected') this.programStatus.exit();
                this.actions.setConnected(state === 'connected');
                this._updateStatus(state, message);
                const term = this.term;
                if (state === 'connected' && term) {
                    const px = this._pixelDims(term.cols, term.rows);
                    this.adapter?.boobaResize(term.cols, term.rows, px.widthPx, px.heightPx);
                }
                if (state === 'disconnected') {
                    term?.write('\r\nConnection closed.\r\n');
                }
            }
        );
    }

    disconnect() {
        const adapter = this.adapter;
        this.adapter = null;
        this.sequenceObserver.reset();
        this.actionStatus.clear();
        this.programStatus.exit();
        this.actions.setConnected(false);
        adapter?.disconnect();
        if (adapter) this._updateStatus('disconnected', 'Disconnected');
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
        this.sequenceObserver.reset();
        this.actionStatus.clear();
        this.programStatus.clear();
        this.term?.reset();
    }

    /** Write data to the terminal display */
    write(data: string | Uint8Array, callback?: () => void): void {
        this.writeTerminal(data, callback);
    }

    /** Write data with a trailing newline */
    writeln(data: string | Uint8Array, callback?: () => void): void {
        this.terminalWriteDepth++;
        try { this.term?.writeln(data, callback); } finally { this.terminalWriteDepth--; }
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
     */
    registerLinkProvider(provider: BoobaLinkProvider): void {
        this.term?.registerLinkProvider(provider);
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
        this.touchInput?.dispose();
        this.touchInput = null;
        this.disconnect();
        if (this._resizeHandler) {
            window.removeEventListener('resize', this._resizeHandler);
            this._resizeHandler = null;
        }
        this._dprCleanup?.();
        this._dprCleanup = null;
        this.programStatusSubscription?.dispose();
        this.programStatusSubscription = null;
        this.programStatus.clear();
        this.term?.dispose();
        this.term = null;
        this.fitAddon = null;
    }

    // --- Advanced Access ---

    /** Get the underlying ghostty-web Terminal instance for advanced use cases */
    get terminal(): Terminal | null {
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

    /**
     * Compute the canvas pixel dimensions corresponding to a (cols, rows) grid.
     * Returns zeros when the renderer hasn't measured yet so the server-side
     * resize falls back to character-only dimensions.
     */
    private _pixelDims(cols: number, rows: number): { widthPx: number; heightPx: number } {
        const renderer = this.term?.renderer;
        if (!renderer) return { widthPx: 0, heightPx: 0 };
        const m = renderer.getMetrics();
        return {
            widthPx: Math.max(0, Math.round(m.width * cols)),
            heightPx: Math.max(0, Math.round(m.height * rows)),
        };
    }

    /**
     * Watch for devicePixelRatio changes (browser zoom, moving between displays).
     * Patches the renderer's cached DPR and forces a canvas re-scale.
     */
    private _watchDevicePixelRatio(): () => void {
        let currentDpr = window.devicePixelRatio;
        let mql: MediaQueryList | null = null;

        const onChange = () => {
            const newDpr = window.devicePixelRatio;
            if (newDpr !== currentDpr) {
                currentDpr = newDpr;
                const term = this.term;
                const renderer = term?.renderer;
                if (term && renderer) {
                    (renderer as any).devicePixelRatio = newDpr;
                    renderer.resize(term.cols, term.rows);
                }
                this.fitAddon?.fit();
            }
            // Re-register: matchMedia for DPR is a one-shot per value
            listen();
        };

        const listen = () => {
            mql?.removeEventListener('change', onChange);
            mql = window.matchMedia(`(resolution: ${currentDpr}dppx)`);
            mql.addEventListener('change', onChange);
        };

        listen();

        return () => {
            mql?.removeEventListener('change', onChange);
        };
    }
}

// Re-export adapter types for convenience
export { BoobaAdapter, BoobaWasmAdapter, BoobaConnectionState };
export { BoobaProtocolAdapter } from './websocket_adapter.js';
export { BoobaAutoAdapter } from './auto_adapter.js';
export { BoobaWebTransportAdapter } from './webtransport_adapter.js';
export { OSC52Scanner } from './clipboard.js';
export { resolveBoobaURLs, type BoobaURLs } from './urls.js';
export type { BoobaTheme, BoobaBufferRange, BoobaKeyEvent, BoobaRenderEvent, BoobaLinkProvider, BoobaLink } from './types.js';

// HUD utilities (optional, treeshakeable) — re-exported from ghostty-web
export { installRendererHud, parseRendererFromURL, type RendererHudOptions, type RendererBackend } from 'ghostty-web';

// Deprecated aliases for source compatibility
import type { RendererHudOptions, RendererBackend as _RendererBackend } from 'ghostty-web';

/**
 * @deprecated Renamed to RendererHudOptions when the HUD moved to ghostty-web.
 * This alias will be removed in a future major version.
 */
export type BoobaRendererHudOptions = RendererHudOptions;

/**
 * @deprecated Renamed to RendererBackend when the HUD moved to ghostty-web.
 * This alias also expanded to include 'webgl', so behavior is wider than the
 * old name implied.
 */
export type BoobaRenderer = _RendererBackend;

// Transport-independent terminal actions and optional host UI.
export { ActionController, ACTION_LIMITS, validateActionSnapshot, decodeActionSnapshot } from './actions.js';
export type { ActionSnapshot, TerminalAction, ActionGesture, ActionControllerOptions, ActionKeyEncoder } from './actions.js';
export { installActionBar, ACTION_BAR_CSS } from './action_bar.js';
export type { ActionBarOptions } from './action_bar.js';
export { ProgramStatusStore, MAX_PROGRAM_STATUS_RECORDS } from './program_status_store.js';
export type { ProgramStatusRecord, ProgramStatusState } from './program_status_store.js';
export { installStatusBar, STATUS_BAR_CSS } from './status_bar.js';
export type { StatusBarOptions } from './status_bar.js';
export type { ProgramStatusReport } from './program_status.js';
export { decodeActionStatus, actionStatusID, ACTION_MESSAGE_MARKER, ACTION_STATUS_NAMESPACE } from './action_status.js';

export { encodeKey, type KeyEncodingModes } from './key_encoder.js';
