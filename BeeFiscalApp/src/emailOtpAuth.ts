import { useEffect, useMemo, useState } from "react";
import * as Crypto from "expo-crypto";
import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

type Tenant = { tenant_id: string; display_name: string; roles: string[] };
type Session = { access_token: string; refresh_token: string; expires_in: number; tenant: Tenant };
const apiRoot = (process.env.EXPO_PUBLIC_FISCAL_API_URL || "http://localhost:8080/public/v1").replace(/\/public\/v1\/?$/, "");
const sessionKey="beefiscal.app.session",instanceKey="beefiscal.app.instance";
const getStored=async(k:string)=>Platform.OS==="web"?(k===sessionKey?null:(globalThis.localStorage?.getItem(k)||null)):SecureStore.getItemAsync(k);
const setStored=async(k:string,v:string)=>{if(Platform.OS==="web"){if(k!==sessionKey)globalThis.localStorage?.setItem(k,v)}else await SecureStore.setItemAsync(k,v)};
const removeStored=async(k:string)=>{if(Platform.OS==="web"){if(k!==sessionKey)globalThis.localStorage?.removeItem(k)}else await SecureStore.deleteItemAsync(k)};

export function useEmailOtpAuth() {
  const [email,setEmail]=useState(""); const [code,setCode]=useState("");
  const [temporary,setTemporary]=useState(""); const [selection,setSelection]=useState("");
  const [tenants,setTenants]=useState<Tenant[]>([]); const [session,setSession]=useState<Session|null>(null);
  const [error,setError]=useState(""); const [busy,setBusy]=useState(false),[booting,setBooting]=useState(true),[instanceId,setInstanceId]=useState("");
  const call=async(path:string,body:unknown,token="")=>{const r=await fetch(apiRoot+path,{method:"POST",headers:{"Content-Type":"application/json",...(token?{Authorization:`Bearer ${token}`}:{})},body:JSON.stringify(body)});const text=await r.text();if(!r.ok)throw new Error(text||`HTTP ${r.status}`);return text?JSON.parse(text):{}};
  const requestCode=async()=>{setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/challenges",{email,language:"bg",app_instance_id:instanceId});setTemporary(out.temporary_token);setCode("");}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const keepSession=async(out:Session)=>{setSession(out);await setStored(sessionKey,JSON.stringify({...out,expires_at:Date.now()+out.expires_in*1000}));await loadTenants(out.access_token)};
  const verifyCode=async()=>{setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/challenges:verify",{code,app_instance_id:instanceId},temporary);if(out.session)await keepSession(out.session);else{setSelection(out.tenant_selection_token);setTenants(out.tenants||[])}}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const loadTenants=async(accessToken:string)=>{const r=await fetch(apiRoot+"/public/v1/app-auth/tenants",{headers:{Authorization:`Bearer ${accessToken}`}});if(r.ok){const out=await r.json();setTenants(out.items||[])}};
  const selectTenant=async(tenantId:string)=>{setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/tenant-session",{tenant_id:tenantId,app_instance_id:instanceId},selection);await keepSession(out)}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const switchTenant=async(tenantId:string)=>{if(!session)return;setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/sessions:switch-tenant",{refresh_token:session.refresh_token,tenant_id:tenantId,app_instance_id:instanceId});await keepSession(out)}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const logout=async()=>{const current=session;setSession(null);setTenants([]);setTemporary("");setSelection("");setCode("");setError("");await removeStored(sessionKey);if(current)try{await call("/public/v1/app-auth/logout",{refresh_token:current.refresh_token,app_instance_id:instanceId})}catch{/* local authority is already cleared */}};
  useEffect(()=>{void(async()=>{try{let id=await getStored(instanceKey);if(!id){id=Crypto.randomUUID();await setStored(instanceKey,id)}setInstanceId(id);const raw=await getStored(sessionKey);if(raw){const saved=JSON.parse(raw);const out=await call("/public/v1/app-auth/refresh",{refresh_token:saved.refresh_token,app_instance_id:id});await keepSession(out)}}catch{await removeStored(sessionKey)}finally{setBooting(false)}})()},[]);
  useEffect(()=>{if(!session||!instanceId)return;const timer=setTimeout(()=>{void(async()=>{try{const out=await call("/public/v1/app-auth/refresh",{refresh_token:session.refresh_token,app_instance_id:instanceId});await keepSession(out)}catch{await logout()}})()},Math.max(30000,(session.expires_in-60)*1000));return()=>clearTimeout(timer)},[session?.refresh_token,instanceId]);
  const roles=useMemo(()=>session?.tenant.roles||[],[session]);
  return {configured:true,ready:!busy&&!booting,accessToken:session?.access_token||"",roles,error,email,setEmail,code,setCode,temporary,tenants,currentTenant:session?.tenant,requestCode,verifyCode,selectTenant,switchTenant,logout};
}
