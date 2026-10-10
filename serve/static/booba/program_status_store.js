/** The protocol's cap on records per terminal. */
export const MAX_PROGRAM_STATUS_RECORDS = 256;
const TRANSIENT = new Set(['working', 'blocked']);
const SETTLED = new Set(['done', 'error']);
/** Order id paths segment by segment; the root (empty id) sorts first. */
export function compareStatusPaths(a, b) {
    const as = a === '' ? [] : a.split('/');
    const bs = b === '' ? [] : b.split('/');
    const n = Math.min(as.length, bs.length);
    for (let i = 0; i < n; i++) {
        if (as[i] !== bs[i])
            return as[i] < bs[i] ? -1 : 1;
    }
    return as.length - bs.length;
}
/**
 * Records keyed by id with the protocol's lifetime rules. Each report
 * replaces its record completely; `clear` removes a subtree; prompts and
 * process exit drop working and blocked records; done and error persist
 * until dismissed.
 */
export class ProgramStatusStore {
    constructor(now = Date.now, onListenersChange) {
        this.now = now;
        this.onListenersChange = onListenersChange;
        this.map = new Map();
        this.listeners = new Set();
        this.cache = null;
    }
    /** Records sorted by id path, root first. */
    get records() {
        if (!this.cache) {
            this.cache = [...this.map.values()]
                .sort((a, b) => compareStatusPaths(a.id, b.id))
                .map(r => ({ ...r, effectiveApp: this.resolveApp(r.id, r.app) }));
        }
        return this.cache;
    }
    get(id) {
        return this.records.find(r => r.id === id);
    }
    get listenerCount() { return this.listeners.size; }
    apply(report) {
        if (report.state === 'clear') {
            const prefix = report.id + '/';
            this.removeWhere(r => report.id === '' || r.id === report.id || r.id.startsWith(prefix));
            return;
        }
        const carriesProgress = TRANSIENT.has(report.state);
        this.map.set(report.id, {
            state: report.state,
            kind: report.state === 'blocked' ? report.kind : null,
            progress: carriesProgress ? report.progress : null,
            id: report.id,
            app: report.app,
            title: report.title,
            message: report.message,
            updatedAt: this.now(),
        });
        if (this.map.size > MAX_PROGRAM_STATUS_RECORDS)
            this.evictOldest(report.id);
        this.changed();
    }
    /** A shell prompt appeared: in-flight work belonged to the previous command. */
    prompt() { this.removeWhere(r => TRANSIENT.has(r.state)); }
    /** The process ended: in-flight work is gone, results stay until dismissed. */
    exit() { this.removeWhere(r => TRANSIENT.has(r.state)); }
    /** The user has seen the results. */
    dismissSettled() { this.removeWhere(r => SETTLED.has(r.state)); }
    clear() { this.removeWhere(() => true); }
    subscribe(listener) {
        this.listeners.add(listener);
        this.onListenersChange?.();
        return () => {
            if (this.listeners.delete(listener))
                this.onListenersChange?.();
        };
    }
    removeWhere(predicate) {
        let removed = false;
        for (const [id, record] of this.map) {
            if (predicate(record)) {
                this.map.delete(id);
                removed = true;
            }
        }
        if (removed)
            this.changed();
    }
    evictOldest(keep) {
        let oldest = null;
        for (const record of this.map.values()) {
            if (record.id !== keep && (!oldest || record.updatedAt < oldest.updatedAt))
                oldest = record;
        }
        if (oldest)
            this.map.delete(oldest.id);
    }
    resolveApp(id, app) {
        if (app)
            return app;
        let path = id;
        while (path !== '') {
            const slash = path.lastIndexOf('/');
            path = slash < 0 ? '' : path.slice(0, slash);
            const parent = this.map.get(path);
            if (parent?.app)
                return parent.app;
        }
        return '';
    }
    changed() {
        this.cache = null;
        for (const listener of [...this.listeners])
            listener();
    }
}
