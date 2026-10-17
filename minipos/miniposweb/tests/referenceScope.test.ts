// Authored only; execution deferred by the global forms/test gate.
import {describe,it,expect} from 'vitest';
import {referenceScope} from '../src/storage/referenceScope';
const token=(payload:unknown)=>`header.${btoa(JSON.stringify(payload))}.signature`;
describe('reference cache scope',()=>{
 it('partitions backend tenant and account',()=>{
  const scopes=[referenceScope('/a',token({tenant_id:'one',sub:'a'})),referenceScope('/b',token({tenant_id:'one',sub:'a'})),referenceScope('/a',token({tenant_id:'two',sub:'a'})),referenceScope('/a',token({tenant_id:'one',sub:'b'}))];expect(new Set(scopes).size).toBe(4);expect(scopes.every(Boolean)).toBe(true);
 });
 it('keeps partition stable across refresh tokens',()=>expect(referenceScope('/a',token({tenant_id:'one',sub:'a',exp:1}))).toBe(referenceScope('/a',token({tenant_id:'one',sub:'a',exp:2}))));
 for(const payload of [{},{tenant_id:'one'},{tenant_id:'',sub:'a'},{tenant_id:'one',sub:3},null])it(`rejects missing identity ${JSON.stringify(payload)}`,()=>expect(referenceScope('/a',token(payload))).toBeUndefined());
 it('rejects malformed token',()=>expect(referenceScope('/a','bad')).toBeUndefined());
});
