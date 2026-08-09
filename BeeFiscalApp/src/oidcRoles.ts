const KNOWN_ROLES = new Set(["CASHIER", "SUPERVISOR", "ADMIN", "AUDITOR", "SERVICE"]);

export function accessTokenRoles(accessToken: string): string[] {
  const encoded = accessToken.split(".")[1];
  if (!encoded) return [];
  try {
    const normalized = encoded
      .replace(/-/g, "+")
      .replace(/_/g, "/")
      .padEnd(Math.ceil(encoded.length / 4) * 4, "=");
    const claims = JSON.parse(globalThis.atob(normalized)) as { roles?: unknown };
    if (!Array.isArray(claims.roles)) return [];
    return [
      ...new Set(
        claims.roles
          .filter((role): role is string => typeof role === "string")
          .map((role) => role.toUpperCase())
          .filter((role) => KNOWN_ROLES.has(role)),
      ),
    ];
  } catch {
    // UI privilege discovery is advisory. An opaque/malformed token receives
    // no privileged controls; the API remains the authorization authority.
    return [];
  }
}
