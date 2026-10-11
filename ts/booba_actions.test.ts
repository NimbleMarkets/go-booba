import { describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import type { BoobaAdapter, BoobaConnectionState } from './adapter.js';
vi.mock('ghostty-web', () => ({ init: vi.fn(), Terminal: class {}, FitAddon: class {}, installRendererHud: vi.fn(), parseRendererFromURL: vi.fn() }));
import { BoobaTerminal } from './booba.js';
import { BoobaProtocolAdapter, type WebSocketAdapterCallbacks } from './websocket_adapter.js';
const snapshot = () => JSON.parse(readFileSync('actions/testdata/snapshots.json', 'utf8'))[0].snapshot;
class Adapter implements BoobaAdapter {
    state!: (state: BoobaConnectionState, message: string) => void;
    output!: (data: string | Uint8Array) => void;
    boobaRead = () => null;
    boobaWrite = vi.fn();
    boobaResize = vi.fn();
    disconnect = vi.fn();
    connect(output: Adapter['output'], state: Adapter['state']) { this.output = output; this.state = state; state('connected', 'ready'); }
}
function setup() {
    const booba = new BoobaTerminal('unused');
    const terminal = { options: { disableStdin: false }, input: vi.fn(), write: vi.fn(), reset: vi.fn(), cols: 80, rows: 24 };
    booba.term = terminal as unknown as NonNullable<BoobaTerminal['term']>;
    const adapter = new Adapter(); booba.connectAdapter(adapter);
    return { booba, terminal, adapter };
}
describe('Booba actions integration', () => {
    it('encodes with live terminal modes and declines before initialization', () => {
        const uninitialized = new BoobaTerminal('unused');
        expect(uninitialized.encodeKey({ key: 'up', modifiers: [] })).toBeNull();
        const { booba, terminal } = setup();
        let application = false;
        Object.assign(terminal, { getMode: (mode: number) => mode === 1 && application });
        const gesture = { key: 'up', modifiers: [] };
        expect(booba.encodeKey(gesture)).toBe('\x1b[A');
        application = true;
        expect(booba.encodeKey(gesture)).toBe('\x1bOA');
    });
    it('waits for writable server options and applies later read-only changes', () => {
        const connect = vi.spyOn(BoobaProtocolAdapter.prototype, 'connect').mockImplementation((_output, state) => state('connected', 'ready'));
        try {
            const { booba, terminal } = setup();
            booba.setActionKeyEncoder({ canEncode: () => true, encode: () => '\r' });
            booba.connectWebSocket('ws://example.invalid');
            booba.actions.publish(snapshot());
            const options = (booba.adapter as unknown as { callbacks: WebSocketAdapterCallbacks }).callbacks;
            const click = booba.actions.capture('open', 0);
            expect(click()).toBe(false);
            options.onOptions!({ readOnly: false }); expect(click()).toBe(true);
            options.onOptions!({ readOnly: true }); expect(click()).toBe(false);
            expect(terminal.input).toHaveBeenCalledTimes(1);
            booba.disconnect();
        } finally { connect.mockRestore(); }
    });
    it('requires explicit encoding, sends through terminal input, and honors disableStdin', () => {
        const { booba, terminal } = setup();
        booba.actions.publish(snapshot());
        expect(booba.actions.capture('open', 0)()).toBe(false);
        booba.setActionKeyEncoder({ canEncode: g => g.key === 'enter', encode: () => '\r' });
        booba.actions.publish(snapshot());
        const click = booba.actions.capture('open', 0);
        expect(click()).toBe(true); expect(terminal.input).toHaveBeenCalledWith('\r', true);
        terminal.options.disableStdin = true; expect(click()).toBe(false);
        terminal.options.disableStdin = false; booba.options.disableStdin = true; expect(click()).toBe(false);
    });
    it('invalidates on reset, replacement and reconnect, ignoring previous adapter callbacks', () => {
        const { booba, adapter } = setup();
        booba.setActionKeyEncoder({ canEncode: () => true, encode: () => '\r' });
        booba.actions.publish(snapshot()); const click = booba.actions.capture('open', 0);
        booba.reset(); expect(click()).toBe(false);
        booba.actions.publish(snapshot());
        const next = new Adapter(); booba.connectAdapter(next);
        expect(adapter.disconnect).toHaveBeenCalled(); expect(booba.actions.snapshot).toBeNull();
        booba.actions.publish(snapshot());
        adapter.state('disconnected', 'old'); expect(booba.actions.snapshot).not.toBeNull();
        next.state('reconnecting', 'retry'); expect(booba.actions.snapshot).toBeNull();
        next.state('connected', 'ready'); expect(booba.actions.snapshot).toBeNull();
        booba.actions.publish(snapshot()); booba.disconnect(); expect(booba.actions.snapshot).toBeNull();
    });
});

describe('optional OSC 7501 bridge', () => {
    it('subscribes only when a host consumes status and disposes replaced handlers', () => {
        const { booba, terminal } = setup();
        expect(booba.supportsProgramStatus).toBe(false);
        const dispose = vi.fn();
        let emit!: (report: any) => void;
        Object.assign(terminal, { onProgramStatus: (listener: typeof emit) => { emit = listener; return { dispose }; } });
        expect(booba.supportsProgramStatus).toBe(true);
        const listener = vi.fn(); booba.onProgramStatus = listener;
        const report = { state: 'working', kind: null, progress: 50, id: '', app: 'demo', title: '', message: 'Loading' };
        emit(report); expect(listener).toHaveBeenCalledWith(report);
        booba.onProgramStatus = null; expect(dispose).toHaveBeenCalledOnce();
    });
});
