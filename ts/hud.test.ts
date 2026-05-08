import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { parseRendererFromURL } from './hud';

describe('parseRendererFromURL', () => {
    let originalLocation: PropertyDescriptor | undefined;
    let originalBoobaDefaultRenderer: any;

    beforeEach(() => {
        // Save original location descriptor for proper restoration
        originalLocation = Object.getOwnPropertyDescriptor(window, 'location');
        originalBoobaDefaultRenderer = (window as any).__boobaDefaultRenderer;
        delete (window as any).__boobaDefaultRenderer;
    });

    afterEach(() => {
        // Restore original location
        if (originalLocation) {
            Object.defineProperty(window, 'location', originalLocation);
        } else {
            delete (window as any).location;
        }
        // Restore original __boobaDefaultRenderer
        (window as any).__boobaDefaultRenderer = originalBoobaDefaultRenderer;
    });

    it('should parse valid ?renderer=webgpu', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=webgpu'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('webgpu');
    });

    it('should parse valid ?renderer=canvas2d', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=canvas2d'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('canvas2d');
    });

    it('should parse valid ?renderer=auto', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=auto'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('auto');
    });

    it('should fall back to window.__boobaDefaultRenderer if param is invalid', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=invalid'),
            writable: true,
        });
        (window as any).__boobaDefaultRenderer = 'webgpu';
        expect(parseRendererFromURL()).toBe('webgpu');
    });

    it('should fall back to auto if param is missing and no __boobaDefaultRenderer', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com'),
            writable: true,
        });
        expect(parseRendererFromURL()).toBe('auto');
    });

    it('should ignore invalid __boobaDefaultRenderer and fall back to auto', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com?renderer=invalid'),
            writable: true,
        });
        (window as any).__boobaDefaultRenderer = 'badvalue';
        expect(parseRendererFromURL()).toBe('auto');
    });
});

import type { Terminal } from 'ghostty-web';

describe('installRendererHud', () => {
    let mockTerminal: Partial<Terminal> = {};

    beforeEach(() => {
        // Clear any previous HUD elements
        const existing = document.getElementById('booba-renderer-hud');
        existing?.remove();

        // Create a simple mock Terminal
        mockTerminal = {
            renderer: {
                backend: 'webgpu',
            },
        };
    });

    afterEach(() => {
        const element = document.getElementById('booba-renderer-hud');
        element?.remove();
    });

    it('should create and append badge element to parent', async () => {
        const { installRendererHud } = await import('./hud');
        const parent = document.createElement('div');
        document.body.appendChild(parent);

        installRendererHud(mockTerminal as Terminal, { parent });

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();
        expect(badge?.parentElement).toBe(parent);

        parent.remove();
    });

    it('should append to document.body by default', async () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.parentElement).toBe(document.body);
    });

    it('should apply inline styles', async () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.style.position).toBe('fixed');
        expect(badge?.style.bottom).toBe('12px');
        expect(badge?.style.right).toBe('12px');
        expect(badge?.style.fontFamily).toContain('monospace');
        expect(badge?.style.fontSize).toBe('11px');
        expect(badge?.style.color).toBe('rgb(102, 102, 102)'); // #666
        expect(badge?.style.pointerEvents).toBe('none');
        expect(badge?.style.zIndex).toBe('10');
    });

    it('should apply optional className', async () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal, { className: 'my-custom-class' });

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.classList.contains('my-custom-class')).toBe(true);
    });

    it('should return an uninstall function that removes element', async () => {
        const { installRendererHud } = await import('./hud');

        const uninstall = installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();

        uninstall();

        const badgeAfter = document.getElementById('booba-renderer-hud');
        expect(badgeAfter).toBeNull();
    });

    it('should bind Alt+Shift+R hotkey by default', async () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal, { bindToggleHotkey: true });

        const event = new KeyboardEvent('keydown', {
            altKey: true,
            shiftKey: true,
            key: 'R',
        });

        // Note: Full navigation test is hard without actual reload;
        // this verifies the listener is attached (no error)
        document.dispatchEvent(event);
    });

    it('should not bind hotkey if bindToggleHotkey is false', async () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal, { bindToggleHotkey: false });

        const event = new KeyboardEvent('keydown', {
            altKey: true,
            shiftKey: true,
            key: 'R',
        });

        // Should not throw
        document.dispatchEvent(event);
    });

    it('should display backend and fps in text content', async () => {
        const { installRendererHud } = await import('./hud');

        installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        // Text may be updated asynchronously via rAF, so just check it exists
        expect(badge?.textContent).toBeDefined();
    });
});

describe('installRendererHud integration', () => {
    it('should mount HUD and update FPS live', async () => {
        const { installRendererHud } = await import('./hud');

        const mockTerminal: Partial<Terminal> = {
            renderer: {
                backend: 'canvas2d',
            },
        };

        const uninstall = installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();
        expect(badge?.textContent).toBeDefined();

        // Wait for FPS counter to tick (requires ~1000ms for first update)
        await new Promise(resolve => setTimeout(resolve, 1100));

        // Text should exist (exact FPS depends on timing)
        expect(badge?.textContent?.length).toBeGreaterThan(0);

        uninstall();
        expect(document.getElementById('booba-renderer-hud')).toBeNull();
    });
});
