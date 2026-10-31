/** OSC 7501 report shape exposed by ntwebterm's osc-7501 branch.
 * Remove this compatibility declaration when that API reaches the pinned build.
 * Consumers own status-record replacement and lifetime policy.
 */
export interface ProgramStatusReport {
    state: 'idle' | 'working' | 'done' | 'blocked' | 'error' | 'clear';
    kind: 'permission' | 'question' | 'auth' | null;
    progress: number | null;
    id: string;
    app: string;
    title: string;
    message: string;
}
export interface ProgramStatusSource {
    onProgramStatus?: (listener: (report: ProgramStatusReport) => void) => { dispose(): void };
}
