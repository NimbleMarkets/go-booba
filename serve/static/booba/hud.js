/**
 * Parse ?renderer= query parameter from the current URL.
 * Falls back to window.__boobaDefaultRenderer, then 'auto'.
 * Only accepts 'auto', 'webgpu', 'canvas2d'.
 */
export function parseRendererFromURL() {
    const params = new URLSearchParams(window.location.search);
    const renderer = params.get('renderer');
    if (renderer === 'canvas2d' || renderer === 'webgpu' || renderer === 'auto') {
        return renderer;
    }
    // Validate server-injected default before using it
    const fallback = window.__boobaDefaultRenderer;
    if (fallback === 'canvas2d' || fallback === 'webgpu' || fallback === 'auto') {
        return fallback;
    }
    return 'auto';
}
/**
 * Install a corner HUD showing active renderer backend and live FPS.
 * Must be called after booba.init() so terminal.renderer exists.
 * Returns an uninstall function.
 */
export function installRendererHud(terminal, opts) {
    // Guard against double-install
    const existing = document.getElementById('booba-renderer-hud');
    if (existing) {
        console.warn('installRendererHud: HUD already installed. Uninstall first if you need to reinstall.');
        return () => { }; // Return no-op uninstall
    }
    if (!terminal) {
        throw new Error('installRendererHud: terminal is null — make sure to call after booba.init()');
    }
    const parent = opts?.parent ?? document.body;
    const className = opts?.className;
    const position = opts?.position ?? 'fixed';
    // Create badge element
    const badge = document.createElement('div');
    badge.id = 'booba-renderer-hud';
    // Inject default styles once (idempotent, position excluded)
    if (!document.getElementById('booba-renderer-hud-styles')) {
        const style = document.createElement('style');
        style.id = 'booba-renderer-hud-styles';
        style.textContent = `
            :where(#booba-renderer-hud) {
                bottom: 12px;
                right: 12px;
                font-family: monospace;
                font-size: 11px;
                color: #666;
                background: rgba(0, 0, 0, 0.4);
                padding: 2px 6px;
                border-radius: 3px;
                pointer-events: auto;
                cursor: pointer;
                user-select: none;
                z-index: 10;
            }
        `;
        document.head.appendChild(style);
    }
    // Position is instance-specific: set as inline style
    badge.style.position = position;
    // Apply optional className (now wins via CSS cascade for other properties)
    if (className) {
        badge.className = className;
    }
    // Set title for discoverability
    badge.title = 'Click to toggle renderer';
    parent.appendChild(badge);
    // rAF loop for FPS counter
    let frames = 0;
    let lastTick = performance.now();
    let rafId = null;
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
    // Click handler to toggle renderer
    const handleClick = () => {
        const cur = terminal.renderer?.backend;
        const next = cur === 'webgpu' ? 'canvas2d' : 'webgpu';
        const url = new URL(window.location.href);
        url.searchParams.set('renderer', next);
        window.location.href = url.toString();
    };
    badge.addEventListener('click', handleClick);
    // Uninstall function
    return () => {
        if (rafId !== null) {
            cancelAnimationFrame(rafId);
        }
        badge.remove();
        badge.removeEventListener('click', handleClick);
    };
}
//# sourceMappingURL=hud.js.map