/**
 * Booba's BubbleTea Communication Adapter
 * 
 * Provides an abstraction layer for communicating with BubbleTea programs.
 * Supports both WebSocket (backend-server) and WASM (pure embedding) modes.
 */

export type BoobaConnectionState = 'connecting' | 'connected' | 'disconnected';

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
     */
    boobaResize(cols: number, rows: number): void;

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
 * WebSocket-based adapter for backend-server mode
 * 
 * Uses a custom binary protocol:
 * - 0x01 + data: User input
 * - 0x02 + JSON: Terminal resize
 */
export class BoobaWebSocketAdapter implements BoobaAdapter {
    private ws: WebSocket | null = null;
    private onDataCallback: ((data: string | Uint8Array) => void) | null = null;

    constructor(private url: string) { }

    boobaRead(): string | Uint8Array | null {
        // WebSocket is push-based, data arrives via onmessage
        // This method is not used in WebSocket mode
        return null;
    }

    boobaWrite(data: string | Uint8Array): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            console.warn('WebSocket not connected, cannot write');
            return;
        }

        // Protocol: 0x01 + data
        const dataStr = typeof data === 'string' ? data : new TextDecoder().decode(data);
        const payload = new Uint8Array(dataStr.length + 1);
        payload[0] = 0x01;
        for (let i = 0; i < dataStr.length; i++) {
            payload[i + 1] = dataStr.charCodeAt(i);
        }
        this.ws.send(payload);
    }

    boobaResize(cols: number, rows: number): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            console.warn('WebSocket not connected, cannot resize');
            return;
        }

        // Protocol: 0x02 + JSON
        const json = JSON.stringify({ cols, rows });
        const encoder = new TextEncoder();
        const jsonBytes = encoder.encode(json);
        const payload = new Uint8Array(jsonBytes.length + 1);
        payload[0] = 0x02;
        payload.set(jsonBytes, 1);
        this.ws.send(payload);
        console.log('Sent resize:', cols, rows);
    }

    connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): void {
        this.onDataCallback = onData;
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            onStateChange('connected', 'Connected');
        };

        this.ws.onmessage = (e: MessageEvent) => {
            if (e.data instanceof Blob) {
                const reader = new FileReader();
                reader.onload = () => {
                    onData(new Uint8Array(reader.result as ArrayBuffer));
                };
                reader.readAsArrayBuffer(e.data);
            } else {
                onData(e.data);
            }
        };

        this.ws.onclose = (e: CloseEvent) => {
            console.log('WebSocket disconnected', e.code, e.reason);
            onStateChange('disconnected', 'Disconnected');
        };

        this.ws.onerror = (e: Event) => {
            console.error('WebSocket error:', e);
            onStateChange('disconnected', 'Error');
        };

        onStateChange('connecting', 'Connecting...');
    }

    disconnect(): void {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.onDataCallback = null;
    }
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

    boobaResize(cols: number, rows: number): void {
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

        // Check if WASM functions are available
        if (typeof (window as any).bubbletea_read !== 'function') {
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

    disconnect(): void {
        if (this.pollInterval !== null) {
            clearInterval(this.pollInterval);
            this.pollInterval = null;
        }
        this.onDataCallback = null;
    }
}
