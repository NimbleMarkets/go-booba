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
    const parent = opts?.parent ?? document.body;
    const className = opts?.className;
    const bindToggleHotkey = opts?.bindToggleHotkey !== false;

    // Create badge element
    const badge = document.createElement('div');
    badge.id = 'booba-renderer-hud';

    // Apply inline styles (exact from reference)
    badge.style.position = 'fixed';
    badge.style.bottom = '12px';
    badge.style.right = '12px';
    badge.style.fontFamily = 'monospace';
    badge.style.fontSize = '11px';
    badge.style.color = 'rgb(102, 102, 102)';
    badge.style.background = 'rgba(0, 0, 0, 0.4)';
    badge.style.padding = '2px 6px';
    badge.style.borderRadius = '3px';
    badge.style.pointerEvents = 'none';
    badge.style.zIndex = '10';

    if (className) {
        badge.className = className;
    }

    parent.appendChild(badge);

    // rAF loop for FPS counter
    let frames = 0;
    let lastTick = performance.now();
    let rafId: number | null = null;

    const updateRendererInfo = () => {
        frames++;
        const now = performance.now();
        if (now - lastTick >= 1000) {
            const backend = terminal.renderer?.backend || '?';
            badge.textContent = `${backend} ${frames} fps`;
            frames = 0;
            lastTick = now;
        }
        rafId = requestAnimationFrame(updateRendererInfo);
    };

    rafId = requestAnimationFrame(updateRendererInfo);

    // Alt+Shift+R hotkey handler
    const handleKeyDown = (e: KeyboardEvent) => {
        if (!bindToggleHotkey) return;
        if (e.altKey && e.shiftKey && (e.key === 'R' || e.key === 'r')) {
            e.preventDefault();
            const cur = terminal.renderer?.backend;
            const next = cur === 'webgpu' ? 'canvas2d' : 'webgpu';
            const url = new URL(window.location.href);
            url.searchParams.set('renderer', next);
            window.location.href = url.toString();
        }
    };

    document.addEventListener('keydown', handleKeyDown);

    // Uninstall function
    return () => {
        if (rafId !== null) {
            cancelAnimationFrame(rafId);
        }
        badge.remove();
        document.removeEventListener('keydown', handleKeyDown);
    };
}
