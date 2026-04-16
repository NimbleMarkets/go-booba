import { MsgInput, MsgOutput, MsgResize, MsgPing, MsgPong, MsgTitle, MsgOptions, MsgClose, encodeWTMessage, jsonPayload, parseJsonPayload, } from './protocol.js';
export class BoobaWebTransportAdapter {
    constructor(url, certHash, callbacks = {}) {
        this.url = url;
        this.certHash = certHash;
        this.transport = null;
        this.writer = null;
        this.onDataCallback = null;
        this.onStateChangeCallback = null;
        this.pingInterval = null;
        this.closed = false;
        this.callbacks = callbacks;
    }
    boobaRead() {
        return null;
    }
    boobaWrite(data) {
        const bytes = typeof data === 'string' ? new TextEncoder().encode(data) : data;
        this._write(MsgInput, bytes);
    }
    boobaResize(cols, rows) {
        this._write(MsgResize, jsonPayload({ cols, rows }));
    }
    async connect(onData, onStateChange) {
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        this.closed = false;
        onStateChange('connecting', 'Connecting (WebTransport)...');
        try {
            // Convert hex cert hash to Uint8Array for serverCertificateHashes
            const hashBytes = new Uint8Array(this.certHash.match(/.{2}/g).map(h => parseInt(h, 16)));
            this.transport = new WebTransport(this.url, {
                serverCertificateHashes: [{
                        algorithm: 'sha-256',
                        value: hashBytes,
                    }],
            });
            await this.transport.ready;
            const stream = await this.transport.createBidirectionalStream();
            this.writer = stream.writable.getWriter();
            onStateChange('connected', 'Connected (WebTransport)');
            this._startPing();
            // Read from the stream
            this._readLoop(stream.readable);
            // Handle transport closure
            this.transport.closed.then(() => {
                if (!this.closed) {
                    this._stopPing();
                    onStateChange('disconnected', 'Disconnected');
                }
            }).catch(() => {
                if (!this.closed) {
                    this._stopPing();
                    onStateChange('disconnected', 'Disconnected');
                }
            });
        }
        catch (err) {
            onStateChange('disconnected', `WebTransport failed: ${err}`);
            throw err; // Let the auto adapter catch this and fall back
        }
    }
    async _readLoop(readable) {
        const reader = readable.getReader();
        let buffer = new Uint8Array(0);
        try {
            while (true) {
                const { value, done } = await reader.read();
                if (done)
                    break;
                if (!value)
                    continue;
                // Append to buffer
                const newBuf = new Uint8Array(buffer.length + value.length);
                newBuf.set(buffer);
                newBuf.set(value, buffer.length);
                buffer = newBuf;
                // Parse length-prefixed messages
                while (buffer.length >= 4) {
                    const msgLen = new DataView(buffer.buffer, buffer.byteOffset).getUint32(0, false);
                    if (buffer.length < 4 + msgLen)
                        break; // Incomplete message
                    const msgType = buffer[4];
                    const payload = buffer.slice(5, 4 + msgLen);
                    buffer = buffer.slice(4 + msgLen);
                    this._handleMessage(msgType, payload);
                }
            }
        }
        catch {
            // Stream closed
        }
        finally {
            reader.releaseLock();
        }
    }
    _handleMessage(msgType, payload) {
        switch (msgType) {
            case MsgOutput:
                this.onDataCallback?.(payload);
                break;
            case MsgPong:
                break;
            case MsgTitle:
                this.callbacks.onTitle?.(new TextDecoder().decode(payload));
                break;
            case MsgOptions:
                this.callbacks.onOptions?.(parseJsonPayload(payload));
                break;
            case MsgClose: {
                this.closed = true;
                const reason = payload.length > 0 ? new TextDecoder().decode(payload) : 'Session ended';
                this.callbacks.onClose?.(reason);
                break;
            }
            default:
                break;
        }
    }
    _write(msgType, payload) {
        if (!this.writer)
            return;
        const msg = encodeWTMessage(msgType, payload);
        this.writer.write(msg).catch(() => { });
    }
    _startPing() {
        this.pingInterval = window.setInterval(() => {
            this._write(MsgPing);
        }, 30000);
    }
    _stopPing() {
        if (this.pingInterval !== null) {
            clearInterval(this.pingInterval);
            this.pingInterval = null;
        }
    }
    disconnect() {
        this.closed = true;
        this._stopPing();
        this.writer?.close().catch(() => { });
        this.writer = null;
        this.transport?.close();
        this.transport = null;
        this.onDataCallback = null;
    }
}
//# sourceMappingURL=webtransport_adapter.js.map