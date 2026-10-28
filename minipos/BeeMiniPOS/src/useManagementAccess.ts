import {useLayoutEffect,useRef} from 'react';
export function managementForRoles(production:boolean,roles?:readonly string[]){return !production||!!roles?.some(role=>role==='ADMIN'||role==='SUPERVISOR');}
export function useManagementAccess(production:boolean,roles?:readonly string[],identity=''){
 const keyFor=(next?:readonly string[])=>JSON.stringify([production,identity,[...(next||[])].sort()]);
 const key=keyFor(roles),token={key};
 type Scope={key:string;allowed:boolean};
 const authority=useRef<{observed?:string;mounted:boolean;scope?:Scope;committed?:{token:object;key:string;allowed:boolean}}>({mounted:true});
 // Render ownership protects invocation. Committed auth scope protects already
 // dispatched work across benign renders, and retires on role/session ABA.
 useLayoutEffect(()=>{if(authority.current.observed!==undefined&&authority.current.observed!==key){authority.current.scope=undefined;return;}const allowed=managementForRoles(production,roles);if(authority.current.scope?.key!==key)authority.current.scope={key,allowed};authority.current.committed={token,key,allowed};return()=>{if(authority.current.committed?.token===token)authority.current.committed=undefined;};});
 useLayoutEffect(()=>{authority.current.mounted=true;return()=>{authority.current.mounted=false;authority.current.scope=undefined;authority.current.committed=undefined;};},[]);
 const synchronizeRoles=(next?:readonly string[])=>{const observed=keyFor(next);authority.current.observed=observed;if(authority.current.committed?.key!==observed)authority.current.committed=undefined;if(authority.current.scope?.key!==observed)authority.current.scope=undefined;};
 const scopeGuard=()=>{const scope=authority.current.scope;return()=>Boolean(scope&&authority.current.mounted&&authority.current.scope===scope&&scope.allowed);};
 return {canManage:managementForRoles(production,roles)&&(authority.current.observed===undefined||authority.current.observed===key),synchronizeRoles,scopeGuard,guard:()=>()=>authority.current.committed?.token===token&&authority.current.committed.allowed};
}
