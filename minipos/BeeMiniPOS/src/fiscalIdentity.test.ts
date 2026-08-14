import assert from "node:assert/strict";
import test from "node:test";
import {
  isFiscalResourceId,
  requireFiscalResourceId,
} from "./fiscalIdentity.ts";

test("accepts canonical UUIDs", () =>
  assert.equal(
    isFiscalResourceId("00000000-0000-4000-8000-000000000001"),
    true,
  ));
test("rejects aliases and blanks", () => {
  assert.equal(isFiscalResourceId("FD000001"), false);
  assert.equal(isFiscalResourceId(" "), false);
});
test("fails closed before a public Fiscal API call", () =>
  assert.throws(
    () => requireFiscalResourceId("register-1", "Фискалната каса"),
    /UUID/,
  ));
