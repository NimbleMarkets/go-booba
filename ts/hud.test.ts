import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { parseRendererFromURL } from './hud';

describe('parseRendererFromURL', () => {
    let originalBoobaDefaultRenderer: any;

    beforeEach(() => {
        // Save original state
        originalBoobaDefaultRenderer = (window as any).__boobaDefaultRenderer;
        // Reset global state
        delete (window as any).__boobaDefaultRenderer;
    });

    afterEach(() => {
        // Restore
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

    it('should fall back to auto if both param and __boobaDefaultRenderer are missing', () => {
        Object.defineProperty(window, 'location', {
            value: new URL('https://example.com'),
            writable: true,
        });
        delete (window as any).__boobaDefaultRenderer;
        expect(parseRendererFromURL()).toBe('auto');
    });
});
