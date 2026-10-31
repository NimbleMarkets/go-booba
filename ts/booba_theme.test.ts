import { describe, expect, it, vi } from 'vitest';
vi.mock('ntwebterm', () => {
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
import { BoobaTerminal, type BoobaTerminalOptions } from './booba.js';

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


describe('BoobaTerminal font options', () => {
    it.each([{}, { fontFamily: undefined }])('uses the preferred local stack with %j', async options => {
        const container = document.createElement('div');
        container.id = 'font-default';
        document.body.append(container);
        const booba = new BoobaTerminal(container.id, options);
        try {
            await booba.init();
            expect(booba.term!.options.fontFamily).toBe('"JetBrains Mono", "Iosevka", "Cascadia Code", ui-monospace, monospace');
            expect(booba.term!.options.fontSize).toBe(15);
            expect(booba.term!.options.fontFaces).toBeUndefined();
        } finally {
            booba.dispose();
            container.remove();
        }
    });

    it.each([undefined, [], [{
        family: 'My Mono', src: ['local("My Mono")', 'url(/fonts/my-mono.woff2)'],
        weight: 400, style: 'normal', display: 'swap' as const,
    }]])('preserves caller font options and face identity (%j)', async fontFaces => {
        const container = document.createElement('div');
        container.id = 'font-override';
        document.body.append(container);
        const options: BoobaTerminalOptions = { fontFamily: 'My Mono, monospace', fontSize: 14, fontFaces };
        const booba = new BoobaTerminal(container.id, options);
        try {
            await booba.init();
            expect(booba.term!.options.fontFamily).toBe(options.fontFamily);
            expect(booba.term!.options.fontSize).toBe(14);
            expect(booba.term!.options.fontFaces).toBe(fontFaces);
            expect(options).toEqual({ fontFamily: 'My Mono, monospace', fontSize: 14, fontFaces });
        } finally {
            booba.dispose();
            container.remove();
        }
    });
});
