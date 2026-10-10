import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { BoobaWasmAdapter } from './adapter.js';

// The WASM bridge is three functions the Go program registers on window.
// Once the Go program returns, the runtime is gone: calling any of them
// throws "Go program has already exited". booba's wasm package announces
// the exit first by setting window.bubbletea_exited and handing the unread
// output over as window.bubbletea_final.
const w = window as any;
const BRIDGE = ['bubbletea_read', 'bubbletea_write', 'bubbletea_resize', 'bubbletea_exited', 'bubbletea_final'];

function installBridge(read: () => string): void {
    w.bubbletea_read = read;
    w.bubbletea_write = vi.fn();
    w.bubbletea_resize = vi.fn();
}

describe('BoobaWasmAdapter program exit', () => {
    beforeEach(() => { vi.useFakeTimers(); });
    afterEach(() => { vi.useRealTimers(); for (const k of BRIDGE) delete w[k]; });

    it('delivers the final output once, reports disconnected, and stops polling', () => {
        const read = vi.fn().mockReturnValueOnce('frame').mockReturnValue('');
        installBridge(read);
        const adapter = new BoobaWasmAdapter(10);
        const onData = vi.fn();
        const onState = vi.fn();
        adapter.connect(onData, onState);
        vi.advanceTimersByTime(10);
        expect(onData).toHaveBeenCalledWith('frame');

        w.bubbletea_final = '\r\nSee you later!\r\n';
        w.bubbletea_exited = true;
        vi.advanceTimersByTime(10);
        expect(onData).toHaveBeenLastCalledWith('\r\nSee you later!\r\n');
        expect(onState).toHaveBeenLastCalledWith('disconnected', 'Program exited');

        const readsSoFar = read.mock.calls.length;
        const dataSoFar = onData.mock.calls.length;
        vi.advanceTimersByTime(200);
        expect(read.mock.calls.length).toBe(readsSoFar);
        expect(onData.mock.calls.length).toBe(dataSoFar);
        expect(onState.mock.calls.filter(c => c[0] === 'disconnected')).toHaveLength(1);
    });

    it('treats a bridge call that throws as the program having exited', () => {
        const read = vi.fn(() => { throw new Error('Go program has already exited'); });
        installBridge(read);
        const adapter = new BoobaWasmAdapter(10);
        const onState = vi.fn();
        adapter.connect(vi.fn(), onState);
        expect(() => vi.advanceTimersByTime(10)).not.toThrow();
        expect(onState).toHaveBeenLastCalledWith('disconnected', 'Program exited');
        const readsSoFar = read.mock.calls.length;
        vi.advanceTimersByTime(200);
        expect(read.mock.calls.length).toBe(readsSoFar);
    });

    it('ignores writes and resizes after the program has exited', () => {
        installBridge(vi.fn().mockReturnValue(''));
        const adapter = new BoobaWasmAdapter(10);
        adapter.connect(vi.fn(), vi.fn());
        w.bubbletea_exited = true;
        vi.advanceTimersByTime(10);
        adapter.boobaWrite('q');
        adapter.boobaResize(80, 24);
        expect(w.bubbletea_write).not.toHaveBeenCalled();
        expect(w.bubbletea_resize).not.toHaveBeenCalled();
    });
});
