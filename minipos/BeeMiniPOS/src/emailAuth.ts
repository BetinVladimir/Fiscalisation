import AsyncStorage from "@react-native-async-storage/async-storage";
import { useEffect, useState } from "react";

export type Language = "bg" | "ru" | "en";
export type AuthStage = "loading" | "email" | "code" | "onboarding" | "authenticated";
type Tokens = { access_token: string; refresh_token: string; expires_in: number };

export function useEmailAuth(apiBase: string) {
  const [stage,setStage]=useState<AuthStage>("loading"), [accessToken,setAccessToken]=useState(""), [email,setEmail]=useState(""), [onboardingToken,setOnboardingToken]=useState(""), [error,setError]=useState(""), [busy,setBusy]=useState(false), [language,setLanguageState]=useState<Language>("bg");
  useEffect(()=>{void Promise.all([AsyncStorage.getItem("minipos-access-token"),AsyncStorage.getItem("minipos-language")]).then(([token,lang])=>{if(lang) setLanguageState(lang as Language);if(token){setAccessToken(token);setStage("authenticated");}else setStage("email");});},[]);
  async function request(path:string, body:unknown){const response=await fetch(apiBase+path,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});const text=await response.text();if(!response.ok)throw new Error(text||`HTTP ${response.status}`);return text?JSON.parse(text):{};}
  async function execute(action:()=>Promise<void>){setBusy(true);setError("");try{await action();}catch(e){setError(e instanceof Error?e.message:String(e));}finally{setBusy(false);}}
  async function save(tokens:Tokens){await AsyncStorage.multiSet([["minipos-access-token",tokens.access_token],["minipos-refresh-token",tokens.refresh_token]]);setAccessToken(tokens.access_token);setStage("authenticated");}
  useEffect(()=>{if(!accessToken)return;const timer=setInterval(()=>{void AsyncStorage.getItem("minipos-refresh-token").then(token=>token?request("/auth/refresh",{refresh_token:token}):Promise.reject()).then(save).catch(()=>void 0);},12*60*1000);return()=>clearInterval(timer);},[accessToken]);
  return {stage,accessToken,email,error,busy,language,setEmail,setLanguage:async(value:Language)=>{setLanguageState(value);await AsyncStorage.setItem("minipos-language",value);},requestCode:()=>execute(async()=>{await request("/auth/request-code",{email,language});setStage("code");}),verifyCode:(code:string)=>execute(async()=>{const result=await request("/auth/verify-code",{email,code});if(result.onboarding_required){setOnboardingToken(result.onboarding_token);setStage("onboarding");}else await save(result);}),onboard:(input:{company_name:string;address:string;tax_identifier:string;full_name:string})=>execute(async()=>save(await request("/auth/onboarding",{onboarding_token:onboardingToken,...input}))),logout:async()=>{await AsyncStorage.multiRemove(["minipos-access-token","minipos-refresh-token"]);setAccessToken("");setStage("email");}};
}
