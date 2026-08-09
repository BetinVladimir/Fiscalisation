import assert from "node:assert/strict";
import test from "node:test";
import { isRegisterId, registerCollectionPath, registerResourcePath } from "./registerFilter.ts";

test("register filter is omitted when no register is configured", () => {
  assert.equal(registerCollectionPath("/operations", ""), "/operations");
});

test("register filter accepts only an OpenAPI UUID", () => {
  const id = "00000000-0000-4000-8000-000000000001";
  assert.equal(isRegisterId(id), true);
  assert.equal(registerCollectionPath("/reports", id), `/reports?register_id=${id}`);
  assert.throws(() => registerCollectionPath("/reports", "FD000001"), /INVALID_REGISTER_ID/);
  assert.equal(registerResourcePath(id, "/reports"), `/registers/${id}/reports`);
  assert.throws(() => registerResourcePath("register-1", "/bindings"), /INVALID_REGISTER_ID/);
});
