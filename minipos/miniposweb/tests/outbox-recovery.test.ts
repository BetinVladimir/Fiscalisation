// Authored only. Do not execute before the global forms/test gate.
import {afterEach,describe,it,expect,vi} from 'vitest';
afterEach(()=>{vi.unstubAllGlobals();vi.resetModules();});
describe('outbox database recovery',()=>{
 it('releases rejected open cache so retry opens again',async()=>{
  const failure=new Error('temporary denied');let opened=0;
  vi.stubGlobal('indexedDB',{open:()=>{opened++;const request:{error:Error;onerror?:()=>void}={error:failure};queueMicrotask(()=>request.onerror?.());return request;}});
  const {listOutbox}=await import('../src/storage/outbox');
  await expect(listOutbox()).rejects.toBe(failure);
  await expect(listOutbox()).rejects.toBe(failure);
  expect(opened).toBe(2);
 });
 it('rejects aborted read instead of returning an uncommitted request result',async()=>{
  const tx:{error:null;onabort?:()=>void;objectStore:()=>unknown}={error:null,objectStore:()=>({getAll:()=>{queueMicrotask(()=>tx.onabort?.());return {result:[{uncommitted:true}]};}})};
  const db={addEventListener:vi.fn(),transaction:()=>tx};
  vi.stubGlobal('indexedDB',{open:()=>{const request:{result:typeof db;onsuccess?:()=>void}={result:db};queueMicrotask(()=>request.onsuccess?.());return request;}});
  const {listOutbox}=await import('../src/storage/outbox');
  await expect(listOutbox()).rejects.toThrow('OUTBOX_TRANSACTION_ABORTED');
 });
});
