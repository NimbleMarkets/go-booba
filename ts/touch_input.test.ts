import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TouchInput } from './touch_input.js';

const touch = (clientX: number, clientY: number, identifier = 1) => ({ clientX, clientY, identifier }) as Touch;
const sgr = (button: number, x: number, y: number, end = 'M') => `\x1b[<${button};${x};${y}${end}`;

describe('TouchInput', () => {
    let host: HTMLDivElement;
    let canvas: HTMLCanvasElement;
    let input: TouchInput;
    let tracking: boolean;
    let sent: string[];
    let frames: Map<number, FrameRequestCallback>;
    let nextFrame: number;

    function event(type: string, touches: Touch[], changedTouches = touches) {
        const ev = new Event(type, { bubbles: true, cancelable: true });
        Object.assign(ev, { touches, changedTouches });
        canvas.dispatchEvent(ev);
        return ev;
    }
    function frame() {
        const callbacks = [...frames.values()]; frames.clear();
        callbacks.forEach(callback => callback(0));
    }
    function setRect(target: HTMLCanvasElement) {
        target.width = 1600; target.height = 960; // 80x24, 10x20 CSS cells, DPR 2.
        vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({ left: 100, top: 50, width: 800, height: 480 } as DOMRect);
    }

    beforeEach(() => {
        vi.useFakeTimers();
        vi.stubGlobal('devicePixelRatio', 2);
        frames = new Map(); nextFrame = 0;
        vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { frames.set(++nextFrame, callback); return nextFrame; });
        vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id));
        host = document.createElement('div'); canvas = document.createElement('canvas');
        host.append(canvas); document.body.append(host); setRect(canvas);
        tracking = true; sent = [];
        input = new TouchInput(host, {
            hasMouseTracking: () => tracking,
            getCellSize: () => ({ width: 10, height: 20 }),
            getSize: () => ({ cols: 80, rows: 24 }),
            sendInput: data => sent.push(data),
        });
    });
    afterEach(() => { input.dispose(); host.remove(); vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.useRealTimers(); });

    it('maps CSS coordinates with canvas offset at DPR 2, uses one-based cells and clamps to the grid', () => {
        event('touchstart', [touch(125, 95)]);
        event('touchend', [], [touch(125, 95)]);
        event('touchstart', [touch(-100, -100)]);
        event('touchend', [], [touch(9999, 9999)]);
        expect(sent).toEqual([sgr(0, 3, 3), sgr(0, 3, 3, 'm'), sgr(0, 1, 1), sgr(0, 80, 24, 'm')]);
    });

    it('emits press, held-button motion and release, coalescing moves per frame and skipping repeated cells', () => {
        event('touchstart', [touch(105, 55)]);
        expect(event('touchmove', [touch(115, 75)]).defaultPrevented).toBe(true);
        event('touchmove', [touch(135, 95)]);
        expect(sent).toEqual([sgr(0, 1, 1)]);
        frame();
        event('touchmove', [touch(139, 99)]); frame();
        event('touchmove', [touch(145, 115)]); frame();
        event('touchend', [], [touch(145, 115)]);
        expect(sent).toEqual([sgr(0, 1, 1), sgr(32, 4, 3), sgr(32, 5, 4), sgr(0, 5, 4, 'm')]);
    });

    it('a quick tap sends exactly one click pair and preserves the canvas touchend focus behavior', () => {
        const focus = vi.fn(); canvas.addEventListener('touchend', focus);
        event('touchstart', [touch(125, 95)]);
        vi.advanceTimersByTime(150);
        event('touchend', [], [touch(125, 95)]);
        expect(sent).toEqual([sgr(0, 3, 3), sgr(0, 3, 3, 'm')]);
        expect(focus).toHaveBeenCalledOnce();
    });

    it('pinches emit wheel up and down at the current midpoint, accumulating sub-threshold movement', () => {
        event('touchstart', [touch(200, 150), touch(300, 150, 2)]);
        event('touchmove', [touch(190, 150), touch(310, 150, 2)]);
        expect(sent).toEqual([]);
        event('touchmove', [touch(180, 150), touch(320, 150, 2)]);
        event('touchmove', [touch(200, 150), touch(300, 150, 2)]);
        event('touchend', []);
        expect(sent).toEqual([sgr(64, 16, 6), sgr(65, 16, 6)]);
    });

    it('two fingers scroll vertically without changing distance', () => {
        event('touchstart', [touch(200, 150), touch(300, 150, 2)]);
        event('touchmove', [touch(200, 110), touch(300, 110, 2)]);
        event('touchmove', [touch(200, 150), touch(300, 150, 2)]);
        expect(sent).toEqual([sgr(65, 16, 4), sgr(64, 16, 6)]);
    });

    it('releases a drag before pinching and waits for both fingers to lift before another drag', () => {
        event('touchstart', [touch(200, 150)]);
        event('touchmove', [touch(210, 150)]); // Pending motion must not arrive during a pinch.
        event('touchstart', [touch(210, 150), touch(310, 150, 2)], [touch(310, 150, 2)]);
        frame();
        event('touchmove', [touch(190, 150), touch(330, 150, 2)]);
        event('touchend', [touch(190, 150)], [touch(330, 150, 2)]);
        event('touchmove', [touch(180, 150)]); frame();
        event('touchend', [], [touch(180, 150)]);
        event('touchstart', [touch(200, 150)]);
        expect(sent).toEqual([sgr(0, 11, 6), sgr(0, 12, 6, 'm'), sgr(64, 17, 6), sgr(0, 11, 6)]);
    });

    it('cancels pending frames on touchcancel and releases at the last touch position', () => {
        event('touchstart', [touch(200, 150)]);
        event('touchmove', [touch(210, 170)]);
        expect(event('touchcancel', [], [touch(210, 170)]).defaultPrevented).toBe(true);
        frame();
        expect(sent).toEqual([sgr(0, 11, 6), sgr(0, 12, 7, 'm')]);
    });

    it('leaves touches and mouse events alone without tracking, and rechecks on each gesture', () => {
        tracking = false;
        expect(event('touchstart', [touch(200, 150)]).defaultPrevented).toBe(false);
        expect(event('touchmove', [touch(210, 150)]).defaultPrevented).toBe(false);
        expect(event('touchend', [], [touch(210, 150)]).defaultPrevented).toBe(false);
        const mouse = new MouseEvent('mousedown', { bubbles: true, cancelable: true }); canvas.dispatchEvent(mouse);
        expect(mouse.defaultPrevented).toBe(false);
        expect(sent).toEqual([]);
        tracking = true;
        event('touchstart', [touch(200, 150)]);
        event('touchmove', [touch(210, 150)]);
        tracking = false;
        frame();
        expect(event('touchmove', [touch(220, 150)]).defaultPrevented).toBe(false);
        event('touchend', []);
        expect(sent).toEqual([sgr(0, 11, 6)]);
    });

    it('suppresses compatibility mouse events before ghostty capture handlers, with a bounded Safari fallback', () => {
        const mouse = vi.fn(); host.addEventListener('mousedown', mouse, true); canvas.addEventListener('click', mouse);
        event('touchstart', [touch(200, 150)]); event('touchend', [], [touch(200, 150)]);
        for (const type of ['mousedown', 'mousemove', 'mouseup', 'click', 'dblclick', 'contextmenu']) {
            const ev = new MouseEvent(type, { bubbles: true, cancelable: true });
            canvas.dispatchEvent(ev); expect(ev.defaultPrevented).toBe(true);
        }
        expect(mouse).not.toHaveBeenCalled();
        const button = document.createElement('button'); host.append(button);
        const buttonClick = new MouseEvent('click', { bubbles: true, cancelable: true });
        button.dispatchEvent(buttonClick); expect(buttonClick.defaultPrevented).toBe(false);
        const realMouse = new MouseEvent('mousedown', { bubbles: true, cancelable: true });
        Object.assign(realMouse, { sourceCapabilities: { firesTouchEvents: false } });
        canvas.dispatchEvent(realMouse); expect(realMouse.defaultPrevented).toBe(false);
        vi.advanceTimersByTime(801);
        canvas.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        expect(mouse).toHaveBeenCalledTimes(2);
    });

    it('handles a replacement canvas, ignores other controls and cleans up listeners and frames', () => {
        const button = document.createElement('button'); host.append(button);
        const ev = new Event('touchstart', { bubbles: true, cancelable: true });
        Object.assign(ev, { touches: [touch(200, 150)] }); button.dispatchEvent(ev);
        expect(ev.defaultPrevented).toBe(false);
        const fresh = document.createElement('canvas'); setRect(fresh); canvas.replaceWith(fresh); canvas = fresh;
        event('touchstart', [touch(200, 150)]);
        event('touchmove', [touch(210, 150)]);
        input.dispose(); frame();
        expect(sent).toEqual([sgr(0, 11, 6), sgr(0, 12, 6, 'm')]);
        expect(event('touchstart', [touch(200, 150)]).defaultPrevented).toBe(false);
        const click = new MouseEvent('click', { bubbles: true, cancelable: true }); canvas.dispatchEvent(click);
        expect(click.defaultPrevented).toBe(false);
    });
});
