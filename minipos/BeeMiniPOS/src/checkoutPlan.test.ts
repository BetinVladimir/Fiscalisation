import test from "node:test";
import assert from "node:assert/strict";
import { newCheckoutPlan, bleFinalizeIntent } from "./checkoutPlan.ts";
test("REST and BLE share one durable operation receipt and payment plan", () => {
  let n = 0;
  const uuid = () => `00000000-0000-4000-8000-${String(++n).padStart(12, "0")}`,
    payments = [{ payment_id: "p", type: "CARD" }];
  const p = newCheckoutPlan(uuid, payments, 2.5),
    ble = bleFinalizeIntent(
      p,
      {
        tenant_id: "t",
        location_id: "l",
        register_id: "r",
        edge_id: "e",
        binding_version: 3,
      },
      { sale_id: "s", external_id: "x", unp: "U", lines: [1] },
    );
  assert.equal(ble.intent_id, p.client_operation_id);
  assert.equal(ble.receipt_session_id, p.receipt_session_id);
  assert.equal(ble.payments, payments);
  assert.equal(ble.location_id, "l");
  assert.equal(ble.command, "SALE_FINALIZE");
  assert.equal("unp" in ble, false);
  assert.deepEqual({ ...p }, p);
});
