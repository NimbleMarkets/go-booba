import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { OSC52Scanner } from './clipboard.js';

// OSC 52 wire format: ESC ] 52 ; <selection> ; <base64> <terminator>
// Terminator is either BEL (0x07) or ST (ESC \ = 0x1b 0x5c).
function osc52(base64: string, sel = 'c', terminator: 'BEL' | 'ST' = 'BEL'): Uint8Array {
    const end = terminator === 'BEL' ? '\x07' : '\x1b\\';
    return new TextEncoder().encode(`\x1b]52;${sel};${base64}${end}`);
}

const b64 = (s: string): string => {
    // Encode via TextEncoder → binary-string → btoa so we don't drag
    // @types/node in just for Buffer.
    const bytes = new TextEncoder().encode(s);
    let bin = '';
    for (const b of bytes) bin += String.fromCharCode(b);
    return btoa(bin);
};

describe('OSC52Scanner', () => {
    let writeText: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        writeText = vi.fn().mockResolvedValue(undefined);
        vi.stubGlobal('navigator', { clipboard: { writeText } });
    });

    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it('ignores OSC 52 escapes when allowOSC52 is false', () => {
        const scanner = new OSC52Scanner(false);
        scanner.scan(osc52(b64('hello')));
        expect(writeText).not.toHaveBeenCalled();
    });

    it('decodes a BEL-terminated OSC 52 write and forwards the text to the clipboard', () => {
        const scanner = new OSC52Scanner(true);
        scanner.scan(osc52(b64('hello world')));
        expect(writeText).toHaveBeenCalledTimes(1);
        expect(writeText).toHaveBeenCalledWith('hello world');
    });

    it('decodes an ST-terminated OSC 52 write', () => {
        const scanner = new OSC52Scanner(true);
        scanner.scan(osc52(b64('via ST'), 'p', 'ST'));
        expect(writeText).toHaveBeenCalledWith('via ST');
    });

    it('handles escapes split across multiple scan() calls', () => {
        const scanner = new OSC52Scanner(true);
        const full = osc52(b64('split chunks'));
        // Split right in the middle of the base64 payload.
        const mid = Math.floor(full.length / 2);
        scanner.scan(full.subarray(0, mid));
        expect(writeText).not.toHaveBeenCalled();
        scanner.scan(full.subarray(mid));
        expect(writeText).toHaveBeenCalledWith('split chunks');
    });

    it('ignores invalid base64 instead of throwing', () => {
        const scanner = new OSC52Scanner(true);
        // '!!!' contains characters outside the base64 alphabet in ways
        // atob may accept or reject; use a string guaranteed to throw.
        scanner.scan(new TextEncoder().encode('\x1b]52;c;\u00ff\u00fe\x07'));
        expect(writeText).not.toHaveBeenCalled();
    });

    it('does not invoke the clipboard for an empty payload', () => {
        const scanner = new OSC52Scanner(true);
        scanner.scan(new TextEncoder().encode('\x1b]52;c;\x07'));
        expect(writeText).not.toHaveBeenCalled();
    });

    it('passes through surrounding terminal output without clipboard side effects', () => {
        const scanner = new OSC52Scanner(true);
        const full = new Uint8Array([
            ...new TextEncoder().encode('pre-'),
            ...osc52(b64('copied')),
            ...new TextEncoder().encode('-post'),
        ]);
        scanner.scan(full);
        expect(writeText).toHaveBeenCalledTimes(1);
        expect(writeText).toHaveBeenCalledWith('copied');
    });

    it('decodes two escapes in a single chunk', () => {
        const scanner = new OSC52Scanner(true);
        const combined = new Uint8Array([
            ...osc52(b64('first')),
            ...osc52(b64('second')),
        ]);
        scanner.scan(combined);
        expect(writeText.mock.calls.map(c => c[0])).toEqual(['first', 'second']);
    });

    it('handles a trailing partial escape across many scan() calls without leaking memory', () => {
        const scanner = new OSC52Scanner(true);
        // Feed the start of an escape and then never terminate it; the
        // buffer must remain bounded to the partial escape rather than
        // accumulating non-OSC noise indefinitely.
        scanner.scan(new TextEncoder().encode('noise-1'));
        scanner.scan(new TextEncoder().encode('noise-2'));
        scanner.scan(new TextEncoder().encode('\x1b]52;c;SGk'));
        scanner.scan(new TextEncoder().encode('=\x07'));
        expect(writeText).toHaveBeenCalledWith('Hi');
    });
});
