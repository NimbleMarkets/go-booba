import { afterEach, describe, expect, it, vi } from 'vitest';
vi.mock('ghostty-web', () => {
    const handler = () => vi.fn(() => ({ dispose: vi.fn() }));
    class Terminal {
        cols = 80; rows = 24;
        options: Record<string, unknown>;
        renderer = { getMetrics: () => ({ width: 10, height: 20 }) };
        constructor(options: Record<string, unknown>) { this.options = options; }
        open = async (host: HTMLElement) => { host.append(document.createElement('canvas')); };
        hasMouseTracking = () => true;
        loadAddon = vi.fn(); dispose = vi.fn();
        onResize = handler(); onData = handler(); onBell = handler(); onSelectionChange = handler();
        onKey = handler(); onTitleChange = handler(); onScroll = handler(); onRender = handler(); onCursorMove = handler();
    }
    class FitAddon { fit = vi.fn(); observeResize = vi.fn(); }
    return { init: vi.fn(async () => {}), Terminal, FitAddon, installRendererHud: vi.fn(), parseRendererFromURL: vi.fn() };
});
import { BoobaTerminal } from './booba.js';
import { BoobaProtocolAdapter, type WebSocketAdapterCallbacks } from './websocket_adapter.js';

afterEach(() => { vi.restoreAllMocks(); });

describe('BoobaTerminal touch integration', () => {
    it('sends gestures and onData to the same adapter sink and honors disableStdin and server read-only', async () => {
        vi.spyOn(console, 'log').mockImplementation(() => {});
        vi.spyOn(BoobaProtocolAdapter.prototype, 'connect').mockImplementation(() => {});
        const write = vi.spyOn(BoobaProtocolAdapter.prototype, 'boobaWrite').mockImplementation(() => {});
        const host = document.createElement('div'); host.id = 'touch-integration'; document.body.append(host);
        const booba = new BoobaTerminal(host.id);
        try {
            await booba.init();
            booba.connectWebSocket('ws://example.invalid');
            const callbacks = (booba.adapter as unknown as { callbacks: WebSocketAdapterCallbacks }).callbacks;
            const canvas = host.querySelector('canvas')!;
            vi.spyOn(canvas, 'getBoundingClientRect').mockReturnValue({ left: 10, top: 20 } as DOMRect);
            function tap() {
                const touch = { identifier: 1, clientX: 35, clientY: 65 };
                for (const type of ['touchstart', 'touchend']) {
                    const event = new Event(type, { bubbles: true, cancelable: true });
                    Object.assign(event, { touches: type === 'touchstart' ? [touch] : [], changedTouches: [touch] });
                    canvas.dispatchEvent(event);
                }
            }
            tap(); expect(write).not.toHaveBeenCalled(); // Await server options.
            callbacks.onOptions!({ readOnly: false });
            tap();
            const onData = vi.mocked(booba.term!.onData).mock.calls[0][0];
            onData('a');
            expect(write.mock.calls).toEqual([['\x1b[<0;3;3M'], ['\x1b[<0;3;3m'], ['a']]);
            write.mockClear();
            booba.term!.options.disableStdin = true; tap();
            booba.term!.options.disableStdin = false;
            booba.options.disableStdin = true; tap();
            booba.options.disableStdin = false;
            callbacks.onOptions!({ readOnly: true }); tap();
            expect(write).not.toHaveBeenCalled();
        } finally { booba.dispose(); host.remove(); }
    });
});
