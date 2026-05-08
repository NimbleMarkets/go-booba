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
