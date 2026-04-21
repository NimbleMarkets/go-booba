import { describe, it, expect } from 'vitest';
import {
    MsgInput, MsgOutput, MsgPing, MsgTitle,
    encodeWTMessage, tryDecodeWTFrame,
} from './protocol.js';

// These tests exercise the same length-prefix parsing shape that
// webtransport_adapter._readLoop uses: accumulate bytes into a buffer,
// call tryDecodeWTFrame in a loop until it returns null, then compact
// the remainder to the front. If _readLoop changes its feeding pattern,
// these tests still pin the contract of the decoder it relies on.

/** Drain all complete frames from a growing buffer. Returns the parsed
 *  frames and any tail bytes that didn't form a complete frame. */
function drain(chunks: Uint8Array[]) {
    let buf = new Uint8Array(4096);
    let len = 0;

    const ensure = (need: number) => {
        if (buf.length >= need) return;
        let n = buf.length * 2;
        while (n < need) n *= 2;
        const next = new Uint8Array(n);
        next.set(buf.subarray(0, len));
        buf = next;
    };

    const frames: Array<{ msgType: number; payload: number[] }> = [];

    for (const chunk of chunks) {
        ensure(len + chunk.length);
        buf.set(chunk, len);
        len += chunk.length;

        let consumed = 0;
        while (true) {
            const frame = tryDecodeWTFrame(buf, consumed, len);
            if (!frame) break;
            frames.push({ msgType: frame.msgType, payload: Array.from(frame.payload) });
            consumed += frame.consumed;
        }
        // Compact remaining bytes to front, as _readLoop does.
        if (consumed > 0) {
            buf.set(buf.subarray(consumed, len), 0);
            len -= consumed;
        }
    }

    return { frames, tail: Array.from(buf.subarray(0, len)) };
}

describe('WebTransport length-prefix framing', () => {
    it('decodes a single frame delivered whole', () => {
        const encoded = encodeWTMessage(MsgOutput, new Uint8Array([0xaa, 0xbb]));
        const { frames, tail } = drain([encoded]);
        expect(frames).toEqual([{ msgType: MsgOutput, payload: [0xaa, 0xbb] }]);
        expect(tail).toEqual([]);
    });

    it('decodes multiple frames delivered in one chunk', () => {
        const a = encodeWTMessage(MsgOutput, new Uint8Array([1]));
        const b = encodeWTMessage(MsgInput, new Uint8Array([2, 3]));
        const c = encodeWTMessage(MsgPing);
        const combined = new Uint8Array(a.length + b.length + c.length);
        combined.set(a, 0);
        combined.set(b, a.length);
        combined.set(c, a.length + b.length);

        const { frames, tail } = drain([combined]);
        expect(frames).toEqual([
            { msgType: MsgOutput, payload: [1] },
            { msgType: MsgInput, payload: [2, 3] },
            { msgType: MsgPing, payload: [] },
        ]);
        expect(tail).toEqual([]);
    });

    it('reassembles a frame split across two chunks', () => {
        const whole = encodeWTMessage(MsgTitle, new TextEncoder().encode('hello'));
        const mid = Math.floor(whole.length / 2);
        const { frames, tail } = drain([whole.subarray(0, mid), whole.subarray(mid)]);
        expect(frames).toEqual([{ msgType: MsgTitle, payload: Array.from(new TextEncoder().encode('hello')) }]);
        expect(tail).toEqual([]);
    });

    it('holds incomplete frames without emitting them', () => {
        const whole = encodeWTMessage(MsgOutput, new Uint8Array([9, 9, 9, 9]));
        // Deliver only the 4-byte length header, nothing else.
        const { frames, tail } = drain([whole.subarray(0, 4)]);
        expect(frames).toEqual([]);
        expect(tail.length).toBe(4);
    });

    it('handles tiny one-byte-at-a-time chunks', () => {
        const whole = encodeWTMessage(MsgOutput, new Uint8Array([7, 8]));
        const oneByteChunks: Uint8Array[] = [];
        for (const b of whole) oneByteChunks.push(new Uint8Array([b]));
        const { frames, tail } = drain(oneByteChunks);
        expect(frames).toEqual([{ msgType: MsgOutput, payload: [7, 8] }]);
        expect(tail).toEqual([]);
    });

    it('handles a mix of complete + trailing partial frame in one chunk', () => {
        const complete = encodeWTMessage(MsgOutput, new Uint8Array([1, 2]));
        const partial = encodeWTMessage(MsgInput, new Uint8Array([3, 4, 5, 6]));
        const mixed = new Uint8Array(complete.length + 5); // complete + 5 bytes of partial
        mixed.set(complete, 0);
        mixed.set(partial.subarray(0, 5), complete.length);

        const { frames, tail } = drain([mixed]);
        expect(frames).toEqual([{ msgType: MsgOutput, payload: [1, 2] }]);
        expect(tail.length).toBe(5); // 4-byte header + 1 partial body byte
    });

    it('decodes a frame whose length header exceeds the initial 4 KiB buffer', () => {
        const bigPayload = new Uint8Array(8 * 1024);
        for (let i = 0; i < bigPayload.length; i++) bigPayload[i] = i & 0xff;
        const encoded = encodeWTMessage(MsgOutput, bigPayload);

        const { frames, tail } = drain([encoded]);
        expect(frames.length).toBe(1);
        expect(frames[0].msgType).toBe(MsgOutput);
        expect(frames[0].payload.length).toBe(bigPayload.length);
        expect(frames[0].payload[0]).toBe(0);
        expect(frames[0].payload[255]).toBe(255);
        expect(tail).toEqual([]);
    });
});
