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

    // Fall back to server's default renderer preference
    return (window as any).__boobaDefaultRenderer || 'auto';
}
