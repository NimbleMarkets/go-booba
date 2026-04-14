/**
 * Booba Protocol v2 — Sip-compatible message encoding/decoding.
 */

export const MsgInput     = 0x30; // '0'
export const MsgOutput    = 0x31; // '1'
export const MsgResize    = 0x32; // '2'
export const MsgPing      = 0x33; // '3'
export const MsgPong      = 0x34; // '4'
export const MsgTitle     = 0x35; // '5'
export const MsgOptions   = 0x36; // '6'
export const MsgClose     = 0x37; // '7'
export const MsgKittyKbd  = 0x38; // '8'

export interface ResizeMessage {
    cols: number;
    rows: number;
}

export interface OptionsMessage {
    readOnly: boolean;
}

export interface KittyKbdMessage {
    flags: number;
}

/** Encode a WebSocket protocol message: [type][payload] */
export function encodeWSMessage(msgType: number, payload?: Uint8Array | string): Uint8Array {
    const payloadBytes = payload
        ? (typeof payload === 'string' ? new TextEncoder().encode(payload) : payload)
        : new Uint8Array(0);
    const msg = new Uint8Array(1 + payloadBytes.length);
    msg[0] = msgType;
    msg.set(payloadBytes, 1);
    return msg;
}

/** Decode a WebSocket protocol message. Returns [type, payload]. */
export function decodeWSMessage(data: Uint8Array): [number, Uint8Array] {
    if (data.length === 0) throw new Error('empty message');
    return [data[0], data.subarray(1)];
}

/** Encode a WebTransport protocol message: [4-byte length][type][payload] */
export function encodeWTMessage(msgType: number, payload?: Uint8Array | string): Uint8Array {
    const payloadBytes = payload
        ? (typeof payload === 'string' ? new TextEncoder().encode(payload) : payload)
        : new Uint8Array(0);
    const bodyLen = 1 + payloadBytes.length;
    const msg = new Uint8Array(4 + bodyLen);
    new DataView(msg.buffer).setUint32(0, bodyLen, false);
    msg[4] = msgType;
    msg.set(payloadBytes, 5);
    return msg;
}

/** Encode a JSON payload as UTF-8 bytes */
export function jsonPayload(obj: unknown): Uint8Array {
    return new TextEncoder().encode(JSON.stringify(obj));
}

/** Decode a UTF-8 JSON payload */
export function parseJsonPayload<T>(data: Uint8Array): T {
    return JSON.parse(new TextDecoder().decode(data));
}
