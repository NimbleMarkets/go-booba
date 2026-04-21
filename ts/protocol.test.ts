import { describe, it, expect } from 'vitest';
import {
    MsgInput, MsgOutput, MsgResize, MsgPing,
    encodeWSMessage, decodeWSMessage,
    encodeWTMessage, tryDecodeWTFrame,
    jsonPayload, parseJsonPayload,
} from './protocol.js';

describe('encodeWSMessage / decodeWSMessage', () => {
    it('writes the message type as the first byte and the payload after', () => {
        const payload = new Uint8Array([0x68, 0x69]); // "hi"
        const encoded = encodeWSMessage(MsgInput, payload);
        expect(encoded).toEqual(new Uint8Array([MsgInput, 0x68, 0x69]));
    });

    it('accepts a string payload and UTF-8 encodes it', () => {
        const encoded = encodeWSMessage(MsgInput, 'hi');
        expect(encoded).toEqual(new Uint8Array([MsgInput, 0x68, 0x69]));
    });

    it('encodes to a single byte when no payload is given', () => {
        const encoded = encodeWSMessage(MsgPing);
        expect(encoded).toEqual(new Uint8Array([MsgPing]));
    });

    it('round-trips through decodeWSMessage', () => {
        const payload = new Uint8Array([1, 2, 3, 4, 5]);
        const [type, out] = decodeWSMessage(encodeWSMessage(MsgOutput, payload));
        expect(type).toBe(MsgOutput);
        expect(Array.from(out)).toEqual([1, 2, 3, 4, 5]);
    });

    it('returns an empty payload when the encoded message has only a type byte', () => {
        const [type, out] = decodeWSMessage(new Uint8Array([MsgPing]));
        expect(type).toBe(MsgPing);
        expect(out.length).toBe(0);
    });

    it('throws on an empty buffer', () => {
        expect(() => decodeWSMessage(new Uint8Array(0))).toThrow(/empty/);
    });
});

describe('encodeWTMessage framing', () => {
    it('writes the body length (type + payload) as a big-endian uint32', () => {
        const payload = new Uint8Array([0xaa, 0xbb, 0xcc]);
        const encoded = encodeWTMessage(MsgInput, payload);
        // length header = 1 byte type + 3 byte payload = 4
        expect(encoded.length).toBe(4 + 1 + 3);
        const header = new DataView(encoded.buffer).getUint32(0, false);
        expect(header).toBe(4);
        expect(encoded[4]).toBe(MsgInput);
        expect(Array.from(encoded.subarray(5))).toEqual([0xaa, 0xbb, 0xcc]);
    });

    it('uses big-endian byte order, not little-endian', () => {
        const payload = new Uint8Array(260); // body len = 261 → 0x00000105
        const encoded = encodeWTMessage(MsgInput, payload);
        expect(Array.from(encoded.subarray(0, 4))).toEqual([0x00, 0x00, 0x01, 0x05]);
    });

    it('encodes empty-payload frames as 4-byte length (=1) + type byte', () => {
        const encoded = encodeWTMessage(MsgPing);
        expect(encoded.length).toBe(5);
        expect(new DataView(encoded.buffer).getUint32(0, false)).toBe(1);
        expect(encoded[4]).toBe(MsgPing);
    });
});

describe('tryDecodeWTFrame', () => {
    it('returns null on too-short input (< 4-byte header)', () => {
        const buf = new Uint8Array([0, 0, 0]);
        expect(tryDecodeWTFrame(buf, 0, 3)).toBeNull();
    });

    it('returns null when the header promises more bytes than we have', () => {
        // length=10 but only 2 body bytes present
        const buf = new Uint8Array([0, 0, 0, 10, MsgInput, 0x01]);
        expect(tryDecodeWTFrame(buf, 0, buf.length)).toBeNull();
    });

    it('decodes a single frame correctly and reports bytes consumed', () => {
        const encoded = encodeWTMessage(MsgOutput, new Uint8Array([1, 2, 3]));
        const frame = tryDecodeWTFrame(encoded, 0, encoded.length);
        expect(frame).not.toBeNull();
        expect(frame!.msgType).toBe(MsgOutput);
        expect(Array.from(frame!.payload)).toEqual([1, 2, 3]);
        expect(frame!.consumed).toBe(encoded.length);
    });

    it('decodes multiple concatenated frames by advancing start', () => {
        const a = encodeWTMessage(MsgOutput, new Uint8Array([1]));
        const b = encodeWTMessage(MsgInput, new Uint8Array([2, 3]));
        const c = encodeWTMessage(MsgPing);
        const combined = new Uint8Array(a.length + b.length + c.length);
        combined.set(a, 0);
        combined.set(b, a.length);
        combined.set(c, a.length + b.length);

        let start = 0;
        const out: Array<[number, number[]]> = [];
        while (true) {
            const frame = tryDecodeWTFrame(combined, start, combined.length);
            if (!frame) break;
            out.push([frame.msgType, Array.from(frame.payload)]);
            start += frame.consumed;
        }
        expect(out).toEqual([
            [MsgOutput, [1]],
            [MsgInput, [2, 3]],
            [MsgPing, []],
        ]);
        expect(start).toBe(combined.length);
    });

    it('handles a frame split across two chunks (simulating partial network read)', () => {
        const whole = encodeWTMessage(MsgResize, new Uint8Array([7, 8, 9, 10]));
        // Simulate a buffer containing only the first 6 of 9 bytes.
        expect(tryDecodeWTFrame(whole, 0, 6)).toBeNull();
        // Once the rest arrives, the full frame decodes.
        const frame = tryDecodeWTFrame(whole, 0, whole.length);
        expect(frame!.msgType).toBe(MsgResize);
        expect(Array.from(frame!.payload)).toEqual([7, 8, 9, 10]);
    });
});

describe('jsonPayload / parseJsonPayload', () => {
    it('round-trips a ResizeMessage', () => {
        const bytes = jsonPayload({ cols: 120, rows: 40 });
        expect(parseJsonPayload<{ cols: number; rows: number }>(bytes))
            .toEqual({ cols: 120, rows: 40 });
    });
});
