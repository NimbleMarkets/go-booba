import type { ProgramStatusRecord, ProgramStatusStore } from './program_status_store.js';
import { displayText, ensureStyles } from './host_styles.js';

export interface StatusBarOptions {
    parent: HTMLElement;
    /** Refit the terminal after the bar changes size. */
    onLayout?: () => void;
    /**
     * Inject the layered default stylesheet (once per document). Default true.
     * Set false under a Content Security Policy that forbids inline styles and
     * serve STATUS_BAR_CSS from your own stylesheet instead.
     */
    styles?: boolean;
}

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

/** Render a store's records into host-owned DOM; returns an idempotent disposer. */
export function installStatusBar(store: ProgramStatusStore, options: StatusBarOptions): () => void {
    const doc = options.parent.ownerDocument;
    if (options.styles !== false) ensureStyles(doc, STYLE_ATTRIBUTE, STATUS_BAR_CSS);
    const root = doc.createElement('div');
    root.className = 'booba-status-bar';
    root.setAttribute('role', 'status');
    root.setAttribute('aria-live', 'polite');
    options.parent.append(root);
    let disposed = false;

    const span = (className: string, text: string): HTMLSpanElement => {
        const el = doc.createElement('span');
        el.className = className;
        el.textContent = displayText(text);
        return el;
    };
    const block = (record: ProgramStatusRecord): HTMLDivElement => {
        const el = doc.createElement('div');
        el.className = `booba-status booba-status--${record.state}`;
        el.setAttribute('data-state', record.state);
        el.setAttribute('data-id', record.id);
        if (record.effectiveApp) el.setAttribute('data-app', record.effectiveApp);
        if (record.state === 'blocked' && record.kind) el.setAttribute('data-kind', record.kind);
        if (record.effectiveApp) el.append(span('booba-status__app', record.effectiveApp));
        if (record.title) el.append(span('booba-status__title', record.title));
        if (record.message) el.append(span('booba-status__message', record.message));
        if (record.state === 'working' || record.state === 'blocked') {
            const progress = doc.createElement('progress');
            progress.className = 'booba-status__progress';
            progress.setAttribute('max', '100');
            if (record.progress !== null) progress.setAttribute('value', String(record.progress));
            el.append(progress);
        }
        return el;
    };
    const render = () => {
        if (disposed) return;
        root.replaceChildren();
        const records = store.records;
        if (records.length === 0) root.setAttribute('hidden', '');
        else root.removeAttribute('hidden');
        for (const record of records) root.append(block(record));
        options.onLayout?.();
    };
    const unsubscribe = store.subscribe(render);
    render();
    return () => {
        if (disposed) return;
        disposed = true;
        unsubscribe();
        root.remove();
        options.onLayout?.();
    };
}
