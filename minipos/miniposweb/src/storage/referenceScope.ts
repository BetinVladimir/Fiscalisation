/** Cache partition only; JWT trust/authorization remains the server's responsibility. */
export function referenceScope(backend:string,token:string):string|undefined {
 try {
  const payload=JSON.parse(atob(token.split('.')[1].replaceAll('-','+').replaceAll('_','/')));
  if(typeof payload.tenant_id!=='string'||!payload.tenant_id.trim()||typeof payload.sub!=='string'||!payload.sub.trim())return undefined;
  return JSON.stringify([backend,payload.tenant_id,payload.sub]);
 }catch{return undefined;}
}
