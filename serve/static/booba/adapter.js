/**
 * Booba's BubbleTea Communication Adapter
 *
 * Provides an abstraction layer for communicating with BubbleTea programs.
 * Supports both WebSocket (backend-server) and WASM (pure embedding) modes.
 */
/**
 * WASM-based adapter for pure embedding mode
 *
 * Communicates with BubbleTea via global WASM functions:
 * - window.booba_write(data: string): void
 * - window.booba_read(): string
 * - window.booba_resize(cols: number, rows: number): void
 */
export class BoobaWasmAdapter {
    constructor(pollMs = 16) {
        this.pollMs = pollMs;
        this.pollInterval = null;
        this.onDataCallback = null;
    } // ~60fps
    boobaRead() {
        if (typeof window.bubbletea_read !== 'function') {
            console.warn('bubbletea_read not available');
            return null;
        }
        const data = window.bubbletea_read();
        return data || null;
    }
    boobaWrite(data) {
        if (typeof window.bubbletea_write !== 'function') {
            console.warn('bubbletea_write not available');
            return;
        }
        const dataStr = typeof data === 'string' ? data : new TextDecoder().decode(data);
        window.bubbletea_write(dataStr);
    }
    boobaResize(cols, rows) {
        if (typeof window.bubbletea_resize !== 'function') {
            console.warn('bubbletea_resize not available');
            return;
        }
        window.bubbletea_resize(cols, rows);
        console.log('Sent resize to WASM:', cols, rows);
    }
    connect(onData, onStateChange) {
        this.onDataCallback = onData;
        // Check if WASM functions are available
        if (typeof window.bubbletea_read !== 'function') {
            onStateChange('disconnected', 'WASM functions not available');
            console.error('WASM BubbleTea functions not found. Ensure the WASM module is loaded.');
            return;
        }
        // Start polling for data from WASM
        this.pollInterval = window.setInterval(() => {
            const data = this.boobaRead();
            if (data && data.length > 0 && this.onDataCallback) {
                this.onDataCallback(data);
            }
        }, this.pollMs);
        onStateChange('connected', 'Connected');
        console.log('WASM adapter connected, polling at', this.pollMs, 'ms');
    }
    disconnect() {
        if (this.pollInterval !== null) {
            clearInterval(this.pollInterval);
            this.pollInterval = null;
        }
        this.onDataCallback = null;
    }
}
//# sourceMappingURL=adapter.js.map