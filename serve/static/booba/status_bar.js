import { displayText, ensureStyles } from './host_styles.js';
/**
 * Layout-only defaults in the `booba` cascade layer: a column of records,
 * each a wrapping row, with the progress element on its own line. The
 * `[hidden]` rule lives in the layer so booba's own in-layer `display: flex`
 * on `.booba-status-bar` does not override the browser's default
 * `[hidden] { display: none }`. Hosts that restyle the root's `display`
 * should scope it with `:not([hidden])`.
 */
export const STATUS_BAR_CSS = `@layer booba {
  .booba-status-bar { display: flex; flex-direction: column; gap: 0.25rem; }
  .booba-status-bar[hidden] { display: none; }
  .booba-status { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.5rem; }
  .booba-status__progress { flex-basis: 100%; }
}
`;
const STYLE_ATTRIBUTE = 'data-booba-status-styles';
/** The live install per parent, so a reinstall replaces the bar instead of stacking a second one. */
const installs = new WeakMap();
/**
 * Render a store's records into host-owned DOM; returns an idempotent disposer.
 * Installing again on the same parent disposes the earlier bar first.
 */
export function installStatusBar(store, options) {
    installs.get(options.parent)?.();
    const doc = options.parent.ownerDocument;
    if (options.styles !== false)
        ensureStyles(doc, STYLE_ATTRIBUTE, STATUS_BAR_CSS);
    const root = doc.createElement('div');
    root.className = 'booba-status-bar';
    root.setAttribute('role', 'status');
    root.setAttribute('aria-live', 'polite');
    options.parent.append(root);
    let disposed = false;
    const span = (className, text) => {
        const el = doc.createElement('span');
        el.className = className;
        el.textContent = displayText(text);
        return el;
    };
    const block = (record) => {
        const el = doc.createElement('div');
        el.className = `booba-status booba-status--${record.state}`;
        el.setAttribute('data-state', record.state);
        el.setAttribute('data-id', record.id);
        if (record.effectiveApp)
            el.setAttribute('data-app', record.effectiveApp);
        if (record.state === 'blocked' && record.kind)
            el.setAttribute('data-kind', record.kind);
        if (record.effectiveApp)
            el.append(span('booba-status__app', record.effectiveApp));
        if (record.title)
            el.append(span('booba-status__title', record.title));
        if (record.message)
            el.append(span('booba-status__message', record.message));
        if (record.state === 'working' || record.state === 'blocked') {
            const progress = doc.createElement('progress');
            progress.className = 'booba-status__progress';
            progress.setAttribute('max', '100');
            if (record.progress !== null)
                progress.setAttribute('value', String(record.progress));
            el.append(progress);
        }
        return el;
    };
    // One element per record id. Only a changed signature rebuilds the block, so a
    // ticking percentage updates the progress element alone and a screen reader on
    // the live region is not told the whole record again each tick.
    const entries = new Map();
    const signature = (record) => [record.state, record.kind ?? '', record.effectiveApp, record.title, record.message].join('\u0000');
    const setProgress = (el, progress) => {
        const bar = el.querySelector('progress');
        if (!bar)
            return;
        if (progress !== null)
            bar.setAttribute('value', String(progress));
        else
            bar.removeAttribute('value');
    };
    const render = () => {
        if (disposed)
            return;
        const records = store.records;
        const next = new Map();
        for (const record of records) {
            const sig = signature(record);
            const existing = entries.get(record.id);
            if (existing && existing.signature === sig) {
                setProgress(existing.el, record.progress);
                next.set(record.id, existing);
            }
            else {
                next.set(record.id, { el: block(record), signature: sig });
            }
        }
        entries.clear();
        for (const [id, entry] of next)
            entries.set(id, entry);
        const keep = new Set(Array.from(next.values(), entry => entry.el));
        for (const child of Array.from(root.children))
            if (!keep.has(child))
                child.remove();
        if (records.length === 0)
            root.setAttribute('hidden', '');
        else
            root.removeAttribute('hidden');
        // Appending an existing child moves it, so this orders and inserts in one pass.
        for (const entry of next.values())
            root.append(entry.el);
        options.onLayout?.();
    };
    const unsubscribe = store.subscribe(render);
    render();
    const dispose = () => {
        if (disposed)
            return;
        disposed = true;
        if (installs.get(options.parent) === dispose)
            installs.delete(options.parent);
        unsubscribe();
        root.remove();
        options.onLayout?.();
    };
    installs.set(options.parent, dispose);
    return dispose;
}
