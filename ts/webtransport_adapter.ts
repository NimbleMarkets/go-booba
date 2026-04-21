/**
 * WebTransport adapter speaking the Booba/Sip protocol with length-prefixed framing.
 *
 * WebTransport uses QUIC for lower latency than WebSocket.
 * Requires TLS — the server provides a /cert-hash endpoint for self-signed cert pinning.
 */
import { BoobaAdapter, BoobaConnectionState } from './adapter.js';
import {
    MsgInput, MsgOutput, MsgResize, MsgPing, MsgPong,
    MsgTitle, MsgOptions, MsgClose,
    encodeWTMessage, jsonPayload, parseJsonPayload, tryDecodeWTFrame,
    type OptionsMessage,
} from './protocol.js';
import type { WebSocketAdapterCallbacks } from './websocket_adapter.js';

export class BoobaWebTransportAdapter implements BoobaAdapter {
    private transport: WebTransport | null = null;
    private writer: WritableStreamDefaultWriter<Uint8Array> | null = null;
    private onDataCallback: ((data: string | Uint8Array) => void) | null = null;
    private onStateChangeCallback: ((state: BoobaConnectionState, message: string) => void) | null = null;
    private pingInterval: number | null = null;
    private callbacks: WebSocketAdapterCallbacks;
    private closed = false;

    constructor(private url: string, private certHash: string, callbacks: WebSocketAdapterCallbacks = {}) {
        this.callbacks = callbacks;
    }

    boobaRead(): string | Uint8Array | null {
        return null;
    }

    boobaWrite(data: string | Uint8Array): void {
        const bytes = typeof data === 'string' ? new TextEncoder().encode(data) : data;
        this._write(MsgInput, bytes);
    }

    boobaResize(cols: number, rows: number): void {
        this._write(MsgResize, jsonPayload({ cols, rows }));
    }

    async connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): Promise<void> {
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        this.closed = false;

        onStateChange('connecting', 'Connecting (WebTransport)...');

        try {
            // Convert hex cert hash to Uint8Array for serverCertificateHashes
            const hashBytes = new Uint8Array(this.certHash.match(/.{2}/g)!.map(h => parseInt(h, 16)));

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

        } catch (err) {
            onStateChange('disconnected', `WebTransport failed: ${err}`);
            throw err; // Let the auto adapter catch this and fall back
        }
    }

    private async _readLoop(readable: ReadableStream<Uint8Array>): Promise<void> {
        const reader = readable.getReader();
        // Pre-allocate a growable buffer; expands by doubling when needed.
        let buf = new Uint8Array(4096);
        let len = 0;

        const grow = (need: number) => {
            if (buf.length >= need) return;
            let n = buf.length * 2;
            while (n < need) n *= 2;
            const next = new Uint8Array(n);
            next.set(buf.subarray(0, len));
            buf = next;
        };

        try {
            while (true) {
                const { value, done } = await reader.read();
                if (done) break;
                if (!value) continue;

                grow(len + value.length);
                buf.set(value, len);
                len += value.length;

                // Parse complete length-prefixed messages
                let consumed = 0;
                while (true) {
                    const frame = tryDecodeWTFrame(buf, consumed, len);
                    if (!frame) break;
                    this._handleMessage(frame.msgType, frame.payload);
                    consumed += frame.consumed;
                }

                // Shift any unconsumed bytes to the front
                if (consumed > 0) {
                    const remaining = len - consumed;
                    if (remaining > 0) {
                        buf.set(buf.subarray(consumed, len), 0);
                    }
                    len = remaining;
                }
            }
        } catch {
            // Stream closed
        } finally {
            reader.releaseLock();
        }
    }

    private _handleMessage(msgType: number, payload: Uint8Array): void {
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
                this.callbacks.onOptions?.(parseJsonPayload<OptionsMessage>(payload));
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

    private _write(msgType: number, payload?: Uint8Array): void {
        if (!this.writer) return;
        const msg = encodeWTMessage(msgType, payload);
        this.writer.write(msg).catch(() => {});
    }

    private _startPing(): void {
        this.pingInterval = window.setInterval(() => {
            this._write(MsgPing);
        }, 30000);
    }

    private _stopPing(): void {
        if (this.pingInterval !== null) {
            clearInterval(this.pingInterval);
            this.pingInterval = null;
        }
    }

    disconnect(): void {
        this.closed = true;
        this._stopPing();
        this.writer?.close().catch(() => {});
        this.writer = null;
        this.transport?.close();
        this.transport = null;
        this.onDataCallback = null;
    }
}
