// Authored only; execution deferred by global forms/test gate.
import {describe,it,expect} from 'vitest';
import {belongsToOutboxScope} from '../src/storage/outboxScope';
const scope={tenant:'tenant',location:'location',register:'register',operator:'001'};
const intent={tenant_id:'tenant',location_id:'location',register_id:'register',operator_code:'001'};
describe('outbox ownership',()=>{
 it('accepts matching persisted intent',()=>expect(belongsToOutboxScope({intent},scope)).toBe(true));
 for(const key of ['tenant_id','location_id','register_id','operator_code'])it(`rejects foreign ${key}`,()=>expect(belongsToOutboxScope({intent:{...intent,[key]:'foreign'}},scope)).toBe(false));
 for(const row of [null,{}, {intent:null},{intent:'legacy'}])it(`rejects malformed ${JSON.stringify(row)}`,()=>expect(belongsToOutboxScope(row,scope)).toBe(false));
 it('rejects missing active scope',()=>expect(belongsToOutboxScope({intent},{...scope,tenant:''})).toBe(false));
});
