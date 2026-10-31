import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import type { BoobaAdapter, BoobaConnectionState } from './adapter.js';
vi.mock('ntwebterm', () => ({ init: vi.fn(), Terminal: class {}, FitAddon: class {}, installRendererHud: vi.fn(), parseRendererFromURL: vi.fn() }));
import { BoobaTerminal } from './booba.js';

// Program output reaches the terminal as bytes from a WebSocket and as
// strings from the wasm bridge. An OSC 52 clipboard write must copy either way.
class Adapter implements BoobaAdapter {
    state!: (state: BoobaConnectionState, message: string) => void;
    output!: (data: string | Uint8Array) => void;
    boobaRead = () => null;
    boobaWrite = vi.fn();
    boobaResize = vi.fn();
    disconnect = vi.fn();
    connect(output: Adapter['output'], state: Adapter['state']) { this.output = output; this.state = state; state('connected', 'ready'); }
}

function setup(allowOSC52: boolean) {
    const booba = new BoobaTerminal('unused', { allowOSC52 });
    const terminal = { options: { disableStdin: false }, input: vi.fn(), write: vi.fn(), reset: vi.fn(), dispose: vi.fn(), cols: 80, rows: 24, onProgramStatus: vi.fn(() => ({ dispose: vi.fn() })) };
    booba.term = terminal as unknown as NonNullable<BoobaTerminal['term']>;
    const adapter = new Adapter(); booba.connectAdapter(adapter);
    return { booba, terminal, adapter };
}

const copy = `\x1b]52;c;${btoa('hello')}\x07`;

describe('BoobaTerminal OSC 52 on program output', () => {
    let writeText: ReturnType<typeof vi.fn>;
    beforeEach(() => { writeText = vi.fn().mockResolvedValue(undefined); vi.stubGlobal('navigator', { clipboard: { writeText } }); });
    afterEach(() => { vi.unstubAllGlobals(); });

    it('copies from string output, as the wasm bridge delivers it', () => {
        const { adapter } = setup(true);
        adapter.output(copy);
        expect(writeText).toHaveBeenCalledWith('hello');
    });

    it('copies from byte output, as a WebSocket delivers it', () => {
        const { adapter } = setup(true);
        adapter.output(new TextEncoder().encode(copy));
        expect(writeText).toHaveBeenCalledWith('hello');
    });

    it('does not copy when allowOSC52 is off', () => {
        const { adapter } = setup(false);
        adapter.output(copy);
        expect(writeText).not.toHaveBeenCalled();
    });
});
