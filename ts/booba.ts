// @ts-ignore - Import will resolve at runtime in browser
import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';

export interface BoobaTerminalOptions {
    fontSize?: number;
    cols?: number;
    rows?: number;
    theme?: {
        background?: string;
        foreground?: string;
        [key: string]: string | undefined;
    };
    [key: string]: any; // Allow additional options to pass through to Terminal
}

export class BoobaTerminal {
    container: HTMLElement | null;
    options: BoobaTerminalOptions;
    term: any; // Using any for now as we don't have full types for Terminal
    ws: WebSocket | null;
    onStatusChange: ((state: string, message: string) => void) | null;
    fitAddon: FitAddon | null;

    constructor(containerId: string, options: BoobaTerminalOptions = {}) {
        this.container = document.getElementById(containerId);
        this.options = {
            fontSize: 14,
            cols: 80,
            rows: 24,
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
            },
            ...options
        };
        this.term = null;
        this.ws = null;
        this.onStatusChange = null;
        this.fitAddon = null;
    }

    async init() {
        await init();
        this.term = new Terminal(this.options);

        this.fitAddon = new FitAddon();
        this.term.loadAddon(this.fitAddon);

        this.term.open(this.container);
        this.fitAddon.fit();
        this.fitAddon.observeResize();

        // Handle window resize as fallback
        window.addEventListener('resize', () => {
            this.fitAddon?.fit();
        });

        // Listen for resize events from the terminal (triggered by fit addon)
        this.term.onResize((size: { cols: number; rows: number }) => {
            this.sendResize(size.cols, size.rows);
        });

        console.log('Terminal opened. Size:', this.term.cols, 'x', this.term.rows);

        this.term.onData((data: string) => {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                // Protocol: 0x01 + data
                const payload = new Uint8Array(data.length + 1);
                payload[0] = 0x01;
                for (let i = 0; i < data.length; i++) {
                    payload[i + 1] = data.charCodeAt(i);
                }
                this.ws.send(payload);
            }
        });
    }

    connect(url: string) {
        this.ws = new WebSocket(url);

        this.ws.onopen = () => {
            console.log('Connected to WebSocket');
            this._updateStatus('connected', 'Connected');
            // Send current size immediately
            if (this.term) {
                this.sendResize(this.term.cols, this.term.rows);
            }
        };

        this.ws.onmessage = (e: MessageEvent) => {
            if (e.data instanceof Blob) {
                const reader = new FileReader();
                reader.onload = () => {
                    this.term.write(new Uint8Array(reader.result as ArrayBuffer));
                };
                reader.readAsArrayBuffer(e.data);
            } else {
                this.term.write(e.data);
            }
        };

        this.ws.onclose = (e: CloseEvent) => {
            console.log('Disconnected from WebSocket', e.code, e.reason);
            this.term.write('\r\nConnection closed.\r\n');
            this._updateStatus('disconnected', 'Disconnected');
        };

        this.ws.onerror = (e: Event) => {
            console.error('WebSocket Error:', e);
            this._updateStatus('disconnected', 'Error');
        };
    }

    sendResize(cols: number, rows: number) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
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
    }

    _updateStatus(state: string, message: string) {
        if (this.onStatusChange) {
            this.onStatusChange(state, message);
        }
    }
}
