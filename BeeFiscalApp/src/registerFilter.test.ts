import assert from "node:assert/strict";
import test from "node:test";
import {
  devicesBoundToRegister,
  isRegisterId,
  registerCollectionPath,
  registerResourcePath,
} from "./registerFilter.ts";

test("register filter is omitted when no register is configured", () => {
  assert.equal(registerCollectionPath("/operations", ""), "/operations");
});

test('device context uses only the fiscal and payment device IDs bound to a register', () => {
  const devices = [{ id: 'fiscal' }, { id: 'payment' }, { id: 'another-register' }];
  assert.deepEqual(devicesBoundToRegister(devices, { fiscal_device_id: 'fiscal', payment_terminal_id: 'payment' }), devices.slice(0, 2));
  assert.deepEqual(devicesBoundToRegister(devices, { fiscal_device_id: null, payment_terminal_id: null }), []);
  assert.deepEqual(devicesBoundToRegister(devices, undefined), []);
});

test("register filter accepts only an OpenAPI UUID", () => {
  const id = "00000000-0000-4000-8000-000000000001";
  assert.equal(isRegisterId(id), true);
  assert.equal(
    registerCollectionPath("/reports", id),
    `/reports?register_id=${id}`,
  );
  assert.throws(
    () => registerCollectionPath("/reports", "FD000001"),
    /INVALID_REGISTER_ID/,
  );
  assert.equal(
    registerResourcePath(id, "/reports"),
    `/registers/${id}/reports`,
  );
  assert.throws(
    () => registerResourcePath("register-1", "/bindings"),
    /INVALID_REGISTER_ID/,
  );
});
