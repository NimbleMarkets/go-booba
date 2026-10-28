import { BoobaProtocolAdapter } from './websocket_adapter.js';
import { BoobaWebTransportAdapter } from './webtransport_adapter.js';
export class BoobaAutoAdapter {
    constructor(wsUrl, wtUrl, certHashUrl, callbacks = {}) {
        this.wsUrl = wsUrl;
        this.wtUrl = wtUrl;
        this.certHashUrl = certHashUrl;
        this.callbacks = callbacks;
        this.adapter = null;
        this.generation = 0;
        this.onDataCallback = null;
        this.onStateChangeCallback = null;
    }
    boobaRead() {
        return this.adapter?.boobaRead() ?? null;
    }
    boobaWrite(data) {
        this.adapter?.boobaWrite(data);
    }
    boobaResize(cols, rows, widthPx, heightPx) {
        this.adapter?.boobaResize(cols, rows, widthPx, heightPx);
    }
    connect(onData, onStateChange) {
        this.disconnect();
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        void this._tryConnect(this.generation);
    }
    async _tryConnect(generation) {
        // Try WebTransport first if URL and cert hash endpoint are available
        if (this.wtUrl && this.certHashUrl && typeof WebTransport !== 'undefined') {
            try {
                const resp = await fetch(this.certHashUrl);
                if (resp.ok) {
                    const { hash } = await resp.json();
                    if (generation !== this.generation)
                        return;
                    const wt = new BoobaWebTransportAdapter(this.wtUrl, hash, this.callbacks);
                    this.adapter = wt;
                    let connected = false;
                    await wt.connect(this.onDataCallback, (state, message) => {
                        if (generation !== this.generation)
                            return;
                        if (state === 'connected')
                            connected = true;
                        // A failed initial attempt is followed by WebSocket, not
                        // a closed terminal session.
                        if (state !== 'disconnected' || connected)
                            this.onStateChangeCallback(state, message);
                    });
                    return; // WebTransport connected successfully
                }
            }
            catch {
                // WebTransport failed — fall through to WebSocket
            }
        }
        if (generation !== this.generation)
            return;
        // Fall back to WebSocket
        const ws = new BoobaProtocolAdapter(this.wsUrl, this.callbacks);
        this.adapter = ws;
        ws.connect(this.onDataCallback, this.onStateChangeCallback);
    }
    disconnect() {
        this.generation++;
        this.adapter?.disconnect();
        this.adapter = null;
    }
}
