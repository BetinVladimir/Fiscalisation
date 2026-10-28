import { canonical } from './goCanonical';

export type AuthorizationEnvelope={algorithm:'ES256_DER';key_id:string;payload:string;sha256:string;signature:string};
export type ExecutionAuthorization={kind:'EXECUTION_AUTHORIZATION';version:1;authorization_id:string;workstation_session_id:string;tenant_id:string;location_id:string;register_id:string;edge_device_id:string;fiscal_device_id:string;binding_sha256:string;binding_generation:number;operator_id:string;operator_code:string;actor_subject:string;app_instance_id:string;client_signing_public_key_der_base64:string;scope:'fiscal.execute';allowed_commands:string[];nbf:number;exp:number;authority:{binding_sha256:string;range_start:number;range_end:number;unp_prefix:string};unp_authority?:{source:'SERVER_ASSIGNED';server_sale_id:string;unp:string;payload_sha256:string;operation_id:string;receipt_session_id:string}};
export type ExecutionScope={backend:string;tenant:string;actorSubject:string;appInstance:string;sessionID:string};
export type AuthorizedContext={envelope:AuthorizationEnvelope;claims:ExecutionAuthorization;scope:ExecutionScope;key:CryptoKey;transportToken?:string};
export const base64url=(bytes:Uint8Array)=>btoa(String.fromCharCode(...bytes)).replaceAll('+','-').replaceAll('/','_').replace(/=+$/,'');
export function unbase64url(value:string):Uint8Array {if(!/^[A-Za-z0-9_-]+$/.test(value))throw new Error('AUTHORIZATION_ENCODING_INVALID');return Uint8Array.from(atob(value.replaceAll('-','+').replaceAll('_','/')),c=>c.charCodeAt(0));}
export async function sha256(value:string):Promise<string>{const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(value));return [...new Uint8Array(digest)].map(v=>v.toString(16).padStart(2,'0')).join('');}
export function p1363ToDER(raw:Uint8Array):Uint8Array {
 if(raw.length!==64)throw new Error('CLIENT_SIGNATURE_FORMAT_INVALID');
 const integer=(part:Uint8Array)=>{let i=0;while(i<part.length-1&&part[i]===0)i++;let bytes=part.slice(i);if(bytes[0]&128)bytes=Uint8Array.from([0,...bytes]);return Uint8Array.from([2,bytes.length,...bytes]);};
 const r=integer(raw.slice(0,32)),s=integer(raw.slice(32));return Uint8Array.from([48,r.length+s.length,...r,...s]);
}
export async function inspectAuthorization(envelope:AuthorizationEnvelope,scope:ExecutionScope,publicKey:string,now=Math.floor(Date.now()/1000)):Promise<ExecutionAuthorization>{
 if(envelope.algorithm!=='ES256_DER'||!envelope.key_id||!envelope.signature||!/^[0-9a-f]{64}$/.test(envelope.sha256))throw new Error('EXECUTION_AUTHORIZATION_INVALID');
 const json=new TextDecoder('utf-8',{fatal:true}).decode(unbase64url(envelope.payload));const claims=JSON.parse(json) as ExecutionAuthorization;
 if(canonical(claims)!==json||await sha256(json)!==envelope.sha256||claims.kind!=='EXECUTION_AUTHORIZATION'||claims.version!==1||claims.scope!=='fiscal.execute'||claims.tenant_id!==scope.tenant||claims.actor_subject!==scope.actorSubject||claims.app_instance_id!==scope.appInstance||claims.client_signing_public_key_der_base64!==publicKey||!claims.authorization_id||!claims.workstation_session_id||!claims.operator_id||!/^[A-Za-z0-9]{4}$/.test(claims.operator_code)||!Number.isSafeInteger(claims.binding_generation)||claims.binding_generation<1||!Number.isSafeInteger(claims.nbf)||!Number.isSafeInteger(claims.exp)||claims.exp<=claims.nbf||claims.exp-claims.nbf>900||now<claims.nbf||now>=claims.exp)throw new Error('EXECUTION_AUTHORIZATION_SCOPE_OR_EXPIRY');
 return claims;
}
export async function signIntentProof(context:AuthorizedContext,intent:{intent_id:string;receipt_session_id:string;payload_sha256:string;server_sale_id:string|null;client_sale_surrogate_id:string|null;command:string},now=Math.floor(Date.now()/1000)){
 await assertActive(context);
 const {claims,envelope}=context;
 if(now<claims.nbf||now>=claims.exp||!claims.allowed_commands.includes(intent.command))throw new Error('EXECUTION_AUTHORIZATION_EXPIRED_OR_SCOPE');
 const proof={kind:'EXECUTION_INTENT_PROOF',version:1,authorization_sha256:envelope.sha256,operation_id:intent.intent_id,receipt_session_id:intent.receipt_session_id,payload_sha256:intent.payload_sha256,server_sale_id:intent.server_sale_id,client_sale_surrogate_id:intent.client_sale_surrogate_id,binding_sha256:claims.binding_sha256,binding_generation:claims.binding_generation,command_type:intent.command};
 const transcript=new TextEncoder().encode('BEELOY-EXECUTION-INTENT-PROOF-V1\n'+canonical(proof));
 const raw=await crypto.subtle.sign({name:'ECDSA',hash:'SHA-256'},context.key,transcript);
 await assertActive(context);
 return {algorithm:'ES256_DER' as const,signature:base64url(p1363ToDER(new Uint8Array(raw)))};
}

const dbName='beeloy-minipos-execution-authorization';
const scopeID=(scope:ExecutionScope)=>canonical([scope.backend,scope.tenant,scope.actorSubject,scope.appInstance,scope.sessionID]);
function database():Promise<IDBDatabase>{return new Promise((resolve,reject)=>{const r=indexedDB.open(dbName,1);r.onupgradeneeded=()=>{r.result.createObjectStore('keys');r.result.createObjectStore('authorizations');};r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error);});}
async function read<T>(db:IDBDatabase,store:string,id:string):Promise<T|undefined>{return new Promise((resolve,reject)=>{const tx=db.transaction(store);const r=tx.objectStore(store).get(id);tx.oncomplete=()=>resolve(r.result);tx.onerror=()=>reject(tx.error);tx.onabort=()=>reject(tx.error);});}
export async function browserSigningKey(scope:ExecutionScope):Promise<{privateKey:CryptoKey;publicKey:string}>{
 if(!scope.sessionID)throw new Error('AUTHENTICATED_SESSION_REQUIRED');
 const db=await database(),id=scopeID(scope);try{const saved=await read<{privateKey:CryptoKey;publicKey:string}>(db,'keys',id);if(saved){if((saved as {revoked?:boolean}).revoked)throw new Error('AUTHENTICATED_SESSION_REVOKED');if(saved.privateKey.extractable||saved.privateKey.type!=='private')throw new Error('CLIENT_KEY_UNSAFE');return saved;}
 const pair=await crypto.subtle.generateKey({name:'ECDSA',namedCurve:'P-256'},false,['sign','verify']);const candidate={privateKey:pair.privateKey,publicKey:base64url(new Uint8Array(await crypto.subtle.exportKey('spki',pair.publicKey)))};
 // Read/write transaction makes concurrent tabs converge on the same key.
 return await new Promise((resolve,reject)=>{const tx=db.transaction('keys','readwrite'),store=tx.objectStore('keys'),r=store.get(id);let selected=candidate;r.onsuccess=()=>{if(r.result)selected=r.result;else store.put(candidate,id);};tx.oncomplete=()=>{if((selected as {revoked?:boolean}).revoked)reject(new Error('AUTHENTICATED_SESSION_REVOKED'));else resolve(selected);};tx.onerror=()=>reject(tx.error);tx.onabort=()=>reject(tx.error);});
 }finally{db.close();}
}
async function assertActive(context:AuthorizedContext):Promise<void>{
 const db=await database();try{const saved=await read<{publicKey?:string;revoked?:boolean}>(db,'keys',scopeID(context.scope));if(!saved||saved.revoked||saved.publicKey!==context.claims.client_signing_public_key_der_base64)throw new Error('AUTHENTICATED_SESSION_REVOKED');}finally{db.close();}
}
export async function cacheExecutionAuthorization(scope:ExecutionScope,envelope:AuthorizationEnvelope,transportToken?:string):Promise<AuthorizedContext>{
 const pair=await browserSigningKey(scope),claims=await inspectAuthorization(envelope,scope,pair.publicKey);
 const context={scope,envelope,claims,key:pair.privateKey,transportToken};
 // A ticket for a prepared online sale must never replace the offline lease ticket.
 if(claims.unp_authority){await assertActive(context);return context;}
 const db=await database();try{await new Promise<void>((resolve,reject)=>{
 const tx=db.transaction(['keys','authorizations'],'readwrite'),r=tx.objectStore('keys').get(scopeID(scope));let error:Error|undefined;
 r.onsuccess=()=>{if(!r.result||r.result.revoked||r.result.publicKey!==pair.publicKey){error=new Error('AUTHENTICATED_SESSION_REVOKED');tx.abort();return;}tx.objectStore('authorizations').put({envelope,transportToken},scopeID(scope));};
 tx.oncomplete=()=>resolve();tx.onerror=()=>reject(error||tx.error);tx.onabort=()=>reject(error||tx.error);
 });}finally{db.close();}return context;
}
export async function cachedExecutionAuthorization(scope:ExecutionScope):Promise<AuthorizedContext|undefined>{
 const db=await database();let saved:{envelope:AuthorizationEnvelope;transportToken?:string}|undefined;try{saved=await read(db,'authorizations',scopeID(scope));}finally{db.close();}
 if(!saved)return;const {envelope,transportToken}=saved;try{const pair=await browserSigningKey(scope);const context={scope,envelope,claims:await inspectAuthorization(envelope,scope,pair.publicKey),key:pair.privateKey,transportToken};await assertActive(context);return context;}catch{return undefined;}
}
export async function clearExecutionAuthorization(scope:ExecutionScope):Promise<void>{
 const db=await database();try{await new Promise<void>((resolve,reject)=>{
 const tx=db.transaction(['keys','authorizations'],'readwrite');
 // A persistent tombstone fences a key-generation or issuance request already in flight.
 tx.objectStore('keys').put({revoked:true},scopeID(scope));tx.objectStore('authorizations').delete(scopeID(scope));
 tx.oncomplete=()=>resolve();tx.onerror=()=>reject(tx.error);tx.onabort=()=>reject(tx.error);
 });}finally{db.close();}
}
