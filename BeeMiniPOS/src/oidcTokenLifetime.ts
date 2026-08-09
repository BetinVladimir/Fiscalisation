export type OidcTokenLifetime = {
  issuedAt?: number;
  expiresIn?: number;
  refreshToken?: string;
};

export type OidcTokenAction = {
  action: "REFRESH" | "EXPIRE";
  delayMs: number;
};

const REFRESH_HEADROOM_SECONDS = 60;
const MAX_TIMER_MS = 2_147_000_000;

export function nextOidcTokenAction(token: OidcTokenLifetime, nowMs = Date.now()): OidcTokenAction | null {
  if (!Number.isFinite(token.issuedAt) || !Number.isFinite(token.expiresIn) || token.expiresIn! <= 0) {
    return {action: "EXPIRE", delayMs: 0};
  }
  const expiresAtMs = (token.issuedAt! + token.expiresIn!) * 1000;
  const targetMs = token.refreshToken
    ? expiresAtMs - Math.min(REFRESH_HEADROOM_SECONDS, Math.max(1, token.expiresIn! / 5)) * 1000
    : expiresAtMs;
  return {
    action: token.refreshToken ? "REFRESH" : "EXPIRE",
    delayMs: Math.min(MAX_TIMER_MS, Math.max(0, Math.floor(targetMs - nowMs))),
  };
}
