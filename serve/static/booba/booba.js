import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';
import { BoobaWasmAdapter } from './adapter.js';
import { BoobaProtocolAdapter } from './websocket_adapter.js';
import { BoobaAutoAdapter } from './auto_adapter.js';
import { OSC52Scanner } from './clipboard.js';
import { ActionController } from './actions.js';
import { ActionStatusBridge } from './action_status.js';
import { ProgramStatusStore } from './program_status_store.js';
import { TerminalSequenceObserver } from './terminal_sequences.js';
import { applyThemeProperties } from './theme_properties.js';
export class BoobaTerminal {
    /** Install a mode-aware encoder; no gestures are enabled without one. */
    setActionKeyEncoder(encoder) {
        this.actionKeyEncoder = encoder;
        this.actions.clear();
    }
    /** Available when the loaded ghostty-web runtime provides OSC 7501 events. */
    get supportsProgramStatus() {
        return typeof this.term?.onProgramStatus === 'function';
    }
    get onProgramStatus() {
        return this.programStatusCallback;
    }
    set onProgramStatus(callback) {
        this.programStatusCallback = callback;
        this.syncProgramStatus();
    }
    get consumesProgramStatus() {
        return !!(this.options.allowOSC7501Actions || this.programStatusCallback || this.programStatus.listenerCount > 0);
    }
    syncProgramStatus() {
        this.programStatusSubscription?.dispose();
        this.programStatusSubscription = null;
        const source = this.term;
        if (this.consumesProgramStatus && source?.onProgramStatus) {
            this.programStatusSubscription = source.onProgramStatus(report => {
                const display = this.options.allowOSC7501Actions ? this.actionStatus.receive(report) : report;
                this.programStatus.apply(display);
                this.programStatusCallback?.(display);
            });
        }
    }
    constructor(containerId, options = {}) {
        this.container = null;
        this.term = null;
        this.adapter = null;
        this.fitAddon = null;
        this._resizeHandler = null;
        this._dprCleanup = null;
        this.actionKeyEncoder = null;
        this.actionReadOnly = false;
        this.terminalWriteDepth = 0;
        this.lifecycleBoundaries = [];
        this.onActionError = null;
        this.programStatusSubscription = null;
        this.programStatusCallback = null;
        // --- Event Callbacks ---
        this.onStatusChange = null;
        this.onBell = null;
        this.onSelectionChange = null;
        this.onKey = null;
        this.onTitleChange = null;
        this.onScroll = null;
        this.onRender = null;
        this.onCursorMove = null;
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
                    this.options.disableStdin || this.term.options.disableStdin)
                    return false;
                this.term.input(encoded, true);
                return true;
            },
        });
        this.actionStatus = new ActionStatusBridge(this.actions, error => this.onActionError?.(error));
        this.programStatus = new ProgramStatusStore(Date.now, () => this.syncProgramStatus());
        this.sequenceObserver = new TerminalSequenceObserver({
            osc: () => { },
            lifecycle: (reason, end) => this.lifecycleBoundaries.push({ reason, end }),
        });
    }
    // Split only at lifecycle boundaries so native onProgramStatus events and
    // clears retain wire order even when multiple sequences share one write.
    writeLiveOutput(data) {
        if (!this.consumesProgramStatus) {
            this.writeTerminal(data);
            return;
        }
        this.lifecycleBoundaries = [];
        this.sequenceObserver.scan(data);
        let start = 0;
        for (const { reason, end } of this.lifecycleBoundaries) {
            this.writeTerminal(data.slice(start, end));
            this.actionStatus.clear();
            // Records belong to the terminal, not the screen: a full reset drops
            // them all, a shell prompt ends in-flight work, a screen switch is nothing.
            if (reason === 'reset')
                this.programStatus.clear();
            else if (reason === 'prompt')
                this.programStatus.prompt();
            start = end;
        }
        if (start < data.length)
            this.writeTerminal(data.slice(start));
    }
    /**
     * Write to the terminal. Replies the terminal generates while parsing
     * (query responses) arrive on onData synchronously inside this call, which
     * is how forwardInput tells them apart from user input.
     */
    writeTerminal(data, callback) {
        this.terminalWriteDepth++;
        try {
            this.term?.write(data, callback);
        }
        finally {
            this.terminalWriteDepth--;
        }
    }
    /**
     * Send user input to the program. The user has now seen any finished or
     * failed status, which the protocol retires on the next interaction.
     * Terminal-generated replies arrive here synchronously inside a write and
     * are not user input, so they never dismiss.
     * @internal exposed for tests; terminal input normally arrives here via onData.
     */
    forwardInput(data) {
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
    connectWebSocket(url) {
        this.disconnect();
        this.actionReadOnly = true;
        const callbacks = {
            onTitle: (title) => { if (this.adapter === adapter)
                this.onTitleChange?.(title); },
            onOptions: (opts) => {
                if (this.adapter !== adapter)
                    return;
                this.actionReadOnly = opts.readOnly !== false;
                this.actions.setReadOnly(this.actionReadOnly);
            },
            onClose: (reason) => {
                if (this.adapter !== adapter)
                    return;
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
    connectAuto(wsUrl, wtUrl = null, certHashUrl = null) {
        this.disconnect();
        this.actionReadOnly = true;
        const callbacks = {
            onTitle: (title) => { if (this.adapter === adapter)
                this.onTitleChange?.(title); },
            onOptions: (opts) => {
                if (this.adapter !== adapter)
                    return;
                this.actionReadOnly = opts.readOnly !== false;
                this.actions.setReadOnly(this.actionReadOnly);
            },
            onClose: (reason) => { if (this.adapter !== adapter)
                return; this.term?.write(`\r\n${reason}\r\n`); },
        };
        const adapter = new BoobaAutoAdapter(wsUrl, wtUrl, certHashUrl, callbacks);
        this.adapter = adapter;
        this._setupAdapter();
    }
    /**
     * Connect to a BubbleTea program running in WASM
     * @param pollMs Polling interval in milliseconds (default: 16ms / ~60fps)
     */
    connectWasm(pollMs = 16) {
        this.disconnect();
        this.actionReadOnly = false;
        this.adapter = new BoobaWasmAdapter(pollMs);
        this._setupAdapter();
    }
    /**
     * Use a custom adapter implementation
     * @param adapter Custom BubbleTeaAdapter
     */
    connectAdapter(adapter) {
        this.disconnect();
        this.actionReadOnly = false;
        this.adapter = adapter;
        this._setupAdapter();
    }
    _setupAdapter() {
        if (!this.adapter)
            return;
        const adapter = this.adapter;
        this.actions.setConnected(false);
        this.actions.setReadOnly(this.actionReadOnly);
        adapter.connect((data) => {
            if (this.adapter !== adapter)
                return;
            if (data instanceof Uint8Array) {
                this.osc52Scanner.scan(data);
            }
            this.writeLiveOutput(data);
        }, (state, message) => {
            if (this.adapter !== adapter)
                return;
            this.sequenceObserver.reset();
            this.actionStatus.clear();
            if (state === 'connected')
                this.programStatus.clear();
            else if (state === 'disconnected')
                this.programStatus.exit();
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
        });
    }
    disconnect() {
        const adapter = this.adapter;
        this.adapter = null;
        this.sequenceObserver.reset();
        this.actionStatus.clear();
        this.programStatus.exit();
        this.actions.setConnected(false);
        adapter?.disconnect();
        if (adapter)
            this._updateStatus('disconnected', 'Disconnected');
    }
    // --- Selection & Clipboard ---
    /** Get the currently selected text */
    getSelection() {
        return this.term?.getSelection() ?? '';
    }
    /** Check if there's an active selection */
    hasSelection() {
        return this.term?.hasSelection() ?? false;
    }
    /** Clear the current selection */
    clearSelection() {
        this.term?.clearSelection();
    }
    /** Copy the current selection to clipboard. Returns true if text was copied. */
    copySelection() {
        return this.term?.copySelection() ?? false;
    }
    /** Select all text in the terminal */
    selectAll() {
        this.term?.selectAll();
    }
    /** Select text at a specific position */
    select(column, row, length) {
        this.term?.select(column, row, length);
    }
    /** Select entire lines from start to end (inclusive) */
    selectLines(start, end) {
        this.term?.selectLines(start, end);
    }
    /** Get the selection position as a buffer range, or undefined if no selection */
    getSelectionPosition() {
        return this.term?.getSelectionPosition();
    }
    // --- Scrollback & Viewport ---
    /** Scroll by a number of lines (positive = down, negative = up into history) */
    scrollLines(amount) {
        this.term?.scrollLines(amount);
    }
    /** Scroll by a number of pages */
    scrollPages(amount) {
        this.term?.scrollPages(amount);
    }
    /** Scroll to the top of the scrollback buffer */
    scrollToTop() {
        this.term?.scrollToTop();
    }
    /** Scroll to the bottom (current output) */
    scrollToBottom() {
        this.term?.scrollToBottom();
    }
    /** Scroll to a specific line in the buffer */
    scrollToLine(line) {
        this.term?.scrollToLine(line);
    }
    /** Get the current viewport Y position (lines scrolled back from bottom) */
    getViewportY() {
        return this.term?.getViewportY() ?? 0;
    }
    // --- Terminal Control ---
    /** Paste text into the terminal (uses bracketed paste if the program supports it) */
    paste(data) {
        this.term?.paste(data);
    }
    /** Input data as if typed by the user */
    input(data) {
        this.term?.input(data, true);
    }
    /** Focus the terminal */
    focus() {
        this.term?.focus();
    }
    /** Remove focus from the terminal */
    blur() {
        this.term?.blur();
    }
    /** Clear the terminal screen */
    clear() {
        this.term?.clear();
    }
    /** Reset the terminal state */
    reset() {
        this.sequenceObserver.reset();
        this.actionStatus.clear();
        this.programStatus.clear();
        this.term?.reset();
    }
    /** Write data to the terminal display */
    write(data, callback) {
        this.writeTerminal(data, callback);
    }
    /** Write data with a trailing newline */
    writeln(data, callback) {
        this.terminalWriteDepth++;
        try {
            this.term?.writeln(data, callback);
        }
        finally {
            this.terminalWriteDepth--;
        }
    }
    // --- Terminal Mode Queries ---
    /** Check if the program has enabled mouse tracking */
    hasMouseTracking() {
        return this.term?.hasMouseTracking() ?? false;
    }
    /** Check if the program has enabled bracketed paste mode */
    hasBracketedPaste() {
        return this.term?.hasBracketedPaste() ?? false;
    }
    /** Check if the program has enabled focus event reporting */
    hasFocusEvents() {
        return this.term?.hasFocusEvents() ?? false;
    }
    /** Query an arbitrary terminal mode by number */
    getMode(mode, isAnsi) {
        return this.term?.getMode(mode, isAnsi) ?? false;
    }
    // --- Link Detection ---
    /**
     * Register a link provider for detecting clickable links in terminal output.
     */
    registerLinkProvider(provider) {
        this.term?.registerLinkProvider(provider);
    }
    // --- Custom Event Handlers ---
    /** Attach a custom keyboard event handler. Return true to prevent default handling. */
    attachCustomKeyEventHandler(handler) {
        this.term?.attachCustomKeyEventHandler(handler);
    }
    /** Attach a custom wheel event handler. Return true to prevent default scroll handling. */
    attachCustomWheelEventHandler(handler) {
        this.term?.attachCustomWheelEventHandler(handler);
    }
    // --- Lifecycle ---
    /** Dispose the terminal and clean up all resources */
    dispose() {
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
    get terminal() {
        return this.term;
    }
    /** Get the current number of columns */
    get cols() {
        return this.term?.cols ?? 0;
    }
    /** Get the current number of rows */
    get rows() {
        return this.term?.rows ?? 0;
    }
    _updateStatus(state, message) {
        if (this.onStatusChange) {
            this.onStatusChange(state, message);
        }
    }
    /**
     * Compute the canvas pixel dimensions corresponding to a (cols, rows) grid.
     * Returns zeros when the renderer hasn't measured yet so the server-side
     * resize falls back to character-only dimensions.
     */
    _pixelDims(cols, rows) {
        const renderer = this.term?.renderer;
        if (!renderer)
            return { widthPx: 0, heightPx: 0 };
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
    _watchDevicePixelRatio() {
        let currentDpr = window.devicePixelRatio;
        let mql = null;
        const onChange = () => {
            const newDpr = window.devicePixelRatio;
            if (newDpr !== currentDpr) {
                currentDpr = newDpr;
                const term = this.term;
                const renderer = term?.renderer;
                if (term && renderer) {
                    renderer.devicePixelRatio = newDpr;
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
export { BoobaWasmAdapter };
export { BoobaProtocolAdapter } from './websocket_adapter.js';
export { BoobaAutoAdapter } from './auto_adapter.js';
export { BoobaWebTransportAdapter } from './webtransport_adapter.js';
export { OSC52Scanner } from './clipboard.js';
export { resolveBoobaURLs } from './urls.js';
// HUD utilities (optional, treeshakeable) — re-exported from ghostty-web
export { installRendererHud, parseRendererFromURL } from '../ghostty-web/ghostty-web.js';
// Transport-independent terminal actions and optional host UI.
export { ActionController, ACTION_LIMITS, validateActionSnapshot, decodeActionSnapshot } from './actions.js';
export { installActionBar, ACTION_BAR_CSS } from './action_bar.js';
export { ProgramStatusStore, MAX_PROGRAM_STATUS_RECORDS } from './program_status_store.js';
export { installStatusBar, STATUS_BAR_CSS } from './status_bar.js';
export { decodeActionStatus, actionStatusID, ACTION_MESSAGE_MARKER, ACTION_STATUS_NAMESPACE } from './action_status.js';
