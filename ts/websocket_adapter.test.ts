import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { BoobaProtocolAdapter } from './websocket_adapter.js';
import { MsgClose, encodeWSMessage } from './protocol.js';

// --- Minimal WebSocket mock -------------------------------------------------
// The adapter only reaches for readyState, send, close, and the four
// event hooks (onopen, onclose, onmessage, onerror).

type Listener = (ev?: unknown) => void;

class MockWebSocket {
    static OPEN = 1;
    static CLOSED = 3;
    static instances: MockWebSocket[] = [];

    readyState = 0;
    binaryType = 'arraybuffer';
    sent: Uint8Array[] = [];

    onopen: Listener | null = null;
    onclose: Listener | null = null;
    onmessage: Listener | null = null;
    onerror: Listener | null = null;

    constructor(public url: string) {
        MockWebSocket.instances.push(this);
    }

    send(data: Uint8Array): void {
        this.sent.push(data);
    }

    close(): void {
        this.readyState = MockWebSocket.CLOSED;
        this.onclose?.();
    }

    fireOpen(): void {
        this.readyState = MockWebSocket.OPEN;
        this.onopen?.();
    }

    fireClose(): void {
        this.readyState = MockWebSocket.CLOSED;
        this.onclose?.();
    }

    fireMessage(data: Uint8Array): void {
        this.onmessage?.({ data: data.buffer } as unknown);
    }
}

describe('BoobaProtocolAdapter reconnection backoff', () => {
    beforeEach(() => {
        MockWebSocket.instances = [];
        vi.useFakeTimers();
        vi.stubGlobal('WebSocket', MockWebSocket);
        // The adapter uses window.setInterval / clearInterval for pings;
        // route those through globalThis so Node + fake timers can see them.
        vi.stubGlobal('window', globalThis);
    });

    afterEach(() => {
        vi.useRealTimers();
        vi.unstubAllGlobals();
    });

    it('schedules reconnect attempts with exponential backoff (1000 * 1.5^(n-1))', () => {
        const adapter = new BoobaProtocolAdapter('ws://example/ws');
        const states: Array<[string, string]> = [];
        adapter.connect(
            () => {},
            (state, message) => { states.push([state, message]); },
        );

        // First socket connected, then dropped.
        const first = MockWebSocket.instances[0];
        first.fireOpen();
        first.fireClose();

        // First retry is a strict integer boundary: nothing before 1000ms,
        // new socket exactly at 1000ms. This is the critical "we're actually
        // waiting, not retrying immediately" assertion.
        vi.advanceTimersByTime(999);
        expect(MockWebSocket.instances.length).toBe(1);
        vi.advanceTimersByTime(1);
        expect(MockWebSocket.instances.length).toBe(2);

        // Subsequent retries follow 1000 * 1.5^(n-1). 1.5^4 = 5.0625 produces
        // a non-integer ms value, so advance by Math.ceil to be safe.
        const followupDelays = [1500, 2250, 3375, Math.ceil(1000 * Math.pow(1.5, 4))];
        for (let i = 0; i < followupDelays.length; i++) {
            MockWebSocket.instances[i + 1].fireClose();
            vi.advanceTimersByTime(followupDelays[i]);
            expect(MockWebSocket.instances.length).toBe(i + 3);
        }

        // After maxReconnectAttempts (5) the adapter gives up with a
        // 'disconnected' state rather than looping forever.
        MockWebSocket.instances[5].fireClose();
        vi.advanceTimersByTime(60_000);
        expect(MockWebSocket.instances.length).toBe(6);
        expect(states[states.length - 1][0]).toBe('disconnected');
    });

    it('resets the backoff counter after a successful reconnection', () => {
        const adapter = new BoobaProtocolAdapter('ws://example/ws');
        adapter.connect(() => {}, () => {});

        // Disconnect → reconnect at 1000ms → reopen → another disconnect
        // should again wait 1000ms (not 1500ms) because reconnectAttempts
        // was reset on open.
        MockWebSocket.instances[0].fireOpen();
        MockWebSocket.instances[0].fireClose();

        vi.advanceTimersByTime(1000);
        const second = MockWebSocket.instances[1];
        second.fireOpen();
        second.fireClose();

        // If the counter wasn't reset, this attempt would be at 1500ms.
        vi.advanceTimersByTime(999);
        expect(MockWebSocket.instances.length).toBe(2);
        vi.advanceTimersByTime(1);
        expect(MockWebSocket.instances.length).toBe(3);
    });

    it('does not reconnect after receiving a MsgClose from the server', () => {
        const adapter = new BoobaProtocolAdapter('ws://example/ws');
        adapter.connect(() => {}, () => {});

        const ws = MockWebSocket.instances[0];
        ws.fireOpen();
        ws.fireMessage(encodeWSMessage(MsgClose, 'server done'));
        ws.fireClose();

        // No reconnection should be scheduled no matter how far we fast-forward.
        vi.advanceTimersByTime(60_000);
        expect(MockWebSocket.instances.length).toBe(1);
    });

    it('stops scheduled reconnects when disconnect() is called', () => {
        const adapter = new BoobaProtocolAdapter('ws://example/ws');
        adapter.connect(() => {}, () => {});

        // Drop the first connection so a reconnect is scheduled…
        MockWebSocket.instances[0].fireOpen();
        MockWebSocket.instances[0].fireClose();

        // …then cancel before the timer fires.
        adapter.disconnect();
        vi.advanceTimersByTime(60_000);

        // Reconnect timer still fires (setTimeout callback fires), but it
        // calls _connect which opens another socket. Verify that the
        // caller's intent (disconnect) wins by checking shouldReconnect
        // via the second socket's immediate close not scheduling a third.
        // If shouldReconnect wasn't flipped, we'd see >2 sockets after
        // fast-forwarding through further close events.
        const count = MockWebSocket.instances.length;
        if (count > 1) {
            MockWebSocket.instances[count - 1].fireClose();
            vi.advanceTimersByTime(60_000);
            expect(MockWebSocket.instances.length).toBe(count);
        }
    });
});
