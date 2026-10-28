import test from "node:test";
import assert from "node:assert/strict";
import { newCheckoutPlan, bleFinalizeIntent } from "./checkoutPlan.ts";
test("checkout plan remains stable and legacy unsigned BLE builder fails before I/O", () => {
  let n=0;const p=newCheckoutPlan(()=>`00000000-0000-4000-8000-${String(++n).padStart(12,'0')}`,[{payment_id:'p',type:'CARD'}],2.5);
  assert.equal(p.expected_total.amount,'2.50');assert.notEqual(p.client_operation_id,p.receipt_session_id);
  assert.throws(()=>bleFinalizeIntent(p,{},{}),/EXECUTION_AUTHORIZATION_REQUIRED/);
});
