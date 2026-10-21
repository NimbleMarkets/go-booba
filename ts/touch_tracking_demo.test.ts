import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { installTouchTrackingDemo } from '../web/_assets/touch-tracking.js';

let frames: FrameRequestCallback[];
let sink: { boobaWrite(data: string): void };
let tracking: boolean;

beforeEach(() => {
    document.body.innerHTML = `
        <div id="terminal"><canvas></canvas><textarea></textarea></div>
        <button id="tracking"></button><button id="keyboard"></button><button id="reset"></button>
        <pre id="counts"></pre><pre id="log"></pre><p id="status"></p>`;
    frames = [];
    tracking = false;
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
        frames.push(callback);
        return frames.length;
    });
});

afterEach(() => vi.unstubAllGlobals());

async function setup() {
    const terminal = {
        term: { cols: 40, rows: 12, textarea: document.querySelector('textarea')! },
        blur: vi.fn(),
        hasMouseTracking: () => tracking,
        connectAdapter: (adapter: typeof sink & { connect(onData: (text: string) => void, onState: () => void): void }) => {
            sink = adapter;
            adapter.connect(text => {
                if (text.includes('\x1b[?1003h')) tracking = true;
                if (text.includes('\x1b[?1003l')) tracking = false;
            }, () => {});
        },
    };
    await installTouchTrackingDemo(terminal);
    flush();
    return terminal;
}

function flush() {
    for (const callback of frames.splice(0)) callback(0);
}

it('recognizes modified trackpad wheel input while preserving the raw SGR log', async () => {
    await setup();
    sink.boobaWrite('\x1b[<80;12;4M\x1b[<81;12;4M\x1b[<68;12;4M');
    flush();
    expect(document.getElementById('counts')!.textContent).toContain('wheel up 2 | down 1 | zoom 1');
    expect(document.getElementById('log')!.textContent).toContain('ESC[<80;12;4M  up [Ctrl]');
    expect(document.getElementById('log')!.textContent).toContain('ESC[<81;12;4M  down [Ctrl]');
    expect(document.getElementById('log')!.textContent).toContain('ESC[<68;12;4M  up [Shift]');
});

it('controls the demo keyboard independently from mouse tracking', async () => {
    const terminal = await setup();
    const textarea = terminal.term.textarea;
    expect(textarea.readOnly).toBe(true);
    expect(textarea.inputMode).toBe('none');
    expect(tracking).toBe(true);
    document.getElementById('keyboard')!.click();
    expect(textarea.readOnly).toBe(false);
    expect(document.activeElement).toBe(textarea);
    expect(tracking).toBe(true);
    document.getElementById('keyboard')!.click();
    expect(textarea.readOnly).toBe(true);
    expect(tracking).toBe(true);
    document.getElementById('tracking')!.click();
    expect(tracking).toBe(false);
});
