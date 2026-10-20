const WHEEL_THRESHOLD = 40; // CSS pixels, for both pinch distance and vertical scroll.
const COMPAT_MOUSE_TIMEOUT = 800;
/** Translate tracked canvas touches into SGR input, without changing native mouse input. */
export class TouchInput {
    constructor(element, options) {
        this.element = element;
        this.options = options;
        this.canvas = null;
        this.finger = null;
        this.lastCell = null;
        this.pendingCell = null;
        this.frame = null;
        this.pair = null;
        this.active = false;
        this.suppressMouseUntil = 0;
        this.mouseEvents = ['mousedown', 'mousemove', 'mouseup', 'click', 'dblclick', 'contextmenu'];
        this.start = (event) => {
            if (!this.options.hasMouseTracking()) {
                this.clear();
                return;
            }
            if (!this.active) {
                const canvas = this.element.querySelector('canvas');
                if (!canvas || event.target !== canvas)
                    return;
                this.canvas = canvas;
                if (!event.touches.length || !this.cell(event.touches[0]))
                    return;
                this.active = true;
            }
            this.prevent(event);
            if (event.touches.length === 1 && this.finger === null && !this.pair) {
                const touch = event.touches[0];
                this.finger = touch.identifier;
                this.lastCell = this.cell(touch);
                // Press immediately so drag consumers respond without a tap delay. A short
                // stationary touch already yields exactly one press/release click pair.
                if (this.lastCell)
                    this.send(0, this.lastCell);
            }
            else if (event.touches.length >= 2) {
                this.release();
                if (event.touches.length === 2) {
                    const [a, b] = Array.from(event.touches);
                    this.pair = { ids: [a.identifier, b.identifier], distance: this.distance(a, b), y: (a.clientY + b.clientY) / 2 };
                }
                else
                    this.pair = null;
            }
        };
        this.move = (event) => {
            if (!this.options.hasMouseTracking()) {
                this.clear();
                return;
            }
            if (!this.active)
                return;
            this.prevent(event);
            if (this.pair && event.touches.length === 2) {
                const touches = Array.from(event.touches);
                const a = touches.find(t => t.identifier === this.pair.ids[0]);
                const b = touches.find(t => t.identifier === this.pair.ids[1]);
                if (!a || !b)
                    return;
                const distance = this.distance(a, b);
                const y = (a.clientY + b.clientY) / 2;
                const cell = this.cell({ clientX: (a.clientX + b.clientX) / 2, clientY: y });
                if (!cell)
                    return;
                const pinch = distance - this.pair.distance;
                const scroll = y - this.pair.y;
                const delta = Math.abs(pinch) >= WHEEL_THRESHOLD ? pinch : scroll;
                if (Math.abs(delta) >= WHEEL_THRESHOLD) {
                    const steps = Math.floor(Math.abs(delta) / WHEEL_THRESHOLD);
                    for (let i = 0; i < steps; i++)
                        this.send(delta > 0 ? 64 : 65, cell);
                    if (Math.abs(pinch) >= WHEEL_THRESHOLD)
                        this.pair.distance += Math.sign(delta) * steps * WHEEL_THRESHOLD;
                    else
                        this.pair.distance = distance;
                    this.pair.y = y;
                }
            }
            else if (this.finger !== null && event.touches.length === 1) {
                const touch = Array.from(event.touches).find(t => t.identifier === this.finger);
                if (!touch)
                    return;
                this.pendingCell = this.cell(touch);
                if (this.frame === null)
                    this.frame = requestAnimationFrame(() => {
                        this.frame = null;
                        const cell = this.pendingCell;
                        this.pendingCell = null;
                        if (!this.options.hasMouseTracking()) {
                            this.clear();
                            return;
                        }
                        if (cell && (cell.x !== this.lastCell?.x || cell.y !== this.lastCell?.y)) {
                            this.send(32, cell);
                            this.lastCell = cell;
                        }
                    });
            }
        };
        this.end = (event) => {
            if (!this.options.hasMouseTracking()) {
                this.clear();
                return;
            }
            if (!this.active)
                return;
            this.prevent(event);
            if (this.finger !== null) {
                const touch = Array.from(event.changedTouches).find(t => t.identifier === this.finger);
                if (touch || event.type === 'touchcancel')
                    this.release(touch ? this.cell(touch) : undefined);
            }
            // After a pinch, wait for all fingers to lift rather than starting a stray drag.
            if (event.touches.length < 2)
                this.pair = null;
            if (!event.touches.length || event.type === 'touchcancel')
                this.clear();
        };
        this.mouse = (event) => {
            if (event.target !== this.element.querySelector('canvas'))
                return;
            const source = event.sourceCapabilities;
            // Chromium identifies touch compatibility events; Safari needs the short time window.
            if (source?.firesTouchEvents === false || Date.now() >= this.suppressMouseUntil)
                return;
            event.preventDefault();
            event.stopImmediatePropagation();
        };
        // Delegate to the terminal's primary canvas: ghostty replaces it on renderer swaps.
        // Capture does not stop touch propagation, preserving its touchend focus listener.
        element.addEventListener('touchstart', this.start, { passive: false, capture: true });
        element.addEventListener('touchmove', this.move, { passive: false, capture: true });
        element.addEventListener('touchend', this.end, { passive: false, capture: true });
        element.addEventListener('touchcancel', this.end, { passive: false, capture: true });
        // Run before ghostty's container capture handlers as well as its canvas handlers.
        for (const type of this.mouseEvents)
            element.ownerDocument.addEventListener(type, this.mouse, true);
    }
    cell(point) {
        const metrics = this.options.getCellSize();
        const { cols, rows } = this.options.getSize();
        if (!this.canvas || !metrics || metrics.width <= 0 || metrics.height <= 0 || cols <= 0 || rows <= 0)
            return null;
        const rect = this.canvas.getBoundingClientRect();
        return {
            x: Math.max(1, Math.min(cols, Math.floor((point.clientX - rect.left) / metrics.width) + 1)),
            y: Math.max(1, Math.min(rows, Math.floor((point.clientY - rect.top) / metrics.height) + 1)),
        };
    }
    send(button, cell, release = false) {
        // Future modifiers are ORed into button here: Shift = 4, Alt = 8, Ctrl = 16.
        this.options.sendInput(`\x1b[<${button};${cell.x};${cell.y}${release ? 'm' : 'M'}`);
    }
    prevent(event) {
        event.preventDefault();
        this.suppressMouseUntil = Date.now() + COMPAT_MOUSE_TIMEOUT;
    }
    distance(a, b) {
        return Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);
    }
    release(cell = this.pendingCell ?? this.lastCell) {
        this.cancelFrame();
        if (this.finger !== null && cell)
            this.send(0, cell, true);
        this.finger = null;
        this.lastCell = null;
    }
    cancelFrame() {
        if (this.frame !== null)
            cancelAnimationFrame(this.frame);
        this.frame = null;
        this.pendingCell = null;
    }
    clear() {
        this.cancelFrame();
        this.active = false;
        this.finger = null;
        this.lastCell = null;
        this.pair = null;
        this.canvas = null;
    }
    dispose() {
        if (this.finger !== null && this.options.hasMouseTracking())
            this.release();
        this.clear();
        this.element.removeEventListener('touchstart', this.start, true);
        this.element.removeEventListener('touchmove', this.move, true);
        this.element.removeEventListener('touchend', this.end, true);
        this.element.removeEventListener('touchcancel', this.end, true);
        for (const type of this.mouseEvents)
            this.element.ownerDocument.removeEventListener(type, this.mouse, true);
    }
}
