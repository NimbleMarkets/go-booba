import { afterEach, describe, expect, it, vi } from 'vitest';
import { ProgramStatusStore } from './program_status_store.js';
import { STATUS_BAR_CSS, installStatusBar } from './status_bar.js';
import type { ProgramStatusReport } from './program_status.js';

function report(over: Partial<ProgramStatusReport>): ProgramStatusReport {
    return { state: 'idle', kind: null, progress: null, id: '', app: '', title: '', message: '', ...over };
}
const styleElements = () => document.head.querySelectorAll('style[data-booba-status-styles]');

afterEach(() => {
    for (const el of styleElements()) el.remove();
    document.body.replaceChildren();
});

describe('status bar markup contract', () => {
    it('renders one block per record with state, kind, app, title, message, and progress', () => {
        const store = new ProgramStatusStore();
        store.apply(report({ id: '', state: 'idle', app: 'terraform', title: 'Plan', message: 'Ready' }));
        store.apply(report({ id: 'apply', state: 'working', progress: 42, message: 'Applying' }));
        store.apply(report({ id: 'apply/db', state: 'blocked', kind: 'permission', message: 'Drop table?' }));
        store.apply(report({ id: 'lint', state: 'error', message: 'Failed' }));
        const parent = document.createElement('div'); document.body.append(parent);
        const dispose = installStatusBar(store, { parent });
        const root = parent.querySelector('.booba-status-bar')!;
        expect(root.getAttribute('role')).toBe('status');
        expect(root.getAttribute('aria-live')).toBe('polite');
        expect(root.hasAttribute('hidden')).toBe(false);
        const blocks = Array.from(root.querySelectorAll('.booba-status'));
        expect(blocks.map(b => b.getAttribute('data-id'))).toEqual(['', 'apply', 'apply/db', 'lint']);

        const idle = blocks[0];
        expect(idle.classList.contains('booba-status--idle')).toBe(true);
        expect(idle.getAttribute('data-state')).toBe('idle');
        expect(idle.getAttribute('data-app')).toBe('terraform');
        expect(idle.querySelector('.booba-status__app')!.textContent).toBe('terraform');
        expect(idle.querySelector('.booba-status__title')!.textContent).toBe('Plan');
        expect(idle.querySelector('.booba-status__message')!.textContent).toBe('Ready');
        expect(idle.querySelector('.booba-status__progress')).toBeNull();

        const working = blocks[1];
        const progress = working.querySelector('progress.booba-status__progress')!;
        expect(progress.getAttribute('max')).toBe('100');
        expect(progress.getAttribute('value')).toBe('42');
        expect(working.getAttribute('data-app')).toBe('terraform'); // inherited from the root
        expect(working.querySelector('.booba-status__title')).toBeNull();

        const blocked = blocks[2];
        expect(blocked.getAttribute('data-kind')).toBe('permission');
        expect(blocked.querySelector('progress')!.hasAttribute('value')).toBe(false); // indeterminate

        const error = blocks[3];
        expect(error.hasAttribute('data-kind')).toBe(false);
        expect(error.querySelector('progress')).toBeNull();

        for (const el of root.querySelectorAll('*')) expect(el.hasAttribute('style')).toBe(false);
        expect(root.hasAttribute('style')).toBe(false);
        dispose();
        expect(parent.querySelector('.booba-status-bar')).toBeNull();
    });

    it('hides the root when the store is empty and re-renders on changes', () => {
        const store = new ProgramStatusStore();
        const parent = document.createElement('div'); document.body.append(parent);
        const onLayout = vi.fn();
        const dispose = installStatusBar(store, { parent, onLayout });
        const root = parent.querySelector('.booba-status-bar')!;
        expect(root.hasAttribute('hidden')).toBe(true);
        expect(onLayout).toHaveBeenCalledTimes(1);
        store.apply(report({ id: 'job', state: 'done', message: 'Finished' }));
        expect(root.hasAttribute('hidden')).toBe(false);
        expect(root.querySelectorAll('.booba-status')).toHaveLength(1);
        expect(onLayout).toHaveBeenCalledTimes(2);
        store.clear();
        expect(root.hasAttribute('hidden')).toBe(true);
        dispose();
        expect(onLayout).toHaveBeenCalledTimes(4);
        store.apply(report({ id: 'late' }));
        expect(onLayout).toHaveBeenCalledTimes(4); // unsubscribed
    });

    it('strips invisible direction controls and never parses text as markup', () => {
        const store = new ProgramStatusStore();
        store.apply(report({ id: '', state: 'idle', message: 'a\u202eb <b>bold</b>' }));
        const parent = document.createElement('div'); document.body.append(parent);
        installStatusBar(store, { parent });
        const message = parent.querySelector('.booba-status__message')!;
        expect(message.textContent).toBe('ab <b>bold</b>');
        expect(message.querySelector('b')).toBeNull();
    });

    it('injects the layered default stylesheet once per document and lets hosts opt out', () => {
        const store = new ProgramStatusStore();
        const a = document.createElement('div'); const b = document.createElement('div');
        document.body.append(a, b);
        const disposeA = installStatusBar(store, { parent: a });
        const disposeB = installStatusBar(store, { parent: b });
        expect(styleElements()).toHaveLength(1);
        expect(styleElements()[0].textContent).toBe(STATUS_BAR_CSS);
        disposeA(); disposeB();
        for (const el of styleElements()) el.remove();
        const disposeC = installStatusBar(store, { parent: a, styles: false });
        expect(styleElements()).toHaveLength(0);
        disposeC();
        expect(STATUS_BAR_CSS).toContain('@layer booba');
        for (const selector of ['.booba-status-bar', '.booba-status-bar[hidden]', '.booba-status', '.booba-status__progress']) {
            expect(STATUS_BAR_CSS).toContain(selector);
        }
        expect(STATUS_BAR_CSS).toContain('display: none');
    });
});
