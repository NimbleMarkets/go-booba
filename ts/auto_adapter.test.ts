import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const ws = vi.hoisted(() => ({ connect: vi.fn(), disconnect: vi.fn() }));
vi.mock('./websocket_adapter.js', () => ({
    BoobaProtocolAdapter: class {
        connect = ws.connect;
        disconnect = ws.disconnect;
    },
}));
import { BoobaAutoAdapter } from './auto_adapter.js';

function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>(r => { resolve = r; });
    return { promise, resolve };
}

describe('automatic WebTransport fallback', () => {
    let adapter: BoobaAutoAdapter;
    let ready: ReturnType<typeof deferred<void>>;
    let transport: {
        ready: Promise<void>;
        closed: Promise<void>;
        createBidirectionalStream: ReturnType<typeof vi.fn>;
        close: ReturnType<typeof vi.fn>;
    };
    let state: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        vi.useFakeTimers();
        vi.clearAllMocks();
        ready = deferred<void>();
        transport = {
            ready: ready.promise,
            closed: new Promise(() => {}),
            createBidirectionalStream: vi.fn(),
            close: vi.fn(),
        };
        vi.stubGlobal('WebTransport', vi.fn(function () { return transport; }));
        vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: async () => ({ hash: 'ab'.repeat(32) }) })));
        adapter = new BoobaAutoAdapter('ws://localhost/ws', 'https://localhost/wt', '/cert-hash');
        state = vi.fn();
    });

    afterEach(() => {
        adapter.disconnect();
        vi.useRealTimers();
        vi.unstubAllGlobals();
    });

    async function start() {
        adapter.connect(vi.fn(), state);
        await vi.advanceTimersByTimeAsync(0);
    }

    it('closes a stalled handshake and falls back to WebSocket after three seconds', async () => {
        await start();
        await vi.advanceTimersByTimeAsync(2999);
        expect(ws.connect).not.toHaveBeenCalled();
        await vi.advanceTimersByTimeAsync(1);
        expect(transport.close).toHaveBeenCalledOnce();
        expect(ws.connect).toHaveBeenCalledOnce();
        expect(state).not.toHaveBeenCalledWith('disconnected', expect.anything());
        ready.resolve();
        await vi.advanceTimersByTimeAsync(0);
        expect(transport.createBidirectionalStream).not.toHaveBeenCalled();
        expect(state).not.toHaveBeenCalledWith('connected', expect.anything());
    });

    it('also times out stalled stream creation and discards a late stream', async () => {
        const opening = deferred<any>();
        transport.createBidirectionalStream.mockReturnValue(opening.promise);
        ready.resolve();
        await start();
        await vi.advanceTimersByTimeAsync(3000);
        expect(ws.connect).toHaveBeenCalledOnce();
        const stream = {
            readable: { cancel: vi.fn(async () => {}) },
            writable: { abort: vi.fn(async () => {}), getWriter: vi.fn() },
        };
        opening.resolve(stream);
        await vi.advanceTimersByTimeAsync(0);
        expect(stream.readable.cancel).toHaveBeenCalledOnce();
        expect(stream.writable.abort).toHaveBeenCalledOnce();
        expect(stream.writable.getWriter).not.toHaveBeenCalled();
    });

    it('does not start a fallback after disconnect', async () => {
        await start();
        adapter.disconnect();
        await vi.advanceTimersByTimeAsync(3000);
        expect(ws.connect).not.toHaveBeenCalled();
        expect(state).toHaveBeenCalledTimes(1);
    });

    it('keeps a successful WebTransport connection without falling back', async () => {
        transport.createBidirectionalStream.mockResolvedValue({
            readable: new ReadableStream({ start(controller) { controller.close(); } }),
            writable: new WritableStream(),
        });
        ready.resolve();
        await start();
        expect(state).toHaveBeenCalledWith('connected', 'Connected (WebTransport)');
        await vi.advanceTimersByTimeAsync(3000);
        expect(ws.connect).not.toHaveBeenCalled();
        expect(transport.close).not.toHaveBeenCalled();
    });

    it('falls back immediately when stream setup rejects', async () => {
        ready.resolve();
        transport.createBidirectionalStream.mockRejectedValue(new Error('unsupported'));
        await start();
        expect(transport.close).toHaveBeenCalledOnce();
        expect(ws.connect).toHaveBeenCalledOnce();
    });
});
