import {useEffect, useMemo, useState} from "react";
import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import {nextOidcTokenAction} from "./oidcTokenLifetime";

WebBrowser.maybeCompleteAuthSession();
const OIDC_SCOPES=["openid","profile","offline_access","fiscal.base"];

export type OperatorOidcState={
  configured:boolean;
  ready:boolean;
  accessToken:string;
  error:string;
  login:()=>Promise<void>;
  logout:()=>void;
};

export function useOperatorOidc():OperatorOidcState{
  const issuer=(process.env.EXPO_PUBLIC_MINIPOS_OIDC_ISSUER||"").replace(/\/$/,"");
  const clientId=process.env.EXPO_PUBLIC_MINIPOS_OIDC_CLIENT_ID||"";
  const configured=issuer.startsWith("https://")&&clientId.length>0;
  const discovery=AuthSession.useAutoDiscovery(configured?issuer:"");
  const redirectUri=useMemo(()=>AuthSession.makeRedirectUri({scheme:"beeminipos",path:"oauth/callback"}),[]);
  const [request,response,promptAsync]=AuthSession.useAuthRequest({clientId,redirectUri,responseType:AuthSession.ResponseType.Code,scopes:OIDC_SCOPES,usePKCE:true},configured?discovery:null);
  const [tokenResponse,setTokenResponse]=useState<AuthSession.TokenResponse|null>(null);
  const [error,setError]=useState("");

  useEffect(()=>{
    if(!response)return;
    if(response.type!=="success"){
      if(response.type!=="dismiss"&&response.type!=="cancel")setError(`OIDC_${response.type.toUpperCase()}`);
      return;
    }
    if(!request?.codeVerifier||!discovery){setError("OIDC_PKCE_STATE_MISSING");return}
    void AuthSession.exchangeCodeAsync({clientId,code:response.params.code,redirectUri,extraParams:{code_verifier:request.codeVerifier}},discovery)
      .then(token=>{if(!token.accessToken)throw new Error("OIDC_ACCESS_TOKEN_MISSING");setTokenResponse(token);setError("")})
      .catch(reason=>setError(reason instanceof Error?reason.message:"OIDC_TOKEN_EXCHANGE_FAILED"));
  },[response,request,discovery,clientId,redirectUri]);

  useEffect(()=>{
    if(!tokenResponse)return;
    const next=nextOidcTokenAction(tokenResponse);
    if(!next)return;
    const timer=setTimeout(()=>{
      if(next.action==="EXPIRE"||!tokenResponse.refreshToken||!discovery){
        setTokenResponse(null);
        setError("OIDC_SESSION_EXPIRED");
        return;
      }
      void AuthSession.refreshAsync({clientId,refreshToken:tokenResponse.refreshToken,scopes:OIDC_SCOPES},discovery)
        .then(token=>{if(!token.accessToken)throw new Error("OIDC_ACCESS_TOKEN_MISSING");if(!token.refreshToken)token.refreshToken=tokenResponse.refreshToken;setTokenResponse(token);setError("")})
        .catch(reason=>{setTokenResponse(null);setError(reason instanceof Error?reason.message:"OIDC_TOKEN_REFRESH_FAILED")});
    },next.delayMs);
    return()=>clearTimeout(timer);
  },[tokenResponse,discovery,clientId]);

  return {
    configured,
    ready:configured&&!!request&&!!discovery,
    accessToken:tokenResponse?.accessToken||"",
    error,
    login:async()=>{if(!configured||!request||!discovery){setError("OIDC_NOT_CONFIGURED");return}setError("");await promptAsync()},
    logout:()=>{setTokenResponse(null);setError("")}
  };
}
