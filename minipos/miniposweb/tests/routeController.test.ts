import { beforeEach, expect, it, vi } from 'vitest';
import type { FiscalIntent, FiscalOperation } from '../src/types';
import type { OutboxRow } from '../src/storage/outbox';
const memory = vi.hoisted(() => ({ rows: new Map<string, any>(), fail: false }));
vi.mock('../src/storage/outbox', () => ({
 prepareOutbox: async (row:OutboxRow) => { if(memory.fail) throw Error('DISK'); const old=memory.rows.get(row.intent.client_operation_id); if(old)return old;memory.rows.set(row.intent.client_operation_id,row);return row; },
 isProvenTerminal: (result:FiscalOperation) => ["COMMITTED","FISCALIZED","REVERSED","PRINTER_TESTED","CANCELLED"].includes(result.state) ? result.certainty==='PROVEN_SUCCESS' : ["FAILED","REJECTED","COMPENSATED"].includes(result.state)&&['PROVEN_FAILURE','NOT_SENT'].includes(result.certainty),
 updateOutboxResult: async (intent:FiscalIntent,result:FiscalOperation) => {const row=memory.rows.get(intent.client_operation_id);memory.rows.set(intent.client_operation_id,{...row,result});return result;},
 getOutbox: async (id:string) => memory.rows.get(id),
 putOutbox: async (row:OutboxRow) => { if(memory.fail) throw Error('DISK'); memory.rows.set(row.intent.client_operation_id, row); },
 claimOutboxAttempt: async (row:OutboxRow, route:string) => { if(memory.fail) throw Error('DISK'); const current=memory.rows.get(row.intent.client_operation_id); if(current?.attempt) return false; memory.rows.set(row.intent.client_operation_id,{...current,...row,attempt:{route,startedAt:new Date().toISOString()}});return true; },
}));
import { RouteController, type FiscalRoute } from '../src/fiscal/routeController';
const intent = { client_operation_id:'one',payload_sha256:'digest' } as FiscalIntent;
const committed = {operation_id:'one',state:'COMMITTED',certainty:'PROVEN_SUCCESS',updated_at:'now'} as FiscalOperation;
const route = (kind:'CLOUD'|'BLE'='CLOUD'):FiscalRoute => ({kind,probe:vi.fn(async()=>true),lookup:vi.fn(async()=>{throw {status:404};}), submit:vi.fn(async()=>committed)});
beforeEach(()=>{memory.rows.clear();memory.fail=false;});
it.each([new Error('timeout'),{status:401},{status:503},{status:404}])('never resubmits uncertain operation across reload: %j', async error=>{
 const cloud=route(),ble=route('BLE'); cloud.submit=vi.fn(async()=>{throw Error('response lost');});
 await expect(new RouteController([cloud,ble]).execute(intent)).rejects.toThrow('UNCERTAIN');
 cloud.lookup=vi.fn(async()=>{throw error;});
 await expect(new RouteController([cloud,ble]).execute(intent)).rejects.toThrow('UNCERTAIN');
 expect(cloud.submit).toHaveBeenCalledTimes(1);expect(ble.submit).not.toHaveBeenCalled();
});
it('fails durable write before any route I/O',async()=>{memory.fail=true;const r=route();await expect(new RouteController([r]).execute(intent)).rejects.toThrow('DISK');expect(r.probe).not.toHaveBeenCalled();expect(r.submit).not.toHaveBeenCalled();});
it('returns terminal replay and rejects digest conflict',async()=>{const r=route();await new RouteController([r]).execute(intent);expect(await new RouteController([r]).execute(intent)).toEqual(committed);await expect(new RouteController([r]).execute({...intent,payload_sha256:'other'})).rejects.toThrow('CONFLICT');expect(r.submit).toHaveBeenCalledTimes(1);});
it('looks up persisted attempt without submit',async()=>{memory.rows.set('one',{intent,attempt:{route:'CLOUD',startedAt:'now'}});const r=route();r.lookup=vi.fn(async()=>committed);expect(await new RouteController([r]).execute(intent)).toEqual(committed);expect(r.submit).not.toHaveBeenCalled();});
it('claims one submit across concurrent controllers',async()=>{const r=route();await Promise.allSettled([new RouteController([r]).execute(intent),new RouteController([r]).execute(intent)]);expect(r.submit).toHaveBeenCalledTimes(1);});
it('treats legacy UNKNOWN as lookup-only',async()=>{memory.rows.set('one',{intent,result:{...committed,state:'UNKNOWN'}});const r=route();await expect(new RouteController([r]).execute(intent)).rejects.toThrow('UNCERTAIN');expect(r.submit).not.toHaveBeenCalled();});
it('commits route marker before submitting',async()=>{const r=route();r.submit=vi.fn(async()=>{expect(memory.rows.get('one').attempt.route).toBe('CLOUD');return committed;});await new RouteController([r]).execute(intent);});
it('can choose another route only before an attempted send',async()=>{const cloud=route(),ble=route('BLE');cloud.probe=vi.fn(async()=>false);await new RouteController([cloud,ble]).execute(intent);expect(cloud.submit).not.toHaveBeenCalled();expect(ble.submit).toHaveBeenCalledTimes(1);});
it('does not reinterpret NOT_SENT text as authorization to retry',async()=>{const r=route();r.submit=vi.fn(async()=>{throw Error('NOT_SENT');});await expect(new RouteController([r]).execute(intent)).rejects.toThrow('UNCERTAIN');r.lookup=vi.fn(async()=>{throw {status:404};});await expect(new RouteController([r]).execute(intent)).rejects.toThrow('UNCERTAIN');expect(r.submit).toHaveBeenCalledTimes(1);});

it('does not send when the durable claim fails after successful probe',async()=>{const r=route();r.probe=vi.fn(async()=>{memory.fail=true;return true;});await expect(new RouteController([r]).execute(intent)).rejects.toThrow('DISK');expect(r.submit).not.toHaveBeenCalled();});
it('preserves pos synchronization metadata on first attempt',async()=>{memory.rows.set('one',{intent,posSync:{shift_id:'shift'},createdAt:'before'});await new RouteController([route()]).execute(intent);expect(memory.rows.get('one').posSync.shift_id).toBe('shift');expect(memory.rows.get('one').createdAt).toBe('before');});
it('restores CLOUD only after the configured healthy probe threshold',async()=>{const cloud=route(),ble=route('BLE');cloud.probe=vi.fn(async()=>false);const controller=new RouteController([cloud,ble],1500,3);await controller.execute({...intent,client_operation_id:'initial'});cloud.probe=vi.fn(async()=>true);await controller.execute({...intent,client_operation_id:'recovery1'});await controller.execute({...intent,client_operation_id:'recovery2'});expect(cloud.submit).not.toHaveBeenCalled();await controller.execute({...intent,client_operation_id:'recovery3'});expect(cloud.submit).toHaveBeenCalledTimes(1);expect(ble.submit).toHaveBeenCalledTimes(3);});
it.each(['PRINTER_TESTED','COMMITTED','FISCALIZED','REVERSED'])('replays proven terminal %s without lookup',async state=>{memory.rows.set('one',{intent,result:{...committed,state}});const r=route();expect((await new RouteController([r]).execute(intent)).state).toBe(state);expect(r.lookup).not.toHaveBeenCalled();expect(r.submit).not.toHaveBeenCalled();});
it('does not treat UNKNOWN certainty as terminal even with successful state',async()=>{memory.rows.set('one',{intent,result:{...committed,certainty:'UNKNOWN'}});const r=route();await expect(new RouteController([r]).execute(intent)).rejects.toThrow('UNCERTAIN');expect(r.lookup).toHaveBeenCalled();expect(r.submit).not.toHaveBeenCalled();});

it('replays compensated proven failure',async()=>{memory.rows.set('one',{intent,result:{...committed,state:'COMPENSATED',certainty:'PROVEN_FAILURE'}});const r=route();expect((await new RouteController([r]).execute(intent)).state).toBe('COMPENSATED');expect(r.lookup).not.toHaveBeenCalled();});
