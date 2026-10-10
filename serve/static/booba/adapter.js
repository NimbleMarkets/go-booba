/**
 * Booba's BubbleTea Communication Adapter
 *
 * Provides an abstraction layer for communicating with BubbleTea programs.
 * Supports both WebSocket (backend-server) and WASM (pure embedding) modes.
 */
/**
 * WASM-based adapter for pure embedding mode
 *
 * Communicates with BubbleTea via global functions the Go program registers:
 * - window.bubbletea_write(data: string): void
 * - window.bubbletea_read(): string
 * - window.bubbletea_resize(cols: number, rows: number): void
 *
 * Once the Go program returns, none of them can be called again, so booba's
 * wasm package announces the exit first: window.bubbletea_exited becomes true
 * and window.bubbletea_final holds the unread output. The adapter delivers that
 * final frame, stops polling, and reports 'disconnected' with "Program exited".
 */
export class BoobaWasmAdapter {
    constructor(pollMs = 16) {
        this.pollMs = pollMs;
        this.pollInterval = null;
        this.onDataCallback = null;
        this.onStateChangeCallback = null;
        this.exited = false;
    } // ~60fps
    boobaRead() {
        if (this.exited)
            return null;
        if (typeof window.bubbletea_read !== 'function') {
            console.warn('bubbletea_read not available');
            return null;
        }
        const data = window.bubbletea_read();
        return data || null;
    }
    boobaWrite(data) {
        if (this.exited)
            return;
        if (typeof window.bubbletea_write !== 'function') {
            console.warn('bubbletea_write not available');
            return;
        }
        const dataStr = typeof data === 'string' ? data : new TextDecoder().decode(data);
        window.bubbletea_write(dataStr);
    }
    boobaResize(cols, rows, _widthPx, _heightPx) {
        if (this.exited)
            return;
        if (typeof window.bubbletea_resize !== 'function') {
            console.warn('bubbletea_resize not available');
            return;
        }
        window.bubbletea_resize(cols, rows);
        console.log('Sent resize to WASM:', cols, rows);
    }
    /** Called when the Go program has returned: flush its last output and end the session. */
    handleExit() {
        if (this.exited)
            return;
        this.exited = true;
        if (this.pollInterval !== null) {
            clearInterval(this.pollInterval);
            this.pollInterval = null;
        }
        const final = window.bubbletea_final;
        if (typeof final === 'string' && final.length > 0) {
            this.onDataCallback?.(final);
        }
        this.onStateChangeCallback?.('disconnected', 'Program exited');
    }
    poll() {
        if (window.bubbletea_exited === true) {
            this.handleExit();
            return;
        }
        let data;
        try {
            data = this.boobaRead();
        }
        catch {
            // The bridge throws "Go program has already exited" once the runtime is gone.
            this.handleExit();
            return;
        }
        if (data && data.length > 0 && this.onDataCallback) {
            this.onDataCallback(data);
        }
    }
    connect(onData, onStateChange) {
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        this.exited = false;
        const startPolling = () => {
            this.pollInterval = window.setInterval(() => this.poll(), this.pollMs);
            onStateChange('connected', 'Connected');
            console.log('WASM adapter connected, polling at', this.pollMs, 'ms');
        };
        if (typeof window.bubbletea_read === 'function') {
            startPolling();
            return;
        }
        // Wait for WASM module to initialize
        let attempts = 0;
        const checkInterval = window.setInterval(() => {
            if (typeof window.bubbletea_read === 'function') {
                window.clearInterval(checkInterval);
                startPolling();
                return;
            }
            attempts++;
            if (attempts > 100) { // 100 * 50ms = 5 seconds timeout
                window.clearInterval(checkInterval);
                onStateChange('disconnected', 'WASM functions not available');
                console.error('WASM BubbleTea functions not found. Ensure the WASM module is loaded.');
            }
        }, 50);
    }
    disconnect() {
        if (this.pollInterval !== null) {
            clearInterval(this.pollInterval);
            this.pollInterval = null;
        }
        this.onDataCallback = null;
        this.onStateChangeCallback = null;
    }
}
