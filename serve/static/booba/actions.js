export const ACTION_LIMITS = { bytes: 65536, actions: 128, keys: 16, groups: 32 };
const idPattern = /^[A-Za-z0-9_.+-]{1,64}$/;
const namedKey = /^(space|enter|escape|tab|backspace|up|down|left|right|home|end|insert|delete|pageup|pagedown|f([1-9]|1[0-2]))$/;
const bytes = (text) => new TextEncoder().encode(text).length;
function fail(message) { throw new Error(`Invalid action snapshot: ${message}`); }
function record(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value))
        fail('expected object');
    return value;
}
function text(value, max) {
    if (typeof value !== 'string' || bytes(value) > max || /[\u0000-\u001f\u007f-\u009f]/u.test(value) || /[\ud800-\udfff]/u.test(value))
        fail('invalid text');
    return value;
}
function id(value) {
    if (typeof value !== 'string' || !idPattern.test(value))
        fail('invalid identifier');
    return value;
}
function array(value, max) {
    if (!Array.isArray(value) || value.length > max)
        fail('invalid array');
    return value;
}
function gesture(value) {
    const g = record(value);
    const key = text(g.key, 16);
    if (!namedKey.test(key) && !([...key].length === 1 && key !== ' '))
        fail('unsupported key');
    const modifiers = array(g.modifiers, 3).map(m => {
        if (m !== 'shift' && m !== 'alt' && m !== 'ctrl')
            fail('invalid modifier');
        return m;
    });
    if (new Set(modifiers).size !== modifiers.length)
        fail('duplicate modifier');
    return { key, modifiers };
}
/** Validate and copy known fields. Unknown fields are ignored; input is never retained. */
export function validateActionSnapshot(value) {
    const s = record(value);
    if (s.version !== 1 || !Number.isSafeInteger(s.revision) || s.revision < 0)
        fail('invalid version or revision');
    const ids = new Set();
    const actions = array(s.actions, ACTION_LIMITS.actions).map(value => {
        const a = record(value);
        const action = { id: id(a.id), label: text(a.label, 256), enabled: a.enabled,
            keys: array(a.keys, ACTION_LIMITS.keys).map(gesture) };
        if (ids.has(action.id) || !action.label || typeof a.enabled !== 'boolean' || !action.keys.length)
            fail('invalid action');
        ids.add(action.id);
        if (a.hint !== undefined) {
            const hint = text(a.hint, 128);
            if (hint)
                action.hint = hint;
        }
        if (a.preferred !== undefined) {
            if (!Number.isInteger(a.preferred) || a.preferred < 0 || a.preferred >= action.keys.length)
                fail('invalid preferred gesture');
            action.preferred = a.preferred;
        }
        return action;
    });
    const refs = (value, seen) => array(value, ACTION_LIMITS.actions).map(v => {
        const ref = id(v);
        if (!ids.has(ref) || seen.has(ref))
            fail('unknown or repeated reference');
        seen.add(ref);
        return ref;
    });
    const fullSeen = new Set();
    const result = { version: 1, instance: id(s.instance), context: id(s.context), revision: s.revision,
        actions, short: refs(s.short, new Set()), full: array(s.full, ACTION_LIMITS.groups).map(g => refs(g, fullSeen)) };
    // Match Go encoding/json's canonical escaped byte count for object validation.
    const encoded = JSON.stringify(result).replace(/[<>&\u2028\u2029]/gu, char => `\\u${char.charCodeAt(0).toString(16).padStart(4, '0')}`);
    if (bytes(encoded) > ACTION_LIMITS.bytes)
        fail('size limit exceeded');
    return result;
}
export function decodeActionSnapshot(json) {
    if (bytes(json) > ACTION_LIMITS.bytes)
        fail('size limit exceeded');
    return validateActionSnapshot(JSON.parse(json));
}
/** Owns one live connection's current actions. Hosts forward lifecycle events to clear(). */
export class ActionController {
    constructor(options) {
        this.options = options;
        this.current = null;
        this.generation = 0;
        this.connected = false;
        this.readOnly = false;
        this.listeners = new Set();
    }
    get snapshot() { return this.current ? validateActionSnapshot(this.current) : null; }
    subscribe(listener) { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; }
    changed() { for (const listener of this.listeners)
        listener(); }
    /** Call for connection/adapter replacement, including reconnecting to the same endpoint. */
    setConnected(connected) { this.connected = connected; this.clear(); }
    setReadOnly(readOnly) { this.readOnly = readOnly; this.changed(); }
    /** Reset, shell prompt, process exit, alternate-screen transition, or replay boundary. */
    clear() { this.current = null; this.generation++; this.changed(); }
    publish(value) {
        const next = validateActionSnapshot(value);
        if (!this.connected)
            return false;
        if (this.current?.instance === next.instance) {
            if (next.revision < this.current.revision)
                return false;
            if (next.revision === this.current.revision) {
                if (JSON.stringify(next) !== JSON.stringify(this.current))
                    fail('conflicting revision');
                return false;
            }
        }
        this.current = next;
        this.generation++;
        this.changed();
        return true;
    }
    canActivate(id, index) {
        const action = this.current?.actions.find(a => a.id === id);
        const key = action?.keys[index];
        return !!(this.connected && !this.readOnly && action?.enabled && key && this.options.canSendKey({ ...key, modifiers: [...key.modifiers] }));
    }
    /** Capture at render/pointer-down time; a replaced snapshot invalidates the closure. */
    capture(id, index) {
        const generation = this.generation;
        return () => {
            if (generation !== this.generation || !this.canActivate(id, index))
                return false;
            const key = this.current.actions.find(a => a.id === id).keys[index];
            return this.options.sendKey({ ...key, modifiers: [...key.modifiers] });
        };
    }
}
