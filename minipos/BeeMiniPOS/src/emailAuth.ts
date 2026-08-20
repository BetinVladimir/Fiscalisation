import AsyncStorage from "@react-native-async-storage/async-storage";
import { useEffect, useState } from "react";
import { AppState } from "react-native";
import { useLanguageStore } from "./languageStore";
import { nextOidcTokenAction, type OidcTokenLifetime } from "./oidcTokenLifetime";

export type { Language } from "./languageStore";
export type AuthStage = "loading" | "email" | "code" | "onboarding" | "authenticated";
type Tokens = { access_token: string; refresh_token: string; expires_in: number };

export function useEmailAuth(apiBase: string) {
  const [stage,setStage]=useState<AuthStage>("loading"), [accessToken,setAccessToken]=useState(""), [tokenLifetime,setTokenLifetime]=useState<OidcTokenLifetime|null>(null), [email,setEmail]=useState(""), [onboardingToken,setOnboardingToken]=useState(""), [error,setError]=useState(""), [busy,setBusy]=useState(false);
  const language=useLanguageStore(state=>state.language), setLanguage=useLanguageStore(state=>state.setLanguage);
  async function request(path:string, body:unknown){const response=await fetch(apiBase+path,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});const text=await response.text();if(!response.ok)throw new Error(text||`HTTP ${response.status}`);return text?JSON.parse(text):{};}
  async function execute(action:()=>Promise<void>){setBusy(true);setError("");try{await action();}catch(e){setError(e instanceof Error?e.message:String(e));}finally{setBusy(false);}}
  async function clearAuthority(){await AsyncStorage.multiRemove(["minipos-access-token","minipos-refresh-token"]);setAccessToken("");setTokenLifetime(null);setStage("email");}
  async function save(tokens:Tokens){await AsyncStorage.multiSet([["minipos-access-token",tokens.access_token],["minipos-refresh-token",tokens.refresh_token]]);setTokenLifetime({issuedAt:Date.now()/1000,expiresIn:tokens.expires_in,refreshToken:tokens.refresh_token});setAccessToken(tokens.access_token);setStage("authenticated");}
  async function refresh(){const token=await AsyncStorage.getItem("minipos-refresh-token");if(!token)throw new Error("refresh token is missing");const tokens=await request("/auth/refresh",{refresh_token:token}) as Tokens;await save(tokens);return tokens.access_token;}
  useEffect(()=>{void Promise.all([AsyncStorage.getItem("minipos-access-token"),AsyncStorage.getItem("minipos-refresh-token")]).then(async([token,refreshToken])=>{if(refreshToken){try{await refresh();return;}catch{await clearAuthority();return;}}if(token){await clearAuthority();return;}setStage("email");});},[]);
  useEffect(()=>{if(!accessToken||!tokenLifetime)return;const renew=async()=>{try{await refresh();}catch{await clearAuthority();}};const next=nextOidcTokenAction(tokenLifetime);const timer=setTimeout(()=>{if(next.action==="REFRESH")void renew();else void clearAuthority();},next.delayMs);const subscription=AppState.addEventListener("change",state=>{if(state==="active")void renew();});return()=>{clearTimeout(timer);subscription.remove();};},[accessToken,tokenLifetime]);
  return {stage,accessToken,email,error,busy,language,setEmail,setLanguage,requestCode:()=>execute(async()=>{await request("/auth/request-code",{email,language});setStage("code");}),verifyCode:(code:string)=>execute(async()=>{const result=await request("/auth/verify-code",{email,code});if(result.onboarding_required){setOnboardingToken(result.onboarding_token);setStage("onboarding");}else await save(result);}),onboard:(input:{company_name:string;address:string;tax_identifier:string;full_name:string})=>execute(async()=>save(await request("/auth/onboarding",{onboarding_token:onboardingToken,...input}))),logout:clearAuthority};
}
