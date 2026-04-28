/**
 * WebSocket adapter speaking the Booba/Sip protocol ('0'-'8' message types).
 */
import { BoobaAdapter, BoobaConnectionState } from './adapter.js';
import {
    MsgInput, MsgOutput, MsgResize, MsgPing, MsgPong,
    MsgTitle, MsgOptions, MsgClose, MsgKittyKbd,
    encodeWSMessage, decodeWSMessage, jsonPayload, parseJsonPayload,
    type ResizeMessage, type OptionsMessage,
} from './protocol.js';

export interface WebSocketAdapterCallbacks {
    onTitle?: (title: string) => void;
    onOptions?: (opts: OptionsMessage) => void;
    onClose?: (reason: string) => void;
}

export class BoobaProtocolAdapter implements BoobaAdapter {
    private ws: WebSocket | null = null;
    private onDataCallback: ((data: string | Uint8Array) => void) | null = null;
    private pingInterval: number | null = null;
    private reconnectAttempts = 0;
    private maxReconnectAttempts = 5;
    private reconnectBaseDelay = 1000;
    private reconnectMultiplier = 1.5;
    private onStateChangeCallback: ((state: BoobaConnectionState, message: string) => void) | null = null;
    private callbacks: WebSocketAdapterCallbacks;
    private shouldReconnect = true;

    constructor(private url: string, callbacks: WebSocketAdapterCallbacks = {}) {
        this.callbacks = callbacks;
    }

    boobaRead(): string | Uint8Array | null {
        return null;
    }

    boobaWrite(data: string | Uint8Array): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
        const bytes = typeof data === 'string' ? new TextEncoder().encode(data) : data;
        this.ws.send(encodeWSMessage(MsgInput, bytes));
    }

    boobaResize(cols: number, rows: number, widthPx?: number, heightPx?: number): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
        const msg: ResizeMessage = { cols, rows };
        if (widthPx && widthPx > 0) msg.widthPx = widthPx;
        if (heightPx && heightPx > 0) msg.heightPx = heightPx;
        this.ws.send(encodeWSMessage(MsgResize, jsonPayload(msg)));
    }

    connect(
        onData: (data: string | Uint8Array) => void,
        onStateChange: (state: BoobaConnectionState, message: string) => void
    ): void {
        this.onDataCallback = onData;
        this.onStateChangeCallback = onStateChange;
        this.shouldReconnect = true;
        this._connect();
    }

    private _connect(): void {
        this.onStateChangeCallback?.('connecting', 'Connecting...');

        this.ws = new WebSocket(this.url);
        this.ws.binaryType = 'arraybuffer';

        this.ws.onopen = () => {
            this.reconnectAttempts = 0;
            this.onStateChangeCallback?.('connected', 'Connected');
            this._startPing();
        };

        this.ws.onmessage = (e: MessageEvent) => {
            const data = new Uint8Array(e.data as ArrayBuffer);
            const [msgType, payload] = decodeWSMessage(data);

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
                    this.shouldReconnect = false;
                    const reason = payload.length > 0 ? new TextDecoder().decode(payload) : 'Session ended';
                    this.callbacks.onClose?.(reason);
                    break;
                }
                default:
                    break;
            }
        };

        this.ws.onclose = () => {
            this._stopPing();
            if (this.shouldReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
                this._reconnect();
            } else {
                this.onStateChangeCallback?.('disconnected', 'Disconnected');
            }
        };

        this.ws.onerror = () => {};
    }

    private _reconnect(): void {
        this.reconnectAttempts++;
        const delay = this.reconnectBaseDelay * Math.pow(this.reconnectMultiplier, this.reconnectAttempts - 1);
        this.onStateChangeCallback?.('reconnecting' as BoobaConnectionState,
            `Reconnecting (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);
        setTimeout(() => this._connect(), delay);
    }

    private _startPing(): void {
        this.pingInterval = window.setInterval(() => {
            if (this.ws?.readyState === WebSocket.OPEN) {
                this.ws.send(encodeWSMessage(MsgPing));
            }
        }, 30000);
    }

    private _stopPing(): void {
        if (this.pingInterval !== null) {
            clearInterval(this.pingInterval);
            this.pingInterval = null;
        }
    }

    disconnect(): void {
        this.shouldReconnect = false;
        this._stopPing();
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.onDataCallback = null;
    }
}
