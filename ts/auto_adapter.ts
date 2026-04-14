/**
 * Auto-detecting adapter that tries WebTransport first, then falls back to WebSocket.
 */
import { BoobaAdapter, BoobaConnectionState } from './adapter.js';
import { BoobaProtocolAdapter, type WebSocketAdapterCallbacks } from './websocket_adapter.js';
import { BoobaWebTransportAdapter } from './webtransport_adapter.js';

export class BoobaAutoAdapter implements BoobaAdapter {
    private adapter: BoobaAdapter | null = null;
    private onDataCallback: ((data: string | Uint8Array) => void) | null = null;
    private onStateChangeCallback: ((state: BoobaConnectionState, message: string) => void) | null = null;

    constructor(
        private wsUrl: string,
        private wtUrl: string | null,
        private certHashUrl: string | null,
        private callbacks: WebSocketAdapterCallbacks = {},
    ) {}

    boobaRead(): string | Uint8Array | null {
        return this.adapter?.boobaRead() ?? null;
    }

    boobaWrite(data: string | Uint8Array): void {
        this.adapter?.boobaWrite(data);
    }

    boobaResize(cols: number, rows: number): void {
        this.adapter?.boobaResize(cols, rows);
    }

    connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): void {
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        this._tryConnect();
    }

    private async _tryConnect(): Promise<void> {
        // Try WebTransport first if URL and cert hash endpoint are available
        if (this.wtUrl && this.certHashUrl && typeof WebTransport !== 'undefined') {
            try {
                const resp = await fetch(this.certHashUrl);
                if (resp.ok) {
                    const { hash } = await resp.json();
                    const wt = new BoobaWebTransportAdapter(this.wtUrl, hash, this.callbacks);
                    this.adapter = wt;
                    await wt.connect(this.onDataCallback!, this.onStateChangeCallback!);
                    return; // WebTransport connected successfully
                }
            } catch {
                // WebTransport failed — fall through to WebSocket
            }
        }

        // Fall back to WebSocket
        const ws = new BoobaProtocolAdapter(this.wsUrl, this.callbacks);
        this.adapter = ws;
        ws.connect(this.onDataCallback!, this.onStateChangeCallback!);
    }

    disconnect(): void {
        this.adapter?.disconnect();
        this.adapter = null;
    }
}
