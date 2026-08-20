import assert from "node:assert/strict";
import test from "node:test";
import { nextOidcTokenAction } from "./oidcTokenLifetime.ts";

test("OIDC refresh is scheduled before access-token expiry", () => {
  assert.deepEqual(
    nextOidcTokenAction(
      { issuedAt: 1_000, expiresIn: 300, refreshToken: "rotating" },
      1_100_000,
    ),
    { action: "REFRESH", delayMs: 140_000 },
  );
});

test("OIDC token without refresh authority expires exactly at its deadline", () => {
  assert.deepEqual(
    nextOidcTokenAction({ issuedAt: 1_000, expiresIn: 300 }, 1_100_000),
    { action: "EXPIRE", delayMs: 200_000 },
  );
});

test("already due and opaque or malformed tokens expire or refresh immediately", () => {
  assert.deepEqual(
    nextOidcTokenAction(
      { issuedAt: 1_000, expiresIn: 60, refreshToken: "r" },
      2_000_000,
    ),
    { action: "REFRESH", delayMs: 0 },
  );
  assert.deepEqual(nextOidcTokenAction({}), { action: "EXPIRE", delayMs: 0 });
  assert.deepEqual(
    nextOidcTokenAction({ issuedAt: 1_000, expiresIn: 0, refreshToken: "r" }),
    { action: "EXPIRE", delayMs: 0 },
  );
});
