// Written only; runtime execution deferred by the global forms/test gate.
import {describe,it,expect} from 'vitest';
import {persistAuthTokens} from '../src/storage/authTokens';
const tokens={access_token:'new-access',refresh_token:'new-refresh',expires_in:3600};
describe('auth token storage failure recovery',()=>{
 for(const prior of [false,true])it(`restores previous pair after second write failure prior=${prior}`,()=>{
  const values=new Map<string,string>(prior?[['minipos-access-token','old-access'],['minipos-refresh-token','old-refresh']]:[]);
  let rejected=false;
  const storage={getItem:(key:string)=>values.get(key)??null,setItem:(key:string,value:string)=>{if(key==='minipos-refresh-token'&&!rejected){rejected=true;throw Error('denied');}values.set(key,value);},removeItem:(key:string)=>{values.delete(key);}};
  expect(()=>persistAuthTokens(storage,tokens)).toThrow('AUTH_STORAGE_WRITE_FAILED');
  expect([...values]).toEqual(prior?[['minipos-access-token','old-access'],['minipos-refresh-token','old-refresh']]:[]);
 });
 it('reports rollback denial rather than claiming success',()=>{
  const storage={getItem:()=>null,setItem:()=>{throw Error('denied');},removeItem:()=>{throw Error('denied');}};
  expect(()=>persistAuthTokens(storage,tokens)).toThrow('AUTH_STORAGE_ROLLBACK_FAILED');
 });
});

describe('auth response validation before storage',()=>{
 for (const invalid of [null,{}, {...tokens,access_token:' '}, {...tokens,refresh_token:''}, {...tokens,expires_in:0}, {...tokens,expires_in:-1}, {...tokens,expires_in:NaN}, {...tokens,expires_in:Infinity}, {...tokens,expires_in:'3600'}]) {
  it(`rejects invalid session ${JSON.stringify(invalid)} without touching storage`,()=>{
   let reads=0,writes=0;
   const storage={getItem:()=>{reads++;return null;},setItem:()=>{writes++;},removeItem:()=>{writes++;}};
   expect(()=>persistAuthTokens(storage,invalid as typeof tokens)).toThrow('AUTH_RESPONSE_INVALID');
   expect(reads).toBe(0);expect(writes).toBe(0);
  });
 }
});
