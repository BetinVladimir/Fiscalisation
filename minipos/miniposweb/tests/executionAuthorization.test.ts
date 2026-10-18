import { describe,it,expect } from 'vitest';
import {buildIntent} from '../src/fiscal/intentBuilder';
describe('execution authorization required before local money IO',()=>{
 it('rejects sale intent without a root-issued caller-bound authorization',async()=>{
  await expect(buildIntent({tenantId:'tenant',locationId:'location',registerId:'register',edgeDeviceId:'edge',bindingGeneration:1,operatorCode:'A001',command:'SALE_FINALIZE',surrogateId:'offline'})).rejects.toThrow('EXECUTION_AUTHORIZATION_REQUIRED');
 });
});
