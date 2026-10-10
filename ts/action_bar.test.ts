import { afterEach, describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { ActionController } from './actions.js';
import { ACTION_BAR_CSS, installActionBar } from './action_bar.js';

const fixtures = JSON.parse(readFileSync('actions/testdata/snapshots.json', 'utf8'));
function controller() {
    const c = new ActionController({ sendKey: vi.fn(() => true), canSendKey: () => true });
    c.setConnected(true);
    c.publish(structuredClone(fixtures[0].snapshot));
    return c;
}
const styleElements = () => document.head.querySelectorAll('style[data-booba-styles]');

afterEach(() => {
    for (const el of styleElements()) el.remove();
    document.body.replaceChildren();
});

describe('action bar styling contract', () => {
    it('applies stable classes to every element and never inline styles', () => {
        const parent = document.createElement('div'); document.body.append(parent);
        const dispose = installActionBar(controller(), { parent, openKeyboard: () => {} });
        const root = parent.querySelector('.booba-action-bar')!;
        expect(root).toBeTruthy();
        const groups = root.querySelectorAll('.booba-action-group');
        expect(groups.length).toBeGreaterThan(0);
        const buttons = Array.from(root.querySelectorAll('button'));
        expect(buttons.length).toBeGreaterThan(2);
        for (const b of buttons) expect(b.classList.contains('booba-action')).toBe(true);
        for (const el of root.querySelectorAll('*')) expect(el.hasAttribute('style')).toBe(false);
        expect(root.hasAttribute('style')).toBe(false);
        const toggle = buttons.find(b => b.textContent === 'More')!;
        expect(toggle.classList.contains('booba-action--toggle')).toBe(true);
        const keyboard = buttons.find(b => b.textContent === 'Keyboard')!;
        expect(keyboard.classList.contains('booba-action--keyboard')).toBe(true);
        toggle.click();
        expect(parent.querySelector('.booba-action--toggle')!.getAttribute('aria-expanded')).toBe('true');
        dispose();
    });

    it('injects the layered default stylesheet once per document', () => {
        const a = document.createElement('div'); const b = document.createElement('div');
        document.body.append(a, b);
        const disposeA = installActionBar(controller(), { parent: a });
        const disposeB = installActionBar(controller(), { parent: b });
        expect(styleElements()).toHaveLength(1);
        disposeA(); disposeB();
        const disposeC = installActionBar(controller(), { parent: a });
        expect(styleElements()).toHaveLength(1);
        expect(styleElements()[0].textContent).toBe(ACTION_BAR_CSS);
        disposeC();
    });

    it('skips injection when styles is false, and exports the CSS for hosts to serve', () => {
        const parent = document.createElement('div'); document.body.append(parent);
        const dispose = installActionBar(controller(), { parent, styles: false });
        expect(styleElements()).toHaveLength(0);
        dispose();
        expect(ACTION_BAR_CSS).toContain('@layer booba');
        for (const selector of ['.booba-action-bar', '.booba-action-group', '.booba-action']) {
            expect(ACTION_BAR_CSS).toContain(selector);
        }
        expect(ACTION_BAR_CSS).toContain('min-height: 44px');
        expect(ACTION_BAR_CSS).toContain('touch-action: manipulation');
    });
});
