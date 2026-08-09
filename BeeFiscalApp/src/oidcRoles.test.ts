import assert from "node:assert/strict";
import test from "node:test";
import { accessTokenRoles } from "./oidcRoles.ts";

function token(claims: unknown): string {
  const payload = Buffer.from(JSON.stringify(claims)).toString("base64url");
  return `header.${payload}.signature`;
}

test("role discovery normalizes only recognized access-token roles", () => {
  assert.deepEqual(
    accessTokenRoles(token({ roles: ["admin", "AUDITOR", "admin", "ROOT", 7] })),
    ["ADMIN", "AUDITOR"],
  );
});

test("opaque, malformed and missing role claims fail closed", () => {
  assert.deepEqual(accessTokenRoles("opaque"), []);
  assert.deepEqual(accessTokenRoles("a.not-json.c"), []);
  assert.deepEqual(accessTokenRoles(token({ scope: "fiscal.base" })), []);
  assert.deepEqual(accessTokenRoles(token({ roles: "ADMIN" })), []);
});
