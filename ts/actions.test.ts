import { describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { ActionController, decodeActionSnapshot, validateActionSnapshot } from './actions.js';
import { installActionBar } from './action_bar.js';

const fixtures = JSON.parse(readFileSync('actions/testdata/snapshots.json', 'utf8'));
const snapshot = () => structuredClone(fixtures[0].snapshot);
function controller() {
    const sendKey = vi.fn(() => true);
    const c = new ActionController({ sendKey, canSendKey: () => true });
    c.setConnected(true);
    c.publish(snapshot());
    return { c, sendKey };
}
describe('shared action fixtures', () => {
    for (const fixture of fixtures) it(fixture.name, () => {
        const decode = () => decodeActionSnapshot(JSON.stringify(fixture.snapshot));
        if (fixture.valid) expect(decode).not.toThrow(); else expect(decode).toThrow();
    });
    it('requires enabled and checks encoded bounds', () => {
        const s = snapshot(); delete s.actions[0].enabled;
        expect(() => validateActionSnapshot(s)).toThrow();
        expect(() => decodeActionSnapshot(' '.repeat(65537))).toThrow();
    });
});
describe('action controller', () => {
    it('rejects stale closures, lower revisions and conflicting revisions', () => {
        const { c, sendKey } = controller();
        const old = c.capture('open', 0);
        const next = snapshot(); next.revision++;
        expect(c.publish(next)).toBe(true);
        expect(old()).toBe(false);
        expect(c.publish(snapshot())).toBe(false);
        expect(c.publish(next)).toBe(false);
        next.context = 'dialog';
        expect(() => c.publish(next)).toThrow('conflicting revision');
        expect(c.capture('open', 0)()).toBe(true);
        expect(sendKey).toHaveBeenCalledExactlyOnceWith({ key: 'enter', modifiers: [] });
    });
    it('clears across reconnects and enforces current permissions', () => {
        const { c, sendKey } = controller();
        const click = c.capture('open', 0);
        c.setReadOnly(true); expect(click()).toBe(false);
        c.setReadOnly(false); expect(click()).toBe(true);
        c.setConnected(false); expect(click()).toBe(false);
        expect(c.publish(snapshot())).toBe(false);
        c.setConnected(true); expect(c.snapshot).toBeNull();
        c.publish(snapshot()); expect(click()).toBe(false);
        c.clear(); expect(c.snapshot).toBeNull();
        expect(sendKey).toHaveBeenCalledTimes(1);
    });
    it('does not retain caller mutations and fails closed for unsupported gestures', () => {
        const { c } = controller();
        c.snapshot!.actions[0].keys[0].key = 'x';
        expect(c.snapshot!.actions[0].keys[0].key).toBe('enter');
        const s = snapshot(); s.revision++; s.actions[0].enabled = false;
        c.publish(s); s.actions[0].enabled = true;
        expect(c.capture('open', 0)()).toBe(false);
        expect(c.capture('open', -1)()).toBe(false);
        const unsupported = new ActionController({ sendKey: () => { throw Error('must not send'); }, canSendKey: () => false });
        unsupported.setConnected(true); unsupported.publish(snapshot());
        expect(unsupported.capture('open', 0)()).toBe(false);
    });
});
describe('action bar', () => {
    it('expands grouped gestures, preserves short-only actions and sends only on activation', () => {
        const { c, sendKey } = controller();
        const parent = document.createElement('div'); document.body.append(parent);
        const keyboard = vi.fn();
        const dispose = installActionBar(c, { parent, openKeyboard: keyboard });
        const find = (label: string) => Array.from(parent.querySelectorAll('button')).find(b => b.textContent === label)!;
        expect(find('Open')).toBeTruthy(); expect(sendKey).not.toHaveBeenCalled();
        find('More').click();
        find('Navigate (down)').click();
        expect(sendKey).toHaveBeenLastCalledWith({ key: 'down', modifiers: [] });
        expect(keyboard).not.toHaveBeenCalled();
        const old = find('Open');
        const s = snapshot(); s.revision++; s.full = [['navigate']]; c.publish(s);
        expect(find('Open')).toBeTruthy(); old.click();
        expect(sendKey).toHaveBeenCalledTimes(1);
        c.setReadOnly(true); expect(find('Open').disabled).toBe(true);
        find('Keyboard').click(); expect(keyboard).toHaveBeenCalledTimes(1);
        dispose(); dispose(); c.clear(); expect(parent.children.length).toBe(0); parent.remove();
    });
    it('renders labels as text and collapses only explicit aliases', () => {
        const { c } = controller();
        const s = snapshot(); s.revision++; s.actions[0].label = '<img src=x onerror=alert(1)>';
        s.actions[1].preferred = 1; s.short = ['open', 'navigate']; c.publish(s);
        const parent = document.createElement('div');
        const dispose = installActionBar(c, { parent });
        expect(parent.querySelector('img')).toBeNull();
        expect(parent.textContent).toContain('<img');
        expect(Array.from(parent.querySelectorAll('button')).filter(b => b.textContent?.startsWith('Navigate'))).toHaveLength(1);
        dispose();
    });
});
