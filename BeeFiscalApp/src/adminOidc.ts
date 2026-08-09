import { useEffect, useMemo, useState } from "react";
import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { nextOidcTokenAction } from "./oidcTokenLifetime";

WebBrowser.maybeCompleteAuthSession();
const OIDC_SCOPES = ["openid", "profile", "offline_access", "fiscal.base"];
const KNOWN_ROLES = new Set(["CASHIER", "SUPERVISOR", "ADMIN", "AUDITOR", "SERVICE"]);

export function accessTokenRoles(accessToken: string): string[] {
  const encoded = accessToken.split(".")[1];
  if (!encoded) return [];
  try {
    const normalized = encoded.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(encoded.length / 4) * 4, "=");
    const claims = JSON.parse(globalThis.atob(normalized)) as { roles?: unknown };
    if (!Array.isArray(claims.roles)) return [];
    return [...new Set(claims.roles.filter((role): role is string => typeof role === "string").map((role) => role.toUpperCase()).filter((role) => KNOWN_ROLES.has(role)))];
  } catch {
    return [];
  }
}

export function useAdminOidc() {
  const issuer = (process.env.EXPO_PUBLIC_FISCAL_OIDC_ISSUER || "").replace(/\/$/, "");
  const clientId = process.env.EXPO_PUBLIC_FISCAL_OIDC_CLIENT_ID || "";
  const configured = issuer.startsWith("https://") && clientId.length > 0;
  const discovery = AuthSession.useAutoDiscovery(configured ? issuer : "");
  const redirectUri = useMemo(
    () => AuthSession.makeRedirectUri({ scheme: "beefiscalapp", path: "oauth/callback" }),
    [],
  );
  const [request, response, promptAsync] = AuthSession.useAuthRequest(
    {
      clientId,
      redirectUri,
      responseType: AuthSession.ResponseType.Code,
      scopes: OIDC_SCOPES,
      usePKCE: true,
    },
    configured ? discovery : null,
  );
  const [tokenResponse, setTokenResponse] = useState<AuthSession.TokenResponse | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!response) return;
    if (response.type !== "success") {
      if (response.type !== "dismiss" && response.type !== "cancel") {
        setError(`OIDC_${response.type.toUpperCase()}`);
      }
      return;
    }
    if (!request?.codeVerifier || !discovery) {
      setError("OIDC_PKCE_STATE_MISSING");
      return;
    }
    void AuthSession.exchangeCodeAsync(
      { clientId, code: response.params.code, redirectUri, extraParams: { code_verifier: request.codeVerifier } },
      discovery,
    )
      .then((token) => {
        if (!token.accessToken) throw new Error("OIDC_ACCESS_TOKEN_MISSING");
        setTokenResponse(token);
        setError("");
      })
      .catch((reason) => setError(reason instanceof Error ? reason.message : "OIDC_TOKEN_EXCHANGE_FAILED"));
  }, [response, request, discovery, clientId, redirectUri]);

  useEffect(() => {
    if (!tokenResponse) return;
    const next = nextOidcTokenAction(tokenResponse);
    if (!next) return;
    const timer = setTimeout(() => {
      if (next.action === "EXPIRE" || !tokenResponse.refreshToken || !discovery) {
        setTokenResponse(null);
        setError("OIDC_SESSION_EXPIRED");
        return;
      }
      void AuthSession.refreshAsync(
        { clientId, refreshToken: tokenResponse.refreshToken, scopes: OIDC_SCOPES },
        discovery,
      )
        .then((token) => {
          if (!token.accessToken) throw new Error("OIDC_ACCESS_TOKEN_MISSING");
          if (!token.refreshToken) token.refreshToken = tokenResponse.refreshToken;
          setTokenResponse(token);
          setError("");
        })
        .catch((reason) => {
          setTokenResponse(null);
          setError(reason instanceof Error ? reason.message : "OIDC_TOKEN_REFRESH_FAILED");
        });
    }, next.delayMs);
    return () => clearTimeout(timer);
  }, [tokenResponse, discovery, clientId]);

  return {
    configured,
    ready: configured && !!request && !!discovery,
    accessToken: tokenResponse?.accessToken || "",
    roles: accessTokenRoles(tokenResponse?.accessToken || ""),
    error,
    login: async () => {
      if (!configured || !request || !discovery) {
        setError("OIDC_NOT_CONFIGURED");
        return;
      }
      setError("");
      await promptAsync();
    },
    logout: () => {
      setTokenResponse(null);
      setError("");
    },
  };
}
