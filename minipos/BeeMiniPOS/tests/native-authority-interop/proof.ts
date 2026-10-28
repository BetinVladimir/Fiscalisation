// Capture the exact production transcript for signing by the OS test application.
import fs from 'node:fs';
import {NativeExecutionAuthority,executionScopeID} from '../../src/nativeExecutionAuthority.ts';
const [fixturePath,evidencePath,scopePath,messagePath]=process.argv.slice(2);
const fixture=JSON.parse(fs.readFileSync(fixturePath,'utf8'));
const evidence=JSON.parse(fs.readFileSync(evidencePath,'utf8'));
const scope=JSON.parse(fs.readFileSync(scopePath,'utf8'));
const store=new Map<string,string>();
const provider={async prepare(s:string){if(s!==executionScopeID(scope))throw Error('SCOPE_MISMATCH');return evidence.public_key;},async sign(s:string,p:string,message:string){if(s!==executionScopeID(scope)||p!==evidence.public_key)throw Error('KEY_MISMATCH');fs.writeFileSync(messagePath,message);return 'PENDING_OS_SIGNATURE';},async revoke(){throw Error('UNUSED');}};
const service=new NativeExecutionAuthority(provider,{async getItem(k:string){return store.get(k)??null;},async setItem(k:string,v:string){store.set(k,v);},async removeItem(k:string){store.delete(k);}},()=>fixture.now);
const context=await service.accept(scope,fixture.authorization);
fixture.command.client_proof=await service.signIntent(context,fixture.command);
fs.writeFileSync(fixturePath,JSON.stringify(fixture));
