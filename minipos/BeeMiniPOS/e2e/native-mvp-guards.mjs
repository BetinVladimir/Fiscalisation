import path from 'node:path';
import {realpath,readFile,mkdir,open,unlink,rm} from 'node:fs/promises';
export async function ownedPath(value,{existing=true}={}){
 if(typeof value!=='string'||!path.isAbsolute(value)||path.normalize(value)!==value||!value.startsWith('/private/tmp/'))throw Error('Normalized absolute /private/tmp path required');
 const resolved=await realpath(existing?value:path.dirname(value));
 if(!resolved.startsWith('/private/tmp/'))throw Error('Owned path escapes /private/tmp');
 return value;
}
export async function verifySnapshot(directory,snapshot,appRelative){
 await ownedPath(directory);
 const expected=path.join(directory,appRelative);
 if(snapshot.status!=='PREPARED_NOT_BUILT'||snapshot.root!==expected||await realpath(expected)!==expected)throw Error('Snapshot root outside owned prepared directory');
 await verifyEntrypoint(expected);
 return expected;
}
export async function verifyEntrypoint(root){
 const pkg=JSON.parse(await readFile(path.join(root,'package.json'),'utf8'));
 if(pkg.main!=='node_modules/expo/AppEntry.js')throw Error('Full application Expo entrypoint required');
 await readFile(path.join(root,'App.tsx'));
}
export function buildEnvironment(input,directory){
 const env={};for(const key of ['PATH','ANDROID_SDK_ROOT','ANDROID_HOME','JAVA_HOME','LANG','LC_ALL','SYSTEMROOT'])if(input[key])env[key]=input[key];
 return {...env,HOME:path.join(directory,'build-home'),GRADLE_USER_HOME:path.join(directory,'gradle-cache'),TMPDIR:path.join(directory,'cache'),CI:'1',EXPO_NO_DOTENV:'1',EXPO_NO_TELEMETRY:'1'};
}
export async function acquireEvidenceLease(out,device){
 await ownedPath(out,{existing:false});
 // Reserve evidence first: failure must not acquire/leak the device lock.
 await mkdir(out);
 const file='/private/tmp/beeloy-native-mvp-'+device+'.lock';let handle;
 try{handle=await open(file,'wx')}catch(error){await rm(out,{recursive:true});throw error}
 let released=false;
 return async()=>{if(released)return;released=true;try{await handle.close()}finally{await unlink(file)}};
}
export function evidenceSafe(value){
 if(Array.isArray(value))return value.map(evidenceSafe);
 if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).map(([key,item])=>[key,/^(authorization|cookie|set-cookie|access_token|refresh_token|temporary_token|selection_token|private_key|signature)$/i.test(key)?'[REDACTED]':evidenceSafe(item)]));
 return value;
}
export function verifyLoopbackOrigin(value){const url=new URL(value);if(url.protocol!=='http:'||url.hostname!=='127.0.0.1'||!url.port||url.username||url.password||url.pathname!=='/'||url.search||url.hash||value!==url.origin)throw Error('Exact loopback origin required');return value;}
export async function runBounded(executable,args,options={},timeoutMs=900000){
 const {spawn}=await import('node:child_process');
 if(!Number.isInteger(timeoutMs)||timeoutMs<1)throw Error('Positive command timeout required');
 const child=spawn(executable,args,{...options,detached:process.platform!=='win32'});
 let timedOut=false;
 const timer=setTimeout(()=>{timedOut=true;try{process.kill(process.platform==='win32'?child.pid:-child.pid,'SIGKILL')}catch(error){if(error.code!=='ESRCH')child.kill('SIGKILL')}},timeoutMs);
 try{const code=await new Promise((resolve,reject)=>{child.once('error',reject);child.once('close',resolve)});if(timedOut)throw Error('Native harness command timed out');return code}finally{clearTimeout(timer)}
}
