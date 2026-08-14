import test from "node:test";
import assert from "node:assert/strict";
import {
  MVP1_DEVICE_PROFILES,
  profileById,
  validateCompositeProfile,
} from "./deviceProfiles.ts";

test("MVP1 compatibility matrix exposes the three exact physical profiles", () => {
  assert.deepEqual(
    MVP1_DEVICE_PROFILES.map((x) => x.id),
    [
      "DATECS_BLUECASH50_EMBEDDED",
      "DATECS_DP150_BLUEPAD50",
      "DAISY_COMPACT_S01",
    ],
  );
  const composite = profileById("DATECS_DP150_BLUEPAD50")!;
  assert.equal(composite.fiscal.transport, "RS232");
  assert.equal(composite.payment?.transport, "BLE_GATT");
  assert.equal(validateCompositeProfile(composite), true);
});

test("unsupported and mutated hardware combinations fail closed", () => {
  assert.equal(profileById("DATECS_DP150_DAISY"), undefined);
  const profile = structuredClone(profileById("DATECS_DP150_BLUEPAD50")!);
  profile.payment!.transport = "USB_SERIAL";
  assert.equal(validateCompositeProfile(profile), false);
});
