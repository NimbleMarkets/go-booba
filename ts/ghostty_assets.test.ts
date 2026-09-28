// @vitest-environment node

import { readFile } from 'node:fs/promises';
import { beforeAll, describe, expect, it } from 'vitest';
import { Ghostty } from '../serve/static/ghostty-web/ghostty-web.js';

// Exercise the actual embedded bundle and WASM together. Type-checking the
// wrapper alone cannot detect a stale WASM binary paired with a newer bundle.
describe('embedded ghostty-web assets', () => {
    let ghostty: Ghostty;

    beforeAll(async () => {
        const wasm = await readFile(new URL('../serve/static/ghostty-web/ghostty-vt.wasm', import.meta.url));
        ghostty = await Ghostty.load(`data:application/wasm;base64,${wasm.toString('base64')}`);
    });

    it('parses terminal output and reports cursor position', () => {
        const term = ghostty.createTerminal(80, 24);
        try {
            term.write('hello\x1b[6n');
            const line = term.getLine(0)!;
            expect(String.fromCodePoint(...line.slice(0, 5).map(cell => cell.codepoint))).toBe('hello');
            expect(term.readResponse()).toBe('\x1b[1;6R');
        } finally {
            term.free();
        }
    });

    it('consumes a browser Kitty shared-memory image', () => {
        const previousRegistry = globalThis.ghosttyKittySharedMemory;
        const term = ghostty.createTerminal(80, 24);
        const name = '/booba-asset-smoke-test';
        const pixels = new Uint8Array([255, 0, 0, 255]);
        try {
            const registry = globalThis.ghosttyKittySharedMemory!;
            expect(registry).toBeInstanceOf(Map);
            registry.set(name, pixels);
            term.write(`\x1b_Ga=T,t=s,f=32,s=1,v=1,S=4,i=7,c=1,r=1,q=2;${btoa(name)}\x1b\\`);
            expect(registry.has(name)).toBe(false);
            const graphics = term.getKittyGraphics();
            expect(graphics).not.toBeNull();
            const image = term.getKittyImagePixels(graphics!, 7);
            expect(image?.width).toBe(1);
            expect(image?.height).toBe(1);
            expect(image?.data).toEqual(pixels);
        } finally {
            globalThis.ghosttyKittySharedMemory?.delete(name);
            globalThis.ghosttyKittySharedMemory = previousRegistry;
            term.free();
        }
    });
});
