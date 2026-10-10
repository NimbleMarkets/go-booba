import { ActionController, decodeActionSnapshot, type ActionSnapshot } from './actions.js';
import type { ProgramStatusReport } from './program_status.js';

export const ACTION_STATUS_NAMESPACE = 'booba-actions';
export const ACTION_MESSAGE_MARKER = ' | booba-actions/v1 ';
export const MAX_ACTION_STATUS_MESSAGE_BYTES = 2048;
export function actionStatusID(instance: string): string {
    if (!/^[A-Za-z0-9_.+-]{1,64}$/.test(instance)) throw new Error('Invalid action instance');
    return `${ACTION_STATUS_NAMESPACE}/${instance.match(/.{1,32}/g)!.join('/')}`;
}
export function decodeActionStatus(report: ProgramStatusReport): { snapshot: ActionSnapshot; summary: string } | null {
    if (!report.id.startsWith(`${ACTION_STATUS_NAMESPACE}/`)) return null;
    const offset = report.message.indexOf(ACTION_MESSAGE_MARKER);
    if (offset < 0) return null;
    if (!['idle', 'working', 'blocked'].includes(report.state)) throw new Error('Inactive action status');
    const size = new TextEncoder().encode(report.message).length;
    if (size > MAX_ACTION_STATUS_MESSAGE_BYTES || /[\u0000-\u001f\u007f-\u009f]/u.test(report.message)) throw new Error('Invalid action status message');
    const summary = report.message.slice(0, offset);
    if (!summary || new TextEncoder().encode(summary).length > 256) throw new Error('Invalid action summary');
    const snapshot = decodeActionSnapshot(report.message.slice(offset + ACTION_MESSAGE_MARKER.length));
    if (report.id !== actionStatusID(snapshot.instance)) throw new Error('Action record does not match its instance');
    return { snapshot, summary };
}
/** Applies the private msg convention only when the embedding host opts in. */
export class ActionStatusBridge {
    private owner: string | null = null;
    constructor(private controller: ActionController, private onError?: (error: Error) => void) {}
    clear(): void { this.owner = null; this.controller.clear(); }
    /** Return a human-facing report with the payload removed when recognized. */
    receive(report: ProgramStatusReport): ProgramStatusReport {
        if (report.state === 'clear') {
            if (this.owner && (!report.id || report.id === this.owner || this.owner.startsWith(report.id + '/'))) this.clear();
            return report;
        }
        try {
            const decoded = decodeActionStatus(report);
            if (!decoded) {
                // OSC 7501 replaces records completely: omission removes actions.
                if (report.id === this.owner) this.clear();
                return report;
            }
            this.controller.publish(decoded.snapshot);
            if (this.controller.snapshot?.instance === decoded.snapshot.instance) this.owner = report.id;
            return { ...report, message: decoded.summary };
        } catch (error) {
            if (report.id === this.owner) this.clear();
            this.onError?.(error instanceof Error ? error : new Error(String(error)));
            return report;
        }
    }
}
