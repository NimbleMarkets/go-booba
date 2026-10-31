import { describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import type { BoobaAdapter, BoobaConnectionState } from './adapter.js';
vi.mock('ntwebterm', () => ({ init: vi.fn(), Terminal: class {}, FitAddon: class {}, installRendererHud: vi.fn(), parseRendererFromURL: vi.fn() }));
import { BoobaTerminal } from './booba.js';
import { ACTION_MESSAGE_MARKER, actionStatusID } from './action_status.js';
import type { ProgramStatusReport } from './program_status.js';

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

function report(over: Partial<ProgramStatusReport>): ProgramStatusReport {
    return { state: 'idle', kind: null, progress: null, id: '', app: '', title: '', message: '', ...over };
}

function setup(options: ConstructorParameters<typeof BoobaTerminal>[1] = {}) {
    const booba = new BoobaTerminal('unused', options);
    const dispose = vi.fn();
    let emit: ((report: ProgramStatusReport) => void) | null = null;
    const subscribe = vi.fn((listener: (report: ProgramStatusReport) => void) => { emit = listener; return { dispose }; });
    const terminal = { options: { disableStdin: false }, input: vi.fn(), write: vi.fn(), reset: vi.fn(), dispose: vi.fn(), cols: 80, rows: 24, onProgramStatus: subscribe };
    booba.term = terminal as unknown as NonNullable<BoobaTerminal['term']>;
    const adapter = new Adapter(); booba.connectAdapter(adapter);
    return { booba, terminal, adapter, subscribe, dispose, emit: (r: ProgramStatusReport) => emit!(r) };
}

describe('BoobaTerminal program status store', () => {
    it('subscribes to the terminal when the store gains a subscriber and disposes when it loses the last', () => {
        const { booba, subscribe, dispose, emit } = setup();
        expect(subscribe).not.toHaveBeenCalled();
        const unsubscribe = booba.programStatus.subscribe(() => {});
        expect(subscribe).toHaveBeenCalledTimes(1);
        emit(report({ id: 'job', state: 'working', message: 'Busy' }));
        expect(booba.programStatus.get('job')!.message).toBe('Busy');
        unsubscribe();
        expect(dispose).toHaveBeenCalledTimes(1);
    });

    it('feeds the store after the action bridge, with the readable summary as the message', () => {
        const { booba, emit } = setup({ allowOSC7501Actions: true });
        booba.programStatus.subscribe(() => {}); // activates the terminal subscription in this harness
        booba.setActionKeyEncoder({ canEncode: () => true, encode: () => '\r' });
        const snap = snapshot();
        emit(report({ id: actionStatusID(snap.instance), state: 'idle', app: 'booba-demo',
            message: 'Choose an item' + ACTION_MESSAGE_MARKER + JSON.stringify(snap) }));
        expect(booba.actions.snapshot).not.toBeNull();
        const record = booba.programStatus.get(actionStatusID(snap.instance))!;
        expect(record.message).toBe('Choose an item');
        expect(record.app).toBe('booba-demo');
    });

    it('still delivers reports to the onProgramStatus callback', () => {
        const { booba, emit } = setup();
        const listener = vi.fn(); booba.onProgramStatus = listener;
        const r = report({ id: 'x', state: 'done', message: 'Finished' });
        emit(r);
        expect(listener).toHaveBeenCalledWith(r);
        expect(booba.programStatus.get('x')!.state).toBe('done');
    });

    it('applies prompt and reset boundaries from live output and ignores alternate-screen switches', () => {
        const { booba, adapter, emit } = setup();
        booba.programStatus.subscribe(() => {});
        emit(report({ id: 'work', state: 'working' }));
        emit(report({ id: 'result', state: 'done' }));
        adapter.output('\x1b[?1049h');
        expect(booba.programStatus.records.map(r => r.id)).toEqual(['result', 'work']);
        adapter.output('\x1b]133;A\x07$ ');
        expect(booba.programStatus.records.map(r => r.id)).toEqual(['result']);
        emit(report({ id: 'work', state: 'blocked', kind: 'question' }));
        adapter.output('\x1bc');
        expect(booba.programStatus.records).toHaveLength(0);
    });

    it('drops in-flight records when the adapter disconnects and clears on a new connection', () => {
        const { booba, adapter, emit } = setup();
        booba.programStatus.subscribe(() => {});
        emit(report({ id: 'work', state: 'working' }));
        emit(report({ id: 'result', state: 'done' }));
        adapter.state('disconnected', 'Program exited');
        expect(booba.programStatus.records.map(r => r.id)).toEqual(['result']);
        const next = new Adapter(); booba.connectAdapter(next); // connect() reports 'connected'
        expect(booba.programStatus.records).toHaveLength(0);
    });

    it('dismisses done and error on the next user input only', () => {
        const { booba, adapter, emit } = setup();
        booba.programStatus.subscribe(() => {});
        emit(report({ id: 'result', state: 'done' }));
        emit(report({ id: 'work', state: 'working' }));
        adapter.output('more output');
        expect(booba.programStatus.get('result')).toBeDefined();
        booba.forwardInput('j');
        expect(adapter.boobaWrite).toHaveBeenCalledWith('j');
        expect(booba.programStatus.records.map(r => r.id)).toEqual(['work']);
        booba.forwardInput('k');
        expect(booba.programStatus.records.map(r => r.id)).toEqual(['work']);
    });

    it('clears on reset and dispose', () => {
        const a = setup(); a.booba.programStatus.subscribe(() => {});
        a.emit(report({ id: 'x', state: 'done' }));
        a.booba.reset();
        expect(a.booba.programStatus.records).toHaveLength(0);
        const b = setup(); b.booba.programStatus.subscribe(() => {});
        b.emit(report({ id: 'y', state: 'error' }));
        b.booba.dispose();
        expect(b.booba.programStatus.records).toHaveLength(0);
    });

    it('does not treat terminal-generated replies as user input', () => {
        const { booba, adapter, emit, terminal } = setup();
        booba.programStatus.subscribe(() => {});
        emit(report({ id: 'result', state: 'done' }));
        // ntwebterm answers queries synchronously inside write(), on the same onData path as keystrokes.
        terminal.write.mockImplementation(() => booba.forwardInput('\x1b]7501;?\x1b\\'));
        adapter.output('\x1b]7501;?\x1b\\');
        expect(adapter.boobaWrite).toHaveBeenCalledWith('\x1b]7501;?\x1b\\');
        expect(booba.programStatus.get('result')).toBeDefined();
        booba.forwardInput('j');
        expect(booba.programStatus.get('result')).toBeUndefined();
    });
});
