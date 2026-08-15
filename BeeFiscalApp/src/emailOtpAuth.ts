import { useMemo, useState } from "react";
import * as Crypto from "expo-crypto";

type Tenant = { tenant_id: string; display_name: string; roles: string[] };
type Session = { access_token: string; refresh_token: string; expires_in: number; tenant: Tenant };
const apiRoot = (process.env.EXPO_PUBLIC_FISCAL_API_URL || "http://localhost:8080/public/v1").replace(/\/public\/v1\/?$/, "");
const instanceId = Crypto.randomUUID();

export function useEmailOtpAuth() {
  const [email,setEmail]=useState(""); const [code,setCode]=useState("");
  const [temporary,setTemporary]=useState(""); const [selection,setSelection]=useState("");
  const [tenants,setTenants]=useState<Tenant[]>([]); const [session,setSession]=useState<Session|null>(null);
  const [error,setError]=useState(""); const [busy,setBusy]=useState(false);
  const call=async(path:string,body:unknown,token="")=>{const r=await fetch(apiRoot+path,{method:"POST",headers:{"Content-Type":"application/json",...(token?{Authorization:`Bearer ${token}`}:{})},body:JSON.stringify(body)});const text=await r.text();if(!r.ok)throw new Error(text||`HTTP ${r.status}`);return text?JSON.parse(text):{}};
  const requestCode=async()=>{setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/challenges",{email,language:"bg",app_instance_id:instanceId});setTemporary(out.temporary_token);setCode("");}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const verifyCode=async()=>{setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/challenges:verify",{code,app_instance_id:instanceId},temporary);if(out.session){setSession(out.session);await loadTenants(out.session.access_token)}else{setSelection(out.tenant_selection_token);setTenants(out.tenants||[])}}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const loadTenants=async(accessToken:string)=>{const r=await fetch(apiRoot+"/public/v1/app-auth/tenants",{headers:{Authorization:`Bearer ${accessToken}`}});if(r.ok){const out=await r.json();setTenants(out.items||[])}};
  const selectTenant=async(tenantId:string)=>{setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/tenant-session",{tenant_id:tenantId,app_instance_id:instanceId},selection);setSession(out);await loadTenants(out.access_token)}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const switchTenant=async(tenantId:string)=>{if(!session)return;setBusy(true);setError("");try{const out=await call("/public/v1/app-auth/sessions:switch-tenant",{refresh_token:session.refresh_token,tenant_id:tenantId,app_instance_id:instanceId});setSession(out);await loadTenants(out.access_token)}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
  const logout=async()=>{const current=session;setSession(null);setTemporary("");setSelection("");setCode("");setError("");if(current)try{await call("/public/v1/app-auth/logout",{refresh_token:current.refresh_token,app_instance_id:instanceId})}catch{/* local authority is already cleared */}};
  const roles=useMemo(()=>session?.tenant.roles||[],[session]);
  return {configured:true,ready:!busy,accessToken:session?.access_token||"",roles,error,email,setEmail,code,setCode,temporary,tenants,currentTenant:session?.tenant,requestCode,verifyCode,selectTenant,switchTenant,logout};
}
