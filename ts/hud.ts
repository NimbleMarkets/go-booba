import type { Terminal } from 'ghostty-web';

/**
 * Parse ?renderer= query parameter from the current URL.
 * Falls back to window.__boobaDefaultRenderer, then 'auto'.
 * Only accepts 'auto', 'webgpu', 'canvas2d'.
 */
export function parseRendererFromURL(): 'auto' | 'webgpu' | 'canvas2d' {
    const params = new URLSearchParams(window.location.search);
    const renderer = params.get('renderer');

    if (renderer === 'canvas2d' || renderer === 'webgpu' || renderer === 'auto') {
        return renderer;
    }

    // Validate server-injected default before using it
    const fallback = (window as any).__boobaDefaultRenderer;
    if (fallback === 'canvas2d' || fallback === 'webgpu' || fallback === 'auto') {
        return fallback;
    }

    return 'auto';
}

export interface BoobaRendererHudOptions {
    /** Where to mount the badge (default: document.body) */
    parent?: HTMLElement;
    /** CSS class for custom styling (applied to badge element) */
    className?: string;
    /** Bind Alt+Shift+R to toggle webgpu↔canvas2d. Default: true. */
    bindToggleHotkey?: boolean;
}

/**
 * Install a corner HUD showing active renderer backend and live FPS.
 * Must be called after booba.init() so terminal.renderer exists.
 * Returns an uninstall function.
 */
export function installRendererHud(
    terminal: Terminal,
    opts?: BoobaRendererHudOptions
): () => void {
    // Implemented in Task 6
    throw new Error('Not implemented');
}
