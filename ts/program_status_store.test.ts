import { describe, expect, it, vi } from 'vitest';
import { MAX_PROGRAM_STATUS_RECORDS, ProgramStatusStore, compareStatusPaths } from './program_status_store.js';
import type { ProgramStatusReport } from './program_status.js';

function report(over: Partial<ProgramStatusReport>): ProgramStatusReport {
    return { state: 'idle', kind: null, progress: null, id: '', app: '', title: '', message: '', ...over };
}

describe('ProgramStatusStore records', () => {
    it('replaces a record completely and sorts by id path with the root first', () => {
        const store = new ProgramStatusStore();
        store.apply(report({ id: 'deploy', state: 'working', message: 'one' }));
        store.apply(report({ id: 'build/test', state: 'idle', title: 'Tests' }));
        store.apply(report({ id: '', state: 'idle', app: 'ci' }));
        store.apply(report({ id: 'build', state: 'done', message: 'Built' }));
        expect(store.records.map(r => r.id)).toEqual(['', 'build', 'build/test', 'deploy']);
        store.apply(report({ id: 'deploy', state: 'error' }));
        const deploy = store.get('deploy')!;
        expect(deploy.state).toBe('error');
        expect(deploy.message).toBe('');
    });

    it('resolves app through the nearest ancestor', () => {
        const store = new ProgramStatusStore();
        store.apply(report({ id: '', app: 'terraform' }));
        store.apply(report({ id: 'plan', app: '' }));
        store.apply(report({ id: 'plan/refresh', app: 'child' }));
        store.apply(report({ id: 'orphan/deep', app: '' }));
        expect(store.get('plan')!.effectiveApp).toBe('terraform');
        expect(store.get('plan/refresh')!.effectiveApp).toBe('child');
        expect(store.get('orphan/deep')!.effectiveApp).toBe('terraform');
        expect(store.get('')!.effectiveApp).toBe('terraform');
    });

    it('normalizes kind and progress to the states that carry them', () => {
        const store = new ProgramStatusStore();
        store.apply(report({ id: 'a', state: 'idle', kind: 'question', progress: 50 }));
        store.apply(report({ id: 'b', state: 'done', progress: 100 }));
        store.apply(report({ id: 'c', state: 'working', kind: 'auth', progress: 10 }));
        store.apply(report({ id: 'd', state: 'blocked', kind: 'permission', progress: 75 }));
        expect(store.get('a')).toMatchObject({ kind: null, progress: null });
        expect(store.get('b')).toMatchObject({ kind: null, progress: null });
        expect(store.get('c')).toMatchObject({ kind: null, progress: 10 });
        expect(store.get('d')).toMatchObject({ kind: 'permission', progress: 75 });
    });
});

describe('ProgramStatusStore lifetimes', () => {
    function populated() {
        const store = new ProgramStatusStore();
        store.apply(report({ id: '', state: 'idle' }));
        store.apply(report({ id: 'build', state: 'working' }));
        store.apply(report({ id: 'build/test', state: 'blocked', kind: 'question' }));
        store.apply(report({ id: 'buildx', state: 'done' }));
        store.apply(report({ id: 'deploy', state: 'error' }));
        return store;
    }

    it('clears a subtree by id and leaves siblings whose id merely starts with the same text', () => {
        const store = populated();
        store.apply(report({ id: 'build', state: 'clear' }));
        expect(store.records.map(r => r.id)).toEqual(['', 'buildx', 'deploy']);
    });

    it('clears everything on an empty-id clear', () => {
        const store = populated();
        store.apply(report({ id: '', state: 'clear' }));
        expect(store.records).toHaveLength(0);
    });

    it('drops only working and blocked at a prompt and on exit', () => {
        const a = populated(); a.prompt();
        expect(a.records.map(r => r.id)).toEqual(['', 'buildx', 'deploy']);
        const b = populated(); b.exit();
        expect(b.records.map(r => r.id)).toEqual(['', 'buildx', 'deploy']);
    });

    it('dismisses only done and error', () => {
        const store = populated(); store.dismissSettled();
        expect(store.records.map(r => r.id)).toEqual(['', 'build', 'build/test']);
    });

    it('evicts the least recently updated record past the cap', () => {
        let t = 0;
        const store = new ProgramStatusStore(() => ++t);
        for (let i = 0; i < MAX_PROGRAM_STATUS_RECORDS; i++) store.apply(report({ id: `r${i}` }));
        store.apply(report({ id: 'r0', message: 'touched' })); // r0 is now the newest
        store.apply(report({ id: 'extra' }));
        expect(store.records).toHaveLength(MAX_PROGRAM_STATUS_RECORDS);
        expect(store.get('r1')).toBeUndefined();
        expect(store.get('r0')).toBeDefined();
        expect(store.get('extra')).toBeDefined();
    });
});

describe('ProgramStatusStore listeners', () => {
    it('fires once per change, not for no-op removals, and reports subscriber changes', () => {
        const onListeners = vi.fn();
        const store = new ProgramStatusStore(Date.now, onListeners);
        const listener = vi.fn();
        const unsubscribe = store.subscribe(listener);
        expect(store.listenerCount).toBe(1);
        expect(onListeners).toHaveBeenCalledTimes(1);
        store.apply(report({ id: 'x', state: 'working' }));
        expect(listener).toHaveBeenCalledTimes(1);
        store.dismissSettled(); // nothing settled: no change
        expect(listener).toHaveBeenCalledTimes(1);
        store.clear();
        expect(listener).toHaveBeenCalledTimes(2);
        store.clear(); // already empty
        expect(listener).toHaveBeenCalledTimes(2);
        unsubscribe();
        expect(store.listenerCount).toBe(0);
        expect(onListeners).toHaveBeenCalledTimes(2);
        store.apply(report({ id: 'y' }));
        expect(listener).toHaveBeenCalledTimes(2);
    });
});

describe('compareStatusPaths', () => {
    it('orders root first, then segment by segment', () => {
        const ids = ['b', 'a/b', 'a', '', 'a/b/c', 'a/a'];
        expect([...ids].sort(compareStatusPaths)).toEqual(['', 'a', 'a/a', 'a/b', 'a/b/c', 'b']);
    });
});
