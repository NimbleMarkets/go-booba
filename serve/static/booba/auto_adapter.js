import { BoobaProtocolAdapter } from './websocket_adapter.js';
import { BoobaWebTransportAdapter } from './webtransport_adapter.js';
export class BoobaAutoAdapter {
    constructor(wsUrl, wtUrl, certHashUrl, callbacks = {}) {
        this.wsUrl = wsUrl;
        this.wtUrl = wtUrl;
        this.certHashUrl = certHashUrl;
        this.callbacks = callbacks;
        this.adapter = null;
        this.onDataCallback = null;
        this.onStateChangeCallback = null;
    }
    boobaRead() {
        return this.adapter?.boobaRead() ?? null;
    }
    boobaWrite(data) {
        this.adapter?.boobaWrite(data);
    }
    boobaResize(cols, rows) {
        this.adapter?.boobaResize(cols, rows);
    }
    connect(onData, onStateChange) {
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        this._tryConnect();
    }
    async _tryConnect() {
        // Try WebTransport first if URL and cert hash endpoint are available
        if (this.wtUrl && this.certHashUrl && typeof WebTransport !== 'undefined') {
            try {
                const resp = await fetch(this.certHashUrl);
                if (resp.ok) {
                    const { hash } = await resp.json();
                    const wt = new BoobaWebTransportAdapter(this.wtUrl, hash, this.callbacks);
                    this.adapter = wt;
                    await wt.connect(this.onDataCallback, this.onStateChangeCallback);
                    return; // WebTransport connected successfully
                }
            }
            catch {
                // WebTransport failed — fall through to WebSocket
            }
        }
        // Fall back to WebSocket
        const ws = new BoobaProtocolAdapter(this.wsUrl, this.callbacks);
        this.adapter = ws;
        ws.connect(this.onDataCallback, this.onStateChangeCallback);
    }
    disconnect() {
        this.adapter?.disconnect();
        this.adapter = null;
    }
}
//# sourceMappingURL=auto_adapter.js.map