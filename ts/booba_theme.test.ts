import { describe, expect, it, vi } from 'vitest';
vi.mock('ghostty-web', () => {
    const handler = () => vi.fn(() => ({ dispose: vi.fn() }));
    class Terminal {
        options: Record<string, unknown>;
        cols = 80; rows = 24;
        constructor(options: Record<string, unknown>) { this.options = options; }
        open = vi.fn(async () => {});
        loadAddon = vi.fn(); dispose = vi.fn();
        onResize = handler(); onData = handler(); onBell = handler(); onSelectionChange = handler();
        onKey = handler(); onTitleChange = handler(); onScroll = handler(); onRender = handler(); onCursorMove = handler();
    }
    class FitAddon { fit = vi.fn(); observeResize = vi.fn(); }
    return { init: vi.fn(async () => {}), Terminal, FitAddon, installRendererHud: vi.fn(), parseRendererFromURL: vi.fn() };
});
import { BoobaTerminal } from './booba.js';

describe('BoobaTerminal theme bridge', () => {
    it('init() exposes the resolved theme colors as custom properties on the container', async () => {
        const container = document.createElement('div'); container.id = 'theme-bridge';
        document.body.append(container);
        vi.spyOn(console, 'log').mockImplementation(() => {});
        const booba = new BoobaTerminal('theme-bridge', { theme: { background: '#10151d', foreground: '#e4edf7' } });
        await booba.init();
        expect(container.style.getPropertyValue('--booba-background')).toBe('#10151d');
        expect(container.style.getPropertyValue('--booba-foreground')).toBe('#e4edf7');
        booba.dispose();
        container.remove();
    });
});
