import { expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { ActionController } from './actions.js';
import { ActionStatusBridge, actionStatusID, ACTION_MESSAGE_MARKER } from './action_status.js';
import type { ProgramStatusReport } from './program_status.js';

const snapshot = JSON.parse(readFileSync('actions/testdata/snapshots.json', 'utf8'))[0].snapshot;
function setup() {
 const sendKey = vi.fn(() => true);
 const controller = new ActionController({sendKey, canSendKey:()=>true});
 controller.setConnected(true);
 const errors = vi.fn();
 const bridge = new ActionStatusBridge(controller, errors);
 const report: ProgramStatusReport = {state:'idle', id:actionStatusID(snapshot.instance), app:'demo', title:'', kind:null, progress:null, message:'Choose an item'+ACTION_MESSAGE_MARKER+JSON.stringify(snapshot)};
 return {controller, bridge, report, errors, sendKey};
}
it('extracts readable status and activates through ordinary keyboard input only', () => {
 const {controller,bridge,report,sendKey} = setup();
 expect(bridge.receive(report).message).toBe('Choose an item');
 expect(sendKey).not.toHaveBeenCalled();
 expect(controller.capture('open',0)()).toBe(true);
 expect(sendKey).toHaveBeenCalledWith({key:'enter',modifiers:[]});
});
it('requires namespace and matching instance and bounds message length', () => {
 const {controller,bridge,report,errors} = setup();
 bridge.receive({...report,id:'other'});
 expect(controller.snapshot).toBeNull();
 bridge.receive({...report,id:actionStatusID('wrong')});
 bridge.receive({...report,message:report.message+' '.repeat(2048)});
 expect(errors).toHaveBeenCalledTimes(2);
 expect(controller.snapshot).toBeNull();
});
it('clears on same-record replacement, malformed payload, and ancestor clear', () => {
 const {controller,bridge,report} = setup();
 for (const replacement of [
  {...report,message:'ordinary status'},
  {...report,message:'Choose'+ACTION_MESSAGE_MARKER+'{'},
  {...report,state:'clear' as const,id:'booba-actions'},
  {...report,state:'clear' as const,id:''},
 ]) {
  bridge.receive(report);
  const stale = controller.capture('open',0);
  bridge.receive(replacement);
  expect(controller.snapshot).toBeNull();
  expect(stale()).toBe(false);
 }
});
it('unrelated clear does not remove current controls', () => {
 const {controller,bridge,report} = setup();
 bridge.receive(report);
 bridge.receive({...report,state:'clear',id:'other'});
 expect(controller.snapshot).not.toBeNull();
});
it('counts decoded UTF-8 bytes at the exact 2048-byte boundary', () => {
 for (const prefix of ['Status', '地図 🗺']) {
  const {controller,bridge,report,errors} = setup();
  const payload = ACTION_MESSAGE_MARKER + JSON.stringify(snapshot);
  const remaining = 2048 - new TextEncoder().encode(prefix + payload).length;
  const message = prefix + payload + ' '.repeat(remaining);
  expect(new TextEncoder().encode(message).length).toBe(2048);
  bridge.receive({...report,message});
  expect(controller.snapshot).not.toBeNull();
  const stale = controller.capture('open',0);
  bridge.receive({...report,message:message+' '});
  expect(controller.snapshot).toBeNull();
  expect(stale()).toBe(false);
  expect(errors).toHaveBeenCalledTimes(1);
 }
});
