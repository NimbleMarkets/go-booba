import { describe, expect, it } from 'vitest';
import { TerminalSequenceObserver } from './terminal_sequences.js';
function observer(limit = 4096) {
    const reports: string[] = [], lifecycle: string[] = [];
    return { reports, lifecycle, parser: new TerminalSequenceObserver({ osc: s => reports.push(s), lifecycle: s => lifecycle.push(s) }, limit) };
}
describe('bounded terminal sequence observation', () => {
    it('handles every split and both terminators without parsing ordinary text', () => {
        const wire = 'hello\x1b]777;example;data\x1b\\world\x1b]0;title\x07';
        for (let split = 0; split <= wire.length; split++) {
            const { parser, reports } = observer();
            parser.scan(new TextEncoder().encode(wire.slice(0, split))); parser.scan(wire.slice(split));
            expect(reports).toEqual(['777;example;data', '0;title']);
        }
    });
    it('ignores fake OSC inside opaque control strings', () => {
        for (const start of ['P', 'X', '^', '_']) {
            const { parser, reports } = observer();
            parser.scan(`\x1b${start}\x1b]777;fake\x07\x1b\\\x1b]777;real\x07`);
            expect(reports).toEqual(['777;real']);
        }
    });
    it('bounds and discards oversized/malformed sequences and recovers', () => {
        const { parser, reports } = observer(32);
        parser.scan('\x1b]' + 'x'.repeat(100000) + '\x07');
        parser.scan('\x1b]bad\nbody\x07\x1b]cancel\x18\x1b]ok\x1b\\');
        expect(reports).toEqual(['ok']);
    });
    it('observes reset, alternate-screen transitions and shell prompts', () => {
        const { parser, lifecycle } = observer();
        const wire = '\x1b[?1049h\x1b[?1049h\x1b[?1049l\x1b]133;A;cl=m\x07\x1bc';
        for (const char of wire) parser.scan(char);
        expect(lifecycle).toEqual(['screen', 'screen', 'prompt', 'reset']);
    });
    it('does not complete old partial output after a connection reset', () => {
        const { parser, reports } = observer(); parser.scan('\x1b]777;old'); parser.reset(); parser.scan('\x07');
        expect(reports).toEqual([]);
    });
});
