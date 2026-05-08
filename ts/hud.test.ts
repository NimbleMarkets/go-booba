import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { parseRendererFromURL, installRendererHud } from './hud';

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

        // Clear any previous HUD style element
        const styleElement = document.getElementById('booba-renderer-hud-styles');
        styleElement?.remove();
    });

    it('should create and append badge element to parent', () => {
        const parent = document.createElement('div');
        document.body.appendChild(parent);

        const uninstall = installRendererHud(mockTerminal as Terminal, { parent });

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();
        expect(badge?.parentElement).toBe(parent);

        uninstall();
        parent.remove();
    });

    it('should append to document.body by default', () => {
        const uninstall = installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.parentElement).toBe(document.body);

        uninstall();
    });

    it('should apply inline styles', () => {
        const uninstall = installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        const computed = window.getComputedStyle(badge!);
        expect(computed.position).toBe('fixed');
        expect(computed.bottom).toBe('12px');
        expect(computed.right).toBe('12px');
        expect(computed.fontFamily).toContain('monospace');
        expect(computed.fontSize).toBe('11px');
        expect(computed.pointerEvents).toBe('none');
        expect(computed.zIndex).toBe('10');

        uninstall();
    });

    it('should apply optional className', () => {
        const uninstall = installRendererHud(mockTerminal as Terminal, { className: 'my-custom-class' });

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.classList.contains('my-custom-class')).toBe(true);

        uninstall();
    });

    it('should return an uninstall function that removes element', () => {
        const uninstall = installRendererHud(mockTerminal as Terminal);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge).toBeDefined();

        uninstall();

        const badgeAfter = document.getElementById('booba-renderer-hud');
        expect(badgeAfter).toBeNull();
    });

    it('should bind Alt+Shift+R hotkey and toggle renderer', () => {
        const originalHref = window.location.href;

        const uninstall = installRendererHud(mockTerminal as Terminal, { bindToggleHotkey: true });

        // Mock window.location.href setter
        let navigatedUrl = '';
        const originalLocation = Object.getOwnPropertyDescriptor(window, 'location');
        Object.defineProperty(window, 'location', {
            value: {
                href: originalHref,
            },
            writable: true,
        });

        // Track navigation
        Object.defineProperty(window.location, 'href', {
            set: (url: string) => {
                navigatedUrl = url;
            },
            get: () => originalHref,
        });

        // Dispatch Alt+Shift+R
        const event = new KeyboardEvent('keydown', {
            altKey: true,
            shiftKey: true,
            key: 'R',
        });

        document.dispatchEvent(event);

        // Assert URL changed to toggle renderer
        expect(navigatedUrl).toContain('renderer=canvas2d'); // webgpu toggles to canvas2d

        // Cleanup
        uninstall();

        // Restore location
        if (originalLocation) {
            Object.defineProperty(window, 'location', originalLocation);
        }
    });

    it('should not bind hotkey if bindToggleHotkey is false', () => {
        const uninstall = installRendererHud(mockTerminal as Terminal, { bindToggleHotkey: false });

        // Mock window.location.href to track if it changes
        const originalLocation = Object.getOwnPropertyDescriptor(window, 'location');
        let navigationAttempted = false;

        // Create a fresh location mock to capture any navigation attempts
        const mockLocation = {
            href: window.location.href,
            search: '',
        };

        const hrefDescriptor = {
            set: () => {
                navigationAttempted = true;
            },
            get: () => window.location.href,
            configurable: true,
        };

        Object.defineProperty(window, 'location', {
            configurable: true,
            value: mockLocation,
        });

        Object.defineProperty(window.location, 'href', hrefDescriptor);

        // Dispatch Alt+Shift+R
        const event = new KeyboardEvent('keydown', {
            altKey: true,
            shiftKey: true,
            key: 'R',
        });

        document.dispatchEvent(event);

        // When bindToggleHotkey is false, the handler should return early
        // and not attempt navigation
        expect(navigationAttempted).toBe(false);

        // Cleanup
        uninstall();

        // Restore location
        if (originalLocation) {
            Object.defineProperty(window, 'location', originalLocation);
        }
    });

    it('should display backend and fps in text content', () => {
        vi.useFakeTimers();
        const mockNow = vi.fn(() => 0);
        vi.stubGlobal('performance', { now: mockNow });

        const uninstall = installRendererHud(mockTerminal as Terminal);

        // Advance 1100ms to trigger one FPS update
        mockNow.mockReturnValue(1100);
        vi.advanceTimersByTime(1100);

        const badge = document.getElementById('booba-renderer-hud');
        expect(badge?.textContent).toMatch(/^(webgpu|canvas2d|\?) \d+ fps$/);

        uninstall();
        vi.useRealTimers();
    });
});

describe('installRendererHud integration', () => {
    it('should mount HUD and update FPS live', async () => {
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

        // Text should match the format
        expect(badge?.textContent).toMatch(/^(canvas2d) \d+ fps$/);

        uninstall();
        expect(document.getElementById('booba-renderer-hud')).toBeNull();
    });
});
