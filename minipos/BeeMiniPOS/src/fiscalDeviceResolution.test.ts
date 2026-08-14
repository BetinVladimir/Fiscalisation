import assert from "node:assert/strict";
import test from "node:test";
import { resolveFiscalDeviceId } from "./fiscalDeviceResolution.ts";

const registerId = "00000000-0000-4000-8000-000000000001";
const deviceId = "00000000-0000-4000-8000-000000000002";

test("resolves the active Fiscal device from the public register resource", () => {
  assert.equal(
    resolveFiscalDeviceId(
      { id: registerId, status: "ACTIVE", fiscal_device_id: deviceId },
      registerId,
    ),
    deviceId,
  );
});

test("fails closed for mismatched, inactive, unbound or malformed resources", () => {
  assert.throws(
    () =>
      resolveFiscalDeviceId(
        { id: deviceId, status: "ACTIVE", fiscal_device_id: deviceId },
        registerId,
      ),
    /съвпада/,
  );
  assert.throws(
    () =>
      resolveFiscalDeviceId(
        { id: registerId, status: "BLOCKED", fiscal_device_id: deviceId },
        registerId,
      ),
    /активна/,
  );
  assert.throws(
    () =>
      resolveFiscalDeviceId(
        { id: registerId, status: "ACTIVE", fiscal_device_id: null },
        registerId,
      ),
    /няма активно/,
  );
  assert.throws(
    () =>
      resolveFiscalDeviceId(
        { id: registerId, status: "ACTIVE", fiscal_device_id: "FD1" },
        registerId,
      ),
    /UUID/,
  );
});

test("permits an explicit DEV fallback only after the public register is active", () => {
  assert.equal(
    resolveFiscalDeviceId(
      { id: registerId, status: "ACTIVE", fiscal_device_id: null },
      registerId,
      deviceId,
    ),
    deviceId,
  );
});
