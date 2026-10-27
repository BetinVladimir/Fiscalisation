import { demoEmailOtpEnabled, normalizeEmailOtp, validEmailOtp } from './emailOtpCode';
import { useEffect, useMemo, useRef, useState } from "react";
import * as Crypto from "expo-crypto";
import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";
import { useUiPreferences } from './ui/preferences';

type Tenant = { tenant_id: string; display_name: string; roles: string[] };
type Session = { access_token: string; refresh_token: string; expires_in: number; tenant: Tenant };
const apiRoot = (process.env.EXPO_PUBLIC_FISCAL_API_URL || "http://localhost:8080/public/v1").replace(/\/public\/v1\/?$/, "");
const sessionKey="beefiscal.app.session",instanceKey="beefiscal.app.instance";
const getStored=async(k:string)=>Platform.OS==="web"?(k===sessionKey?null:(globalThis.localStorage?.getItem(k)||null)):SecureStore.getItemAsync(k);
const setStored=async(k:string,v:string)=>{if(Platform.OS==="web"){if(k!==sessionKey)globalThis.localStorage?.setItem(k,v)}else await SecureStore.setItemAsync(k,v)};
const removeStored=async(k:string)=>{if(Platform.OS==="web"){if(k!==sessionKey)globalThis.localStorage?.removeItem(k)}else await SecureStore.deleteItemAsync(k)};

export function useEmailOtpAuth() {
  const { language } = useUiPreferences();
  const [email,setEmail]=useState(""); const [code,setCode]=useState("");
  const [temporary,setTemporary]=useState(""); const [selection,setSelection]=useState("");
  const [tenants,setTenants]=useState<Tenant[]>([]); const [session,setSession]=useState<Session|null>(null);
  const [error,setError]=useState(""); const [busy,setBusy]=useState(false),[booting,setBooting]=useState(true),[instanceId,setInstanceId]=useState("");
  const call=async(path:string,body:unknown,token="")=>{const r=await fetch(apiRoot+path,{method:"POST",headers:{"Content-Type":"application/json",...(token?{Authorization:`Bearer ${token}`}:{})},body:JSON.stringify(body)});const text=await r.text();if(!r.ok)throw new Error(`HTTP ${r.status}`);return text?JSON.parse(text):{}};
  // Authority survives renders, but is revoked synchronously by logout/unmount.
  const mounted=useRef(false),epoch=useRef(0),pending=useRef<object|null>(null);
  const expiresAt=useRef(0);
  const storageTail=useRef<Promise<void>>(Promise.resolve());
  const snapshot={email,code,temporary,selection,session,tenants,instanceId,booting,language};
  const latest=useRef(snapshot);latest.current=snapshot;
  const current=()=>mounted.current&&latest.current===snapshot;
  const live=(owner:number)=>mounted.current&&epoch.current===owner;
  const serializeStorage=(job:()=>Promise<void>)=>{
    const next=storageTail.current.then(job,job);
    storageTail.current=next.catch(()=>{});return next;
  };
  const validTenant=(value:unknown):value is Tenant=>{
    const v=value as Tenant|undefined;
    return !!v&&typeof v.tenant_id==='string'&&!!v.tenant_id.trim()&&typeof v.display_name==='string'&&!!v.display_name.trim()&&Array.isArray(v.roles)&&v.roles.every(role=>typeof role==='string');
  };
  const validTenants=(value:unknown):value is Tenant[]=>Array.isArray(value)&&value.every(validTenant)&&new Set(value.map(v=>v.tenant_id)).size===value.length;
  const validSession=(value:unknown):value is Session=>{
    const v=value as Session|undefined;
    return !!v&&typeof v.access_token==='string'&&!!v.access_token.trim()&&typeof v.refresh_token==='string'&&!!v.refresh_token.trim()&&Number.isFinite(v.expires_in)&&v.expires_in>0&&validTenant(v.tenant);
  };
  const loadTenants=async(accessToken:string,owner:number)=>{
    const r=await fetch(apiRoot+"/public/v1/app-auth/tenants",{headers:{Authorization:`Bearer ${accessToken}`}});
    if(!r.ok)throw new Error(`HTTP ${r.status}`);
    const out=await r.json();if(!validTenants(out.items))throw new Error('INVALID_TENANTS');
    if(live(owner))setTenants(out.items);
  };
  const keepSession=async(out:unknown,owner:number)=>{
    if(!validSession(out))throw new Error('INVALID_SESSION');
    if(!live(owner))return;
    await serializeStorage(async()=>{if(live(owner))await setStored(sessionKey,JSON.stringify({...out,expires_at:Date.now()+out.expires_in*1000}));});
    if(!live(owner))return;
    expiresAt.current=Date.now()+out.expires_in*1000;
    setSession(out);setTenants([out.tenant]);
    try{await loadTenants(out.access_token,owner)}catch(e){if(live(owner))setError(e instanceof Error?e.message:String(e))}
  };
  const run=async(job:(owner:number)=>Promise<void>)=>{
    if(!current()||booting||!instanceId||pending.current)return;
    const ticket={},owner=epoch.current;pending.current=ticket;setBusy(true);setError('');
    try{await job(owner)}catch(e){if(live(owner))setError(e instanceof Error?e.message:String(e))}
    finally{if(pending.current===ticket){pending.current=null;if(live(owner))setBusy(false)}}
  };
  const requestCode=async()=>{
    if(temporary||session||! /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()))return;
    await run(async owner=>{const out=await call("/public/v1/app-auth/challenges",{email:email.trim(),language,app_instance_id:instanceId});
      if(typeof out.temporary_token!=='string'||!out.temporary_token.trim())throw new Error('INVALID_CHALLENGE');
      if(live(owner)){setTemporary(out.temporary_token);setCode('');setSelection('');setTenants([])}
    });
  };
  const verifyCode=async()=>{
    if(!temporary||session||selection||!validEmailOtp(code,demoEmailOtpEnabled()))return;
    await run(async owner=>{const out=await call("/public/v1/app-auth/challenges:verify",{code,app_instance_id:instanceId},temporary);
      if(out.session)await keepSession(out.session,owner);
      else{if(typeof out.tenant_selection_token!=='string'||!out.tenant_selection_token.trim()||!validTenants(out.tenants)||!out.tenants.length)throw new Error('INVALID_TENANT_SELECTION');
        if(live(owner)){setSelection(out.tenant_selection_token);setTenants(out.tenants)}
      }
    });
  };
  const selectTenant=async(tenantId:string)=>{
    if(!selection||session||!tenants.some(t=>t.tenant_id===tenantId))return;
    await run(async owner=>{const out=await call("/public/v1/app-auth/tenant-session",{tenant_id:tenantId,app_instance_id:instanceId},selection);
      if(!validSession(out)||out.tenant.tenant_id!==tenantId)throw new Error('INVALID_SESSION');await keepSession(out,owner);
    });
  };
  const switchTenant=async(tenantId:string)=>{
    if(!session||!tenants.some(t=>t.tenant_id===tenantId)||tenantId===session.tenant.tenant_id)return;
    await run(async owner=>{const out=await call("/public/v1/app-auth/sessions:switch-tenant",{refresh_token:session.refresh_token,tenant_id:tenantId,app_instance_id:instanceId});
      if(!validSession(out)||out.tenant.tenant_id!==tenantId)throw new Error('INVALID_SESSION');await keepSession(out,owner);
    });
  };
  const logout=async()=>{
    if(!current())return;
    const previous=session;epoch.current++;pending.current=null;setBusy(false);setSession(null);setTenants([]);setTemporary('');setSelection('');setCode('');setError('');
    await serializeStorage(()=>removeStored(sessionKey));
    if(previous)try{await call("/public/v1/app-auth/logout",{refresh_token:previous.refresh_token,app_instance_id:instanceId})}catch{/* local authority is already cleared */}
  };
  const refresh=async()=>{
    if(!session)return;
    await run(async owner=>{
      try{await keepSession(await call("/public/v1/app-auth/refresh",{refresh_token:session.refresh_token,app_instance_id:instanceId}),owner)}
      catch(e){if(live(owner)){epoch.current++;pending.current=null;setBusy(false);setSession(null);setTenants([]);setTemporary('');setSelection('');setCode('');setError(e instanceof Error?e.message:String(e));await serializeStorage(()=>removeStored(sessionKey))}}
    });
  };
  const refreshRef=useRef(refresh);refreshRef.current=refresh;
  useEffect(()=>{
    mounted.current=true;const owner=epoch.current;
    void(async()=>{try{
      let id=await getStored(instanceKey);if(!live(owner))return;
      if(!id){id=Crypto.randomUUID();await serializeStorage(async()=>{if(live(owner))await setStored(instanceKey,id!)})}
      if(!live(owner))return;setInstanceId(id);
      const raw=await getStored(sessionKey);if(!live(owner))return;
      if(raw){const saved=JSON.parse(raw);if(typeof saved.refresh_token!=='string'||!saved.refresh_token.trim())throw new Error('INVALID_SAVED_SESSION');
        await keepSession(await call("/public/v1/app-auth/refresh",{refresh_token:saved.refresh_token,app_instance_id:id}),owner)}
    }catch{if(live(owner))await serializeStorage(()=>removeStored(sessionKey))}
    finally{if(live(owner))setBooting(false)}})();
    return()=>{mounted.current=false;epoch.current++;pending.current=null};
  },[]);
  useEffect(()=>{
    if(!session||!instanceId)return;
    // Re-schedule when another command releases the gate; refresh is never concurrent.
    if(busy)return;
    const timer=setTimeout(()=>{void refreshRef.current()},Math.max(1000,expiresAt.current-Date.now()-60000));
    return()=>clearTimeout(timer);
  },[session,instanceId,busy]);
  const editEmail=(value:string)=>{if(current()&&!booting&&!pending.current&&!temporary&&!session){setEmail(value);setError('')}};
  const editCode=(value:string)=>{if(current()&&!booting&&!pending.current&&temporary&&!selection&&!session){setCode(normalizeEmailOtp(value,demoEmailOtpEnabled()));setError('')}};
  const roles=useMemo(()=>session?.tenant.roles||[],[session]);
  return {configured:true,ready:!busy&&!booting,accessToken:session?.access_token||"",roles,error,email,setEmail:editEmail,code,setCode:editCode,temporary,tenants,currentTenant:session?.tenant,requestCode,verifyCode,selectTenant,switchTenant,logout};
}
