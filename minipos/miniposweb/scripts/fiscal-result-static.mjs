import path from'node:path';import{realpath,stat}from'node:fs/promises';
/** Static verified production artifact; root resolves only to its own index. No SPA wildcard. */
export async function resolveResultAsset(root,request){
 if(typeof request!=='string'||!request.startsWith('/')||request.startsWith('//')||/[?#]/.test(request)||/%(?:2f|5c|25)/i.test(request))throw Error('Invalid asset request');
 const decoded=decodeURIComponent(request);if(decoded.includes('\\')||decoded.includes('\0')||decoded.split('/').some(part=>part==='..'||part==='.')||decoded.includes('//'))throw Error('Traversal');
 const base=await realpath(root),relative=decoded==='/'?'index.html':decoded.slice(1),candidate=path.resolve(base,relative);
 if(!candidate.startsWith(base+path.sep))throw Error('Containment');
 const target=await realpath(candidate);if(!target.startsWith(base+path.sep)||!(await stat(target)).isFile())throw Error('Symlink or nonfile');
 return target;
}
