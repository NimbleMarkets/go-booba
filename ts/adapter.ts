/**
 * Booba's BubbleTea Communication Adapter
 * 
 * Provides an abstraction layer for communicating with BubbleTea programs.
 * Supports both WebSocket (backend-server) and WASM (pure embedding) modes.
 */

export type BoobaConnectionState = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

export interface BoobaAdapter {
    /**
     * Read output from the BubbleTea program
     * @returns Data from the program, or null if no data available
     */
    boobaRead(): string | Uint8Array | null;

    /**
     * Write input to the BubbleTea program
     * @param data User input to send
     */
    boobaWrite(data: string | Uint8Array): void;

    /**
     * Notify the BubbleTea program of a terminal resize
     * @param cols Number of columns
     * @param rows Number of rows
     * @param widthPx Optional canvas width in pixels (forwarded to PTY for kitty graphics tools)
     * @param heightPx Optional canvas height in pixels
     */
    boobaResize(cols: number, rows: number, widthPx?: number, heightPx?: number): void;

    /**
     * Set up the connection and start listening for data
     * @param onData Callback when data is received from BubbleTea
     * @param onStateChange Callback when connection state changes
     */
    connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): void;

    /**
     * Disconnect and clean up resources
     */
    disconnect(): void;
}

/**
 * WASM-based adapter for pure embedding mode
 * 
 * Communicates with BubbleTea via global WASM functions:
 * - window.booba_write(data: string): void
 * - window.booba_read(): string
 * - window.booba_resize(cols: number, rows: number): void
 */
export class BoobaWasmAdapter implements BoobaAdapter {
    private pollInterval: number | null = null;
    private onDataCallback: ((data: string | Uint8Array) => void) | null = null;

    constructor(private pollMs: number = 16) { } // ~60fps

    boobaRead(): string | null {
        if (typeof (window as any).bubbletea_read !== 'function') {
            console.warn('bubbletea_read not available');
            return null;
        }
        const data = (window as any).bubbletea_read();
        return data || null;
    }

    boobaWrite(data: string | Uint8Array): void {
        if (typeof (window as any).bubbletea_write !== 'function') {
            console.warn('bubbletea_write not available');
            return;
        }
        const dataStr = typeof data === 'string' ? data : new TextDecoder().decode(data);
        (window as any).bubbletea_write(dataStr);
    }

    boobaResize(cols: number, rows: number, _widthPx?: number, _heightPx?: number): void {
        if (typeof (window as any).bubbletea_resize !== 'function') {
            console.warn('bubbletea_resize not available');
            return;
        }
        (window as any).bubbletea_resize(cols, rows);
        console.log('Sent resize to WASM:', cols, rows);
    }

    connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): void {
        this.onDataCallback = onData;

        const startPolling = () => {
            this.pollInterval = window.setInterval(() => {
                const data = this.boobaRead();
                if (data && data.length > 0 && this.onDataCallback) {
                    this.onDataCallback(data);
                }
            }, this.pollMs);

            onStateChange('connected', 'Connected');
            console.log('WASM adapter connected, polling at', this.pollMs, 'ms');
        };

        if (typeof (window as any).bubbletea_read === 'function') {
            startPolling();
            return;
        }

        // Wait for WASM module to initialize
        let attempts = 0;
        const checkInterval = window.setInterval(() => {
            if (typeof (window as any).bubbletea_read === 'function') {
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

    disconnect(): void {
        if (this.pollInterval !== null) {
            clearInterval(this.pollInterval);
            this.pollInterval = null;
        }
        this.onDataCallback = null;
    }
}
