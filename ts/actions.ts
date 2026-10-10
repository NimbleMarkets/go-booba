/** Framework-neutral terminal action descriptions. No OSC number is assigned. */
export interface ActionGesture { key: string; modifiers: ('shift' | 'alt' | 'ctrl')[] }
export interface TerminalAction {
    id: string;
    label: string;
    hint?: string;
    enabled: boolean;
    keys: ActionGesture[];
    /** Explicitly declares aliases and selects the one presented by default. */
    preferred?: number;
}
export interface ActionSnapshot {
    version: 1;
    instance: string;
    context: string;
    revision: number;
    actions: TerminalAction[];
    short: string[];
    full: string[][];
}
export const ACTION_LIMITS = { bytes: 65536, actions: 128, keys: 16, groups: 32 } as const;
const idPattern = /^[A-Za-z0-9_.+-]{1,64}$/;
const namedKey = /^(space|enter|escape|tab|backspace|up|down|left|right|home|end|insert|delete|pageup|pagedown|f([1-9]|1[0-2]))$/;
const bytes = (text: string) => new TextEncoder().encode(text).length;
function fail(message: string): never { throw new Error(`Invalid action snapshot: ${message}`); }
function record(value: unknown): Record<string, unknown> {
    if (!value || typeof value !== 'object' || Array.isArray(value)) fail('expected object');
    return value as Record<string, unknown>;
}
function text(value: unknown, max: number): string {
    if (typeof value !== 'string' || bytes(value) > max || /[\u0000-\u001f\u007f-\u009f]/u.test(value) || /[\ud800-\udfff]/u.test(value)) fail('invalid text');
    return value;
}
function id(value: unknown): string {
    if (typeof value !== 'string' || !idPattern.test(value)) fail('invalid identifier');
    return value;
}
function array(value: unknown, max: number): unknown[] {
    if (!Array.isArray(value) || value.length > max) fail('invalid array');
    return value;
}
function gesture(value: unknown): ActionGesture {
    const g = record(value);
    const key = text(g.key, 16);
    if (!namedKey.test(key) && !([...key].length === 1 && key !== ' ')) fail('unsupported key');
    const modifiers = array(g.modifiers, 3).map(m => {
        if (m !== 'shift' && m !== 'alt' && m !== 'ctrl') fail('invalid modifier');
        return m;
    });
    if (new Set(modifiers).size !== modifiers.length) fail('duplicate modifier');
    return { key, modifiers };
}
/** Validate and copy known fields. Unknown fields are ignored; input is never retained. */
export function validateActionSnapshot(value: unknown): ActionSnapshot {
    const s = record(value);
    if (s.version !== 1 || !Number.isSafeInteger(s.revision) || (s.revision as number) < 0) fail('invalid version or revision');
    const ids = new Set<string>();
    const actions = array(s.actions, ACTION_LIMITS.actions).map(value => {
        const a = record(value);
        const action: TerminalAction = { id: id(a.id), label: text(a.label, 256), enabled: a.enabled as boolean,
            keys: array(a.keys, ACTION_LIMITS.keys).map(gesture) };
        if (ids.has(action.id) || !action.label || typeof a.enabled !== 'boolean' || !action.keys.length) fail('invalid action');
        ids.add(action.id);
        if (a.hint !== undefined) { const hint = text(a.hint, 128); if (hint) action.hint = hint; }
        if (a.preferred !== undefined) {
            if (!Number.isInteger(a.preferred) || (a.preferred as number) < 0 || (a.preferred as number) >= action.keys.length) fail('invalid preferred gesture');
            action.preferred = a.preferred as number;
        }
        return action;
    });
    const refs = (value: unknown, seen: Set<string>): string[] => array(value, ACTION_LIMITS.actions).map(v => {
        const ref = id(v);
        if (!ids.has(ref) || seen.has(ref)) fail('unknown or repeated reference');
        seen.add(ref);
        return ref;
    });
    const fullSeen = new Set<string>();
    const result: ActionSnapshot = { version: 1, instance: id(s.instance), context: id(s.context), revision: s.revision as number,
        actions, short: refs(s.short, new Set()), full: array(s.full, ACTION_LIMITS.groups).map(g => refs(g, fullSeen)) };
    // Match Go encoding/json's canonical escaped byte count for object validation.
    const encoded = JSON.stringify(result).replace(/[<>&\u2028\u2029]/gu,
        char => `\\u${char.charCodeAt(0).toString(16).padStart(4, '0')}`);
    if (bytes(encoded) > ACTION_LIMITS.bytes) fail('size limit exceeded');
    return result;
}
export function decodeActionSnapshot(json: string): ActionSnapshot {
    if (bytes(json) > ACTION_LIMITS.bytes) fail('size limit exceeded');
    return validateActionSnapshot(JSON.parse(json));
}

/** Host-provided encoder; capabilities and encoding must reflect current terminal modes. */
export interface ActionKeyEncoder {
    canEncode: (gesture: ActionGesture) => boolean;
    encode: (gesture: ActionGesture) => string | null;
}

export interface ActionControllerOptions {
    /** Must encode a complete gesture in current terminal modes and send via normal input. */
    sendKey: (gesture: ActionGesture) => boolean;
    /** Consult current input permissions and encoder capabilities. */
    canSendKey: (gesture: ActionGesture) => boolean;
}
/** Owns one live connection's current actions. Hosts forward lifecycle events to clear(). */
export class ActionController {
    private current: ActionSnapshot | null = null;
    private generation = 0;
    private connected = false;
    private readOnly = false;
    private listeners = new Set<() => void>();
    constructor(private readonly options: ActionControllerOptions) {}
    get snapshot(): ActionSnapshot | null { return this.current ? validateActionSnapshot(this.current) : null; }
    subscribe(listener: () => void): () => void { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; }
    private changed(): void { for (const listener of this.listeners) listener(); }
    /** Call for connection/adapter replacement, including reconnecting to the same endpoint. */
    setConnected(connected: boolean): void { this.connected = connected; this.clear(); }
    setReadOnly(readOnly: boolean): void { this.readOnly = readOnly; this.changed(); }
    /** Reset, shell prompt, process exit, alternate-screen transition, or replay boundary. */
    clear(): void { this.current = null; this.generation++; this.changed(); }
    publish(value: unknown): boolean {
        const next = validateActionSnapshot(value);
        if (!this.connected) return false;
        if (this.current?.instance === next.instance) {
            if (next.revision < this.current.revision) return false;
            if (next.revision === this.current.revision) {
                if (JSON.stringify(next) !== JSON.stringify(this.current)) fail('conflicting revision');
                return false;
            }
        }
        this.current = next;
        this.generation++;
        this.changed();
        return true;
    }
    canActivate(id: string, index: number): boolean {
        const action = this.current?.actions.find(a => a.id === id);
        const key = action?.keys[index];
        return !!(this.connected && !this.readOnly && action?.enabled && key && this.options.canSendKey({ ...key, modifiers: [...key.modifiers] }));
    }
    /** Capture at render/pointer-down time; a replaced snapshot invalidates the closure. */
    capture(id: string, index: number): () => boolean {
        const generation = this.generation;
        return () => {
            if (generation !== this.generation || !this.canActivate(id, index)) return false;
            const key = this.current!.actions.find(a => a.id === id)!.keys[index];
            return this.options.sendKey({ ...key, modifiers: [...key.modifiers] });
        };
    }
}
