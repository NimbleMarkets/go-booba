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
