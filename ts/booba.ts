// @ts-ignore - Import will resolve at runtime in browser
import { init, Terminal, FitAddon } from '../ghostty-web/ghostty-web.js';
import { BoobaAdapter, BoobaConnectionState, BoobaWebSocketAdapter, BoobaWasmAdapter } from './adapter.js';

export interface BoobaTerminalOptions {
    fontSize?: number;
    fontFamily?: string;
    cols?: number;
    rows?: number;
    cursorBlink?: boolean;
    cursorStyle?: 'block' | 'underline' | 'bar';
    scrollback?: number;
    allowTransparency?: boolean;
    convertEol?: boolean;
    disableStdin?: boolean;
    smoothScrollDuration?: number;
    theme?: {
        foreground?: string;
        background?: string;
        cursor?: string;
        cursorAccent?: string;
        selectionBackground?: string;
        selectionForeground?: string;
        black?: string;
        red?: string;
        green?: string;
        yellow?: string;
        blue?: string;
        magenta?: string;
        cyan?: string;
        white?: string;
        brightBlack?: string;
        brightRed?: string;
        brightGreen?: string;
        brightYellow?: string;
        brightBlue?: string;
        brightMagenta?: string;
        brightCyan?: string;
        brightWhite?: string;
    };
}

export class BoobaTerminal {
    container: HTMLElement | null;
    options: BoobaTerminalOptions;
    term: any; // Using any for now as we don't have full types for Terminal
    adapter: BoobaAdapter | null;
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
        this.adapter = null;
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
            this.adapter?.boobaResize(size.cols, size.rows);
        });

        console.log('Terminal opened. Size:', this.term.cols, 'x', this.term.rows);

        // Send user input through adapter
        this.term.onData((data: string) => {
            this.adapter?.boobaWrite(data);
        });
    }

    /**
     * Connect to a BubbleTea backend via WebSocket
     * @param url WebSocket URL (e.g., 'ws://localhost:8080/ws')
     */
    connectWebSocket(url: string) {
        this.adapter = new BoobaWebSocketAdapter(url);
        this._setupAdapter();
    }

    /**
     * Connect to a BubbleTea program running in WASM
     * @param pollMs Polling interval in milliseconds (default: 16ms / ~60fps)
     */
    connectWasm(pollMs: number = 16) {
        this.adapter = new BoobaWasmAdapter(pollMs);
        this._setupAdapter();
    }

    /**
     * Use a custom adapter implementation
     * @param adapter Custom BubbleTeaAdapter
     */
    connectAdapter(adapter: BoobaAdapter) {
        this.adapter = adapter;
        this._setupAdapter();
    }

    private _setupAdapter() {
        if (!this.adapter) return;

        this.adapter.connect(
            (data: string | Uint8Array) => {
                // Write data from BubbleTea to terminal
                this.term.write(data);
            },
            (state: BoobaConnectionState, message: string) => {
                // Update connection status
                this._updateStatus(state, message);

                // Send initial size when connected
                if (state === 'connected' && this.term) {
                    this.adapter?.boobaResize(this.term.cols, this.term.rows);
                }

                // Show disconnect message in terminal
                if (state === 'disconnected') {
                    this.term.write('\r\nConnection closed.\r\n');
                }
            }
        );
    }

    disconnect() {
        this.adapter?.disconnect();
        this.adapter = null;
    }

    _updateStatus(state: string, message: string) {
        if (this.onStatusChange) {
            this.onStatusChange(state, message);
        }
    }
}

// Re-export adapter types for convenience
export { BoobaAdapter, BoobaWebSocketAdapter, BoobaWasmAdapter, BoobaConnectionState };
