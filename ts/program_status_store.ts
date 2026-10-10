import type { ProgramStatusReport } from './program_status.js';

/** A program status record's state; `clear` is a command, not a state. */
export type ProgramStatusState = Exclude<ProgramStatusReport['state'], 'clear'>;

/** One Program Status Protocol (OSC 7501) record as the store keeps it. */
export interface ProgramStatusRecord {
    state: ProgramStatusState;
    /** Only `blocked` records carry a kind. */
    kind: ProgramStatusReport['kind'];
    /** Percentage for `working` and `blocked`; null means indeterminate or not applicable. */
    progress: number | null;
    id: string;
    /** The app the report named, possibly empty. */
    app: string;
    /** The app resolved through ancestor records when this record names none. */
    effectiveApp: string;
    title: string;
    message: string;
    updatedAt: number;
}

/** The protocol's cap on records per terminal. */
export const MAX_PROGRAM_STATUS_RECORDS = 256;

type Stored = Omit<ProgramStatusRecord, 'effectiveApp'>;

const TRANSIENT: ReadonlySet<string> = new Set(['working', 'blocked']);
const SETTLED: ReadonlySet<string> = new Set(['done', 'error']);

/** Order id paths segment by segment; the root (empty id) sorts first. */
export function compareStatusPaths(a: string, b: string): number {
    const as = a === '' ? [] : a.split('/');
    const bs = b === '' ? [] : b.split('/');
    const n = Math.min(as.length, bs.length);
    for (let i = 0; i < n; i++) {
        if (as[i] !== bs[i]) return as[i] < bs[i] ? -1 : 1;
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
    private readonly map = new Map<string, Stored>();
    private readonly listeners = new Set<() => void>();
    private cache: readonly ProgramStatusRecord[] | null = null;

    constructor(private readonly now: () => number = Date.now, private readonly onListenersChange?: () => void) {}

    /** Records sorted by id path, root first. */
    get records(): readonly ProgramStatusRecord[] {
        if (!this.cache) {
            this.cache = [...this.map.values()]
                .sort((a, b) => compareStatusPaths(a.id, b.id))
                .map(r => ({ ...r, effectiveApp: this.resolveApp(r.id, r.app) }));
        }
        return this.cache;
    }

    get(id: string): ProgramStatusRecord | undefined {
        return this.records.find(r => r.id === id);
    }

    get listenerCount(): number { return this.listeners.size; }

    apply(report: ProgramStatusReport): void {
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
        if (this.map.size > MAX_PROGRAM_STATUS_RECORDS) this.evictOldest(report.id);
        this.changed();
    }

    /** A shell prompt appeared: in-flight work belonged to the previous command. */
    prompt(): void { this.removeWhere(r => TRANSIENT.has(r.state)); }

    /** The process ended: in-flight work is gone, results stay until dismissed. */
    exit(): void { this.removeWhere(r => TRANSIENT.has(r.state)); }

    /** The user has seen the results. */
    dismissSettled(): void { this.removeWhere(r => SETTLED.has(r.state)); }

    clear(): void { this.removeWhere(() => true); }

    subscribe(listener: () => void): () => void {
        this.listeners.add(listener);
        this.onListenersChange?.();
        return () => {
            if (this.listeners.delete(listener)) this.onListenersChange?.();
        };
    }

    private removeWhere(predicate: (record: Stored) => boolean): void {
        let removed = false;
        for (const [id, record] of this.map) {
            if (predicate(record)) { this.map.delete(id); removed = true; }
        }
        if (removed) this.changed();
    }

    private evictOldest(keep: string): void {
        let oldest: Stored | null = null;
        for (const record of this.map.values()) {
            if (record.id !== keep && (!oldest || record.updatedAt < oldest.updatedAt)) oldest = record;
        }
        if (oldest) this.map.delete(oldest.id);
    }

    private resolveApp(id: string, app: string): string {
        if (app) return app;
        let path = id;
        while (path !== '') {
            const slash = path.lastIndexOf('/');
            path = slash < 0 ? '' : path.slice(0, slash);
            const parent = this.map.get(path);
            if (parent?.app) return parent.app;
        }
        return '';
    }

    private changed(): void {
        this.cache = null;
        for (const listener of [...this.listeners]) listener();
    }
}
