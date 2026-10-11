import { describe, expect, it } from 'vitest';
import { encodeKey } from './key_encoder.js';
import type { ActionGesture } from './actions.js';

const g = (key: string, ...modifiers: ActionGesture['modifiers']): ActionGesture => ({ key, modifiers });
describe('legacy key encoder', () => {
    it('honors cursor, backarrow and newline modes', () => {
        expect(encodeKey(g('up'), {})).toBe('\x1b[A');
        expect(encodeKey(g('up'), { applicationCursor: true })).toBe('\x1bOA');
        expect(encodeKey(g('up', 'ctrl', 'alt'), { applicationCursor: true })).toBe('\x1b[1;7A');
        expect(encodeKey(g('backspace'), {})).toBe('\x7f');
        expect(encodeKey(g('backspace'), { backarrow: true })).toBe('\b');
        expect(encodeKey(g('enter'), { newline: true })).toBe('\r\n');
    });
    it.each([
        [g('a', 'ctrl'), '\x01'], [g('space', 'ctrl'), '\x00'],
        [g('a', 'shift', 'alt'), '\x1bA'], [g('界'), '界'],
        [g('tab', 'shift'), '\x1b[Z'], [g('f1'), '\x1bOP'],
        [g('f12', 'ctrl'), '\x1b[24;5~'], [g('delete'), '\x1b[3~'],
    ])('encodes %j', (gesture, expected) => expect(encodeKey(gesture, {})).toBe(expected));
    it('declines unsupported protocols and gestures', () => {
        expect(encodeKey(g('a'), { extendedKeyboard: true })).toBeNull();
        for (const gesture of [g('unknown'), g('constructor'), g('__proto__'), g('\n'), g('1', 'shift'), g('界', 'ctrl'), g('a', 'alt', 'alt')]) {
            expect(encodeKey(gesture, {})).toBeNull();
        }
    });
});
